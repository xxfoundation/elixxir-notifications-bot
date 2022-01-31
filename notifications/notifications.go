////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// This file contains the main logic for notifications, including the main implementation and core loop

package notifications

import (
	"gitlab.com/elixxir/notifications-bot/constants"
	"gitlab.com/elixxir/notifications-bot/notifications/notificationProvider"
	"sync"

	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/notificationBot"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/crypto/registration"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/elixxir/primitives/notifications"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gitlab.com/xx_network/primitives/ndf"
	"gitlab.com/xx_network/primitives/netTime"
	"gitlab.com/xx_network/primitives/utils"
	"time"
)

// Params struct holds info passed in for configuration
type Params struct {
	Address                string
	CertPath               string
	KeyPath                string
	FBCreds                string
	NotificationsPerBatch  int
	MaxNotificationPayload int
	NotificationRate       int
	APNS                   notificationProvider.APNSParams
}

// Impl for notifications; holds comms, storage object, creds and main functions
type Impl struct {
	Comms                 *notificationBot.Comms
	Storage               *storage.Storage
	inst                  *network.Instance
	notificationProviders map[constants.NotificationProvider]notificationProvider.Provider
	receivedNdf           *uint32
	roundStore            sync.Map
	maxNotifications      int
	maxPayloadBytes       int

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

	receivedNdf := uint32(0)

	impl := &Impl{
		receivedNdf:           &receivedNdf,
		maxNotifications:      params.NotificationsPerBatch,
		maxPayloadBytes:       params.MaxNotificationPayload,
		notificationProviders: map[constants.NotificationProvider]notificationProvider.Provider{},
	}

	if params.APNS.KeyPath == "" {
		jww.WARN.Println("WARNING: RUNNING WITHOUT APNS")
	} else {
		apnsProvider, err := notificationProvider.NewAPNS(params.APNS)
		if err != nil {
			return nil, errors.WithMessage(err, "Failed to create APNS provider")
		}
		impl.notificationProviders[constants.APNS] = apnsProvider
	}

	// Set up firebase messaging client
	if noFirebase {
		jww.WARN.Println("WARNING: RUNNING WITHOUT FIREBASE")
	} else {
		fcm, err := notificationProvider.NewFCM(params.FBCreds)
		if err != nil {
			return nil, errors.WithMessage(err, "Failed to create FCM provider")
		}
		impl.notificationProviders[constants.FCM] = fcm
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

// SendBatch accepts the map of ephemeralID:list[notifications.Data]
// It handles logic for building the CSV & sending to devices
func (nb *Impl) SendBatch(data map[int64][]*notifications.Data) ([]*notifications.Data, error) {
	csvs := map[int64]string{}
	var ephemerals []int64
	var unsent []*notifications.Data
	for i, ilist := range data {
		var overflow, toSend []*notifications.Data
		if len(ilist) > nb.maxNotifications {
			overflow = ilist[nb.maxNotifications:]
			toSend = ilist[:nb.maxNotifications]
		} else {
			toSend = ilist[:]
		}

		notifs, rest := notifications.BuildNotificationCSV(toSend, nb.maxPayloadBytes-len([]byte(constants.NotificationsTag)))
		overflow = append(overflow, rest...)
		csvs[i] = string(notifs)
		ephemerals = append(ephemerals, i)
		unsent = append(unsent, overflow...)
	}
	toNotify, err := nb.Storage.GetToNotify(ephemerals)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to get list of tokens to notify")
	}
	for i := range toNotify {
		go func(res storage.GTNResult) {
			nb.notify(csvs[res.EphemeralId], res)
		}(toNotify[i])
	}
	return unsent, nil
}

// notify is a helper function which handles sending notifications to either APNS or firebase
func (nb *Impl) notify(csv string, toNotify storage.GTNResult) {
	targetProvider := constants.NotificationProvider(toNotify.NotificationProvider)
	jww.INFO.Printf("Notifying ephemeral ID %d via %s to token %s", toNotify.EphemeralId, targetProvider.String(), toNotify.Token)
	provider, ok := nb.notificationProviders[targetProvider]
	if !ok {
		jww.ERROR.Printf("NO PROVIDER CONFIGURED FOR TARGET %s", targetProvider.String())
	}
	tokenOK, err := provider.Notify(csv, toNotify)
	if err != nil {
		jww.ERROR.Println(err)
		if !tokenOK {
			jww.DEBUG.Printf("User with tRSA hash %+v has invalid token [%+v] - attempting to remove", toNotify.TransmissionRSAHash, toNotify.Token)
			err := nb.Storage.DeleteUserByHash(toNotify.TransmissionRSAHash)
			if err != nil {
				jww.ERROR.Printf("Failed to remove user registration tRSA hash %+v: %+v", toNotify.TransmissionRSAHash, err)
			}
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
	u, err := nb.Storage.AddUser(request.IntermediaryId, request.TransmissionRsa, request.IIDTransmissionRsaSig, request.Token, constants.FCM)
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
				// Retreive & swap notification buffer
				notifBuf := nb.Storage.GetNotificationBuffer()
				notifMap := notifBuf.Swap()

				unsent := map[uint64][]*notifications.Data{}
				rest, err := nb.SendBatch(notifMap)
				if err != nil {
					jww.ERROR.Printf("Failed to send notification batch: %+v", err)
					// If we fail to run SendBatch, put everything back in unsent
					for _, elist := range notifMap {
						for _, n := range elist {
							unsent[n.RoundID] = append(unsent[n.RoundID], n)
						}
					}
				} else {
					// Loop through rest and add to unsent map
					for _, n := range rest {
						unsent[n.RoundID] = append(unsent[n.RoundID], n)
					}
				}
				// Re-add unsent notifications to the buffer
				for rid, nd := range unsent {
					notifBuf.Add(id.Round(rid), nd)
				}
			}()
		}
	}
}
