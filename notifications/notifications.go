////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// This file contains the main logic for notifications, including the main implementation and core loop

package notifications

import (
	"encoding/base64"
	"gitlab.com/elixxir/notifications-bot/notifications/apns"
	"sync"

	"github.com/pkg/errors"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	apnstoken "github.com/sideshow/apns2/token"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/notificationBot"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/crypto/registration"
	"gitlab.com/elixxir/notifications-bot/notifications/firebase"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/elixxir/primitives/notifications"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gitlab.com/xx_network/primitives/ndf"
	"gitlab.com/xx_network/primitives/netTime"
	"gitlab.com/xx_network/primitives/utils"
	"gorm.io/gorm"
	"strings"
	"time"
)

const notificationsTag = "notificationData"

// Function type definitions for the main operations (poll and notify)
type NotifyFunc func(int64, []*notifications.Data, *apns.ApnsComm,
	*firebase.FirebaseComm, *storage.Storage, int, int) ([]*notifications.Data, error)

// Params struct holds info passed in for configuration
type Params struct {
	Address                string
	CertPath               string
	KeyPath                string
	FBCreds                string
	NotificationsPerBatch  int
	MaxNotificationPayload int
	NotificationRate       int
	APNS                   APNSParams
}
type APNSParams struct {
	KeyPath  string
	KeyID    string
	Issuer   string
	BundleID string
	Dev      bool
}

// Local impl for notifications; holds comms, storage object, creds and main functions
type Impl struct {
	Comms            *notificationBot.Comms
	Storage          *storage.Storage
	inst             *network.Instance
	notifyFunc       NotifyFunc
	fcm              *firebase.FirebaseComm
	apnsClient       *apns.ApnsComm
	receivedNdf      *uint32
	roundStore       sync.Map
	maxNotifications int
	maxPayloadBytes  int

	ndfStopper Stopper
}

// StartNotifications creates an Impl from the information passed in
func StartNotifications(params Params, noTLS, noFirebase bool) (*Impl, error) {
	var cert, key []byte
	var err error

	// Read in private key
	if params.KeyPath != "" {
		key, err = utils.ReadFile(params.KeyPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read key at %+v", params.KeyPath)
		}
	} else {
		jww.WARN.Println("Running without key...")
	}

	if !noTLS {
		// Read in TLS keys from files
		cert, err = utils.ReadFile(params.CertPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read certificate at %+v", params.CertPath)
		}
	}

	// Set up firebase messaging client
	var fbComm *firebase.FirebaseComm
	if !noFirebase {
		app, err := firebase.SetupMessagingApp(params.FBCreds)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to setup firebase messaging app")
		}
		fbComm = firebase.NewFirebaseComm(app)
	}
	receivedNdf := uint32(0)

	impl := &Impl{
		notifyFunc:       notifyUser,
		fcm:              fbComm,
		receivedNdf:      &receivedNdf,
		maxNotifications: params.NotificationsPerBatch,
		maxPayloadBytes:  params.MaxNotificationPayload,
	}

	if params.APNS.KeyPath == "" {
		jww.WARN.Println("WARNING: RUNNING WITHOUT APNS")
	} else {
		if params.APNS.KeyID == "" || params.APNS.Issuer == "" || params.APNS.BundleID == "" {
			return nil, errors.WithMessagef(err, "APNS not properly configured: %+v", params.APNS)
		}

		authKey, err := apnstoken.AuthKeyFromFile(params.APNS.KeyPath)
		if err != nil {
			return nil, errors.WithMessage(err, "Failed to load auth key from file")
		}
		token := &apnstoken.Token{
			AuthKey: authKey,
			// KeyID from developer account (Certificates, Identifiers & Profiles -> Keys)
			KeyID: params.APNS.KeyID,
			// TeamID from developer account (View Account -> Membership)
			TeamID: params.APNS.Issuer,
		}
		apnsClient := apns2.NewTokenClient(token)
		if params.APNS.Dev {
			jww.INFO.Printf("Running with dev apns gateway")
			apnsClient.Development()
		} else {
			apnsClient.Production()
		}

		impl.apnsClient = apns.NewApnsComm(apnsClient, params.APNS.BundleID)
	}

	// Start notification comms server
	handler := NewImplementation(impl)
	comms := notificationBot.StartNotificationBot(&id.NotificationBot, params.Address, handler, cert, key)
	impl.Comms = comms
	i, err := network.NewInstance(impl.Comms.ProtoComms, &ndf.NetworkDefinition{AddressSpace: []ndf.AddressSpace{{Size: 16, Timestamp: netTime.Now()}}}, nil, nil, network.None, false)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to start instance")
	}
	i.SetGatewayAuthentication()
	impl.inst = i

	go impl.Cleaner()
	go impl.Sender(params.NotificationRate)

	return impl, nil
}

// NewImplementation initializes impl object
func NewImplementation(instance *Impl) *notificationBot.Implementation {
	impl := notificationBot.NewImplementation()

	impl.Functions.RegisterForNotifications = func(request *pb.NotificationRegisterRequest) error {
		return instance.RegisterForNotifications(request)
	}

	impl.Functions.UnregisterForNotifications = func(request *pb.NotificationUnregisterRequest) error {
		return instance.UnregisterForNotifications(request)
	}

	impl.Functions.ReceiveNotificationBatch = func(data *pb.NotificationBatch, auth *connect.Auth) error {
		return instance.ReceiveNotificationBatch(data, auth)
	}

	return impl
}

func (nb *Impl) SendBatch(data map[int64][]*notifications.Data, maxBatchSize, maxBytes int) error {
	csvs := map[int64]string{}
	var ephemerals []int64
	for i, ilist := range data {
		var overflow, trimmed []*notifications.Data
		if len(data) > maxBatchSize {
			overflow = ilist[maxBatchSize:]
			trimmed = ilist[:maxBatchSize]
		}

		notifs, rest := notifications.BuildNotificationCSV(trimmed, maxBytes-len([]byte(notificationsTag)))
		for _, nd := range rest {
			nb.Storage.GetNotificationBuffer().Add(id.Round(nd.RoundID), []*notifications.Data{nd}) // TODO build lists by rid for more efficient re-insertion?  Accumulator would let us do this with the other size check in swap
		}
		overflow = append(overflow, rest...)
		csvs[i] = string(notifs)
		ephemerals = append(ephemerals, i)
	}
	toNotify, err := nb.Storage.GetToNotify(ephemerals)
	if err != nil {
		return errors.WithMessage(err, "Failed to get list of tokens to notify")
	}
	for _, n := range toNotify {
		nb.notify(n.Token, csvs[n.EphemeralId], n.EphemeralId, n.TransmissionRSAHash)
	}
	return nil
}

func (nb *Impl) notify(token, csv string, ephID int64, transmissionRSAHash []byte) {
	isAPNS := !strings.Contains(token, ":")
	// mutableContent := 1
	if isAPNS {
		jww.INFO.Printf("Notifying ephemeral ID %+v via APNS to token %+v", ephID, token)
		notifPayload := payload.NewPayload().AlertTitle("Privacy: protected!").AlertBody(
			"Some notifications are not for you to ensure privacy; we hope to remove this notification soon").MutableContent().Custom(
			notificationsTag, csv)
		notif := &apns2.Notification{
			CollapseID:  base64.StdEncoding.EncodeToString(transmissionRSAHash),
			DeviceToken: token,
			Expiration:  time.Now().Add(time.Hour * 24 * 7),
			Priority:    apns2.PriorityHigh,
			Payload:     notifPayload,
			PushType:    apns2.PushTypeAlert,
			Topic:       nb.apnsClient.GetTopic(),
		}
		resp, err := nb.apnsClient.Push(notif)
		if err != nil {
			jww.ERROR.Printf("Failed to send notification via APNS: %+v: %+v", resp, err)
			// TODO : Should be re-enabled for specific error cases? deep dive on apns docs may be helpful
			//err := db.DeleteUserByHash(u.TransmissionRSAHash)
			//if err != nil {
			//	return errors.WithMessagef(err, "Failed to remove user registration tRSA hash: %+v", u.TransmissionRSAHash)
			//}
		} else {
			jww.INFO.Printf("Notified ephemeral ID %+v [%+v] and received response %+v", ephID, token, resp)
		}
	} else {
		resp, err := nb.fcm.SendNotification(nb.fcm.Client, token, csv)
		if err != nil {
			// Catch firebase errors that we don't want to crash on
			// 404 indicate that the token stored is incorrect
			// this means rather than crashing we should log and unregister the user
			// 400 can also indicate incorrect token, do extra checking on this (12/27/2021)
			// Error documentation: https://firebase.google.com/docs/reference/fcm/rest/v1/ErrorCode
			// Stale token documentation: https://firebase.google.com/docs/cloud-messaging/manage-tokens
			jww.ERROR.Printf("Error sending notification: %+v", err)
			invalidToken := strings.Contains(err.Error(), "400") &&
				strings.Contains(err.Error(), "Invalid registration")
			if strings.Contains(err.Error(), "404") || invalidToken {
				jww.ERROR.Printf("User with Transmission RSA hash %+v has invalid token, unregistering...", transmissionRSAHash)
				err := nb.Storage.DeleteUserByHash(transmissionRSAHash)
				if err != nil {
					jww.ERROR.Printf("Failed to remove user registration tRSA hash %+v: %+v", transmissionRSAHash, err)
				}
			} else {
				jww.ERROR.Printf("Failed to send notification to user with tRSA hash %+v: %+v", transmissionRSAHash, err)
			}
		} else {
			jww.INFO.Printf("Notified ephemeral ID %+v [%+v] and received response %+v", ephID, token, resp)
		}
	}
}

// RegisterForNotifications is called by the client, and adds a user registration to our database
func (nb *Impl) RegisterForNotifications(request *pb.NotificationRegisterRequest) error {
	var err error
	// Check auth & inputs
	if string(request.Token) == "" {
		return errors.New("Cannot register for notifications with empty client token")
	}

	// Verify permissioning RSA signature
	permHost, ok := nb.Comms.GetHost(&id.Permissioning)
	if !ok {
		return errors.New("Could not find permissioning host to verify client signature")
	}
	err = registration.VerifyWithTimestamp(permHost.GetPubKey(), request.RegistrationTimestamp,
		string(request.TransmissionRsa), request.TransmissionRsaSig)
	if err != nil {
		return errors.WithMessage(err, "Failed to verify perm sig with timestamp")
	}

	// Verify IID transmission RSA signature
	h, err := hash.NewCMixHash()
	if err != nil {
		return errors.WithMessage(err, "Failed to create cmix hash")
	}
	_, err = h.Write(request.IntermediaryId)
	if err != nil {
		return errors.Wrap(err, "Failed to write intermediary id to hash")
	}
	pub, err := rsa.LoadPublicKeyFromPem(request.TransmissionRsa)
	if err != nil {
		return errors.WithMessage(err, "Failed to load public key from bytes")
	}
	err = rsa.Verify(pub, hash.CMixHash, h.Sum(nil), request.IIDTransmissionRsaSig, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to verify IID signature from client")
	}

	// Add the user to storage
	u, err := nb.Storage.AddUser(request.IntermediaryId, request.TransmissionRsa, request.IIDTransmissionRsaSig, request.Token)
	if err != nil {
		return errors.Wrap(err, "Failed to register user with notifications")
	}
	_, epoch := ephemeral.HandleQuantization(time.Now())
	def := nb.inst.GetPartialNdf()
	// FIXME: Does the address space need more logic here?
	e, err := nb.Storage.AddLatestEphemeral(u, epoch, uint(def.Get().AddressSpace[0].Size))
	if err != nil {
		return errors.WithMessage(err, "Failed to add ephemeral ID for user")
	}
	jww.INFO.Printf("Added ephemeral ID %+v for user %+v", e.EphemeralId, u.IntermediaryId)

	return nil
}

// UnregisterForNotifications is called by the client, and removes a user registration from our database
func (nb *Impl) UnregisterForNotifications(request *pb.NotificationUnregisterRequest) error {
	h, err := hash.NewCMixHash()
	if err != nil {
		return errors.WithMessage(err, "Failed to create cmix hash")
	}
	_, err = h.Write(request.IntermediaryId)
	if err != nil {
		return errors.WithMessage(err, "Failed to write intermediary id to hash")
	}

	u, err := nb.Storage.GetUser(request.IntermediaryId)
	if err != nil {
		return errors.WithMessagef(err, "Failed to find user with intermediary ID %+v", request.IntermediaryId)
	}

	pub, err := rsa.LoadPublicKeyFromPem(u.TransmissionRSA)
	if err != nil {
		return errors.WithMessage(err, "Failed to load public key from database")
	}
	err = rsa.Verify(pub, hash.CMixHash, h.Sum(nil), request.IIDTransmissionRsaSig, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to verify IID signature from client")
	}
	err = nb.Storage.DeleteUserByHash(u.TransmissionRSAHash)
	if err != nil {
		return errors.Wrap(err, "Failed to unregister user with notifications")
	}
	return nil
}

// ReceiveNotificationBatch receives the batch of notification data from gateway.
func (nb *Impl) ReceiveNotificationBatch(notifBatch *pb.NotificationBatch, auth *connect.Auth) error {
	//if !auth.IsAuthenticated {
	//	return errors.New("Cannot receive notification data: client is not authenticated")
	//}

	rid := notifBatch.RoundID

	_, loaded := nb.roundStore.LoadOrStore(rid, time.Now())
	if loaded {
		jww.DEBUG.Printf("Dropping duplicate notification batch for round %+v", notifBatch.RoundID)
		return nil
	}

	jww.INFO.Printf("Received notification batch for round %+v", notifBatch.RoundID)

	buffer := nb.Storage.GetNotificationBuffer()
	data := processNotificationBatch(notifBatch)
	buffer.Add(id.Round(notifBatch.RoundID), data)

	return nil
}

func processNotificationBatch(l *pb.NotificationBatch) []*notifications.Data {
	var res []*notifications.Data
	for _, item := range l.Notifications {
		res = append(res, &notifications.Data{
			EphemeralID: item.EphemeralID,
			RoundID:     l.RoundID,
			IdentityFP:  item.IdentityFP,
			MessageHash: item.MessageHash,
		})
	}
	return res
}

func (nb *Impl) ReceivedNdf() *uint32 {
	return nb.receivedNdf
}

func (nb *Impl) Cleaner() {
	cleanF := func(key, val interface{}) bool {
		t := val.(time.Time)
		if time.Since(t) > (5 * time.Minute) {
			nb.roundStore.Delete(key)
		}
		return true
	}

	cleanTicker := time.NewTicker(time.Minute * 10)

	for {
		select {
		case <-cleanTicker.C:
			nb.roundStore.Range(cleanF)
		}
	}
}

func (nb *Impl) Sender(sendFreq int) {
	sendTicker := time.NewTicker(time.Duration(sendFreq) * time.Second)
	for {
		select {
		case <-sendTicker.C:
			go func() {
				notifBuf := nb.Storage.GetNotificationBuffer()
				notifMap := notifBuf.Swap()
				unsent := map[uint64][]*notifications.Data{}
				for ephID := range notifMap {
					localEphID := ephID
					notifList := notifMap[localEphID]
					rest, err := nb.notifyFunc(localEphID, notifList, nb.apnsClient, nb.fcm, nb.Storage, nb.maxNotifications, nb.maxPayloadBytes)
					if err != nil {
						jww.ERROR.Printf("Failed to notify %d: %+v", localEphID, err)
					}
					for _, n := range rest {
						unsent[n.RoundID] = append(unsent[n.RoundID], n)
					}
				}
				for rid, nd := range unsent {
					notifBuf.Add(id.Round(rid), nd)
				}
			}()
		}
	}
}
