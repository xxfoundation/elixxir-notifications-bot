////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// This file contains the main logic for notifications, including the main implementation and core loop

package notifications

import (
	"encoding/base64"
	"firebase.google.com/go/messaging"
	"github.com/jonahh-yeti/apns"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/notificationBot"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/crypto/registration"
	"gitlab.com/elixxir/notifications-bot/firebase"
	"gitlab.com/elixxir/notifications-bot/storage"
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

// Function type definitions for the main operations (poll and notify)
type NotifyFunc func(*pb.NotificationData, ApnsSender, *messaging.Client, *firebase.FirebaseComm, *storage.Storage) error
type ApnsSender interface {
	Send(token string, p apns.Payload, opts ...apns.SendOption) (*apns.Response, error)
}

// Params struct holds info passed in for configuration
type Params struct {
	Address  string
	CertPath string
	KeyPath  string
	FBCreds  string
	APNS     APNSParams
}
type APNSParams struct {
	KeyPath  string
	KeyID    string
	Issuer   string
	BundleID string
}

// Local impl for notifications; holds comms, storage object, creds and main functions
type Impl struct {
	Comms       *notificationBot.Comms
	Storage     *storage.Storage
	inst        *network.Instance
	notifyFunc  NotifyFunc
	fcm         *messaging.Client
	apnsClient  *apns.Client
	receivedNdf *uint32

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
	var app *messaging.Client
	if !noFirebase {
		app, err = firebase.SetupMessagingApp(params.FBCreds)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to setup firebase messaging app")
		}
	}
	receivedNdf := uint32(0)

	impl := &Impl{
		notifyFunc:  notifyUser,
		fcm:         app,
		receivedNdf: &receivedNdf,
	}

	if params.APNS.KeyPath == "" {
		jww.WARN.Println("WARNING: RUNNING WITHOUT APNS")
	} else {
		if params.APNS.KeyID == "" || params.APNS.Issuer == "" || params.APNS.BundleID == "" {
			return nil, errors.WithMessagef(err, "APNS not properly configured: %+v", params.APNS)
		}
		apnsKey, err := utils.ReadFile(params.APNS.KeyPath)
		if err != nil {
			return nil, errors.WithMessage(err, "Failed to read APNS key")
		}

		apnsClient, err := apns.NewClient(
			apns.WithJWT(apnsKey, params.APNS.KeyID, params.APNS.Issuer),
			apns.WithBundleID(params.APNS.BundleID),
			apns.WithMaxIdleConnections(100),
			apns.WithTimeout(5*time.Second))
		if err != nil {
			return nil, errors.WithMessage(err, "Failed to setup apns client")
		}
		impl.apnsClient = apnsClient
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

// NotifyUser accepts a UID and service key file path.
// It handles the logic involved in retrieving a user's token and sending the notification
func notifyUser(data *pb.NotificationData, apnsClient ApnsSender, fcm *messaging.Client, fc *firebase.FirebaseComm, db *storage.Storage) error {
	elist, err := db.GetEphemeral(data.EphemeralID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			jww.DEBUG.Printf("No registration found for ephemeral ID %+v", data.EphemeralID)
			// This path is not an error.  if no results are returned, the user hasn't registered for notifications
			return nil
		}
		return errors.WithMessagef(err, "Could not retrieve registration for ephemeral ID %+v", data.EphemeralID)
	}
	for _, e := range elist {
		u, err := db.GetUserByHash(e.TransmissionRSAHash)
		if err != nil {
			return errors.WithMessagef(err, "Failed to lookup user with tRSA hash %+v", e.TransmissionRSAHash)
		}

		isAPNS := !strings.Contains(u.Token, ":")
		mutableContent := 1
		if isAPNS {
			jww.INFO.Printf("Notifying ephemeral ID %+v via APNS to token %+v", data.EphemeralID, u.Token)
			resp, err := apnsClient.Send(u.Token, apns.Payload{
				APS: apns.APS{
					Alert: apns.Alert{
						Title: "Privacy: protected!",
						Body:  "Some notifications are not for you to ensure privacy; we hope to remove this notification soon",
					},
					MutableContent: &mutableContent,
				},
				CustomValues: map[string]interface{}{
					"messagehash":         base64.StdEncoding.EncodeToString(data.MessageHash),
					"identityfingerprint": base64.StdEncoding.EncodeToString(data.IdentityFP),
				},
			}, apns.WithExpiration(604800), // 1 week
				apns.WithPriority(10),
				apns.WithCollapseID(base64.StdEncoding.EncodeToString(u.TransmissionRSAHash)),
				apns.WithPushType("alert"))
			if err != nil {
				jww.ERROR.Printf("Failed to send notification via APNS: %+v", err)
				err := db.DeleteUserByHash(u.TransmissionRSAHash)
				if err != nil {
					return errors.WithMessagef(err, "Failed to remove user registration tRSA hash: %+v", u.TransmissionRSAHash)
				}
			} else {
				jww.INFO.Printf("Notified ephemeral ID %+v [%+v] and received response %+v", data.EphemeralID, u.Token, resp)
			}
		} else {
			resp, err := fc.SendNotification(fcm, u.Token, data)
			if err != nil {
				// Catch two firebase errors that we don't want to crash on
				// 403 and 404 indicate that the token stored is incorrect
				// this means rather than crashing we should log and unregister the user
				// Error documentation: https://firebase.google.com/docs/reference/fcm/rest/v1/ErrorCode
				if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "404") {
					jww.ERROR.Printf("User with Transmission RSA hash %+v has invalid token, unregistering...", u.TransmissionRSAHash)
					err := db.DeleteUserByHash(u.TransmissionRSAHash)
					if err != nil {
						return errors.WithMessagef(err, "Failed to remove user registration tRSA hash: %+v", u.TransmissionRSAHash)
					}
				} else {
					jww.ERROR.Printf("Error sending notification: %+v", err)
					return errors.WithMessagef(err, "Failed to send notification to user with tRSA hash %+v", u.TransmissionRSAHash)
				}
			}
			jww.INFO.Printf("Notified ephemeral ID %+v [%+v] and received response %+v", data.EphemeralID, u.Token, resp)
		}
	}
	return nil
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

	jww.INFO.Printf("Received notification batch for round %+v", notifBatch.RoundID)

	fbComm := firebase.NewFirebaseComm()
	for _, notifData := range notifBatch.GetNotifications() {
		err := nb.notifyFunc(notifData, nb.apnsClient, nb.fcm, fbComm, nb.Storage)
		if err != nil {
			return err
		}
	}

	return nil
}

func (nb *Impl) ReceivedNdf() *uint32 {
	return nb.receivedNdf
}
