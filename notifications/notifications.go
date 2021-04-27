////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// This file contains the main logic for notifications, including the main implementation and core loop

package notifications

import (
	"firebase.google.com/go/messaging"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/notificationBot"
	"gitlab.com/elixxir/notifications-bot/firebase"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gitlab.com/xx_network/primitives/ndf"
	"gitlab.com/xx_network/primitives/utils"
	"gorm.io/gorm"
	"strings"
	"time"
)

// Function type definitions for the main operations (poll and notify)
type NotifyFunc func(*pb.NotificationData, *messaging.Client, *firebase.FirebaseComm, *storage.Storage) error

// Params struct holds info passed in for configuration
type Params struct {
	Address  string
	CertPath string
	KeyPath  string
	FBCreds  string
}

// Local impl for notifications; holds comms, storage object, creds and main functions
type Impl struct {
	Comms       *notificationBot.Comms
	Storage     *storage.Storage
	inst        *network.Instance
	notifyFunc  NotifyFunc
	fcm         *messaging.Client
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

	// Start notification comms server
	handler := NewImplementation(impl)
	comms := notificationBot.StartNotificationBot(&id.NotificationBot, params.Address, handler, cert, key)
	impl.Comms = comms
	i, err := network.NewInstance(impl.Comms.ProtoComms, &ndf.NetworkDefinition{AddressSpaceSize: 16}, nil, nil, network.None)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to start instance")
	}
	i.SetGatewayAuthentication()
	impl.inst = i

	return impl, nil
}

// NewImplementation
func NewImplementation(instance *Impl) *notificationBot.Implementation {
	impl := notificationBot.NewImplementation()

	impl.Functions.RegisterForNotifications = func(request *pb.NotificationRegisterRequest, auth *connect.Auth) error {
		return instance.RegisterForNotifications(request, auth)
	}

	impl.Functions.UnregisterForNotifications = func(request *pb.NotificationUnregisterRequest, auth *connect.Auth) error {
		return instance.UnregisterForNotifications(request, auth)
	}

	impl.Functions.ReceiveNotificationBatch = func(data *pb.NotificationBatch, auth *connect.Auth) error {
		return instance.ReceiveNotificationBatch(data, auth)
	}

	return impl
}

// NotifyUser accepts a UID and service key file path.
// It handles the logic involved in retrieving a user's token and sending the notification
func notifyUser(data *pb.NotificationData, fcm *messaging.Client, fc *firebase.FirebaseComm, db *storage.Storage) error {
	e, err := db.GetEphemeral(data.EphemeralID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			jww.DEBUG.Printf("No registration found for ephemeral ID %+v", data.EphemeralID)
			// This path is not an error.  if no results are returned, the user hasn't registered for notifications
			return nil
		}
		return errors.WithMessagef(err, "Could not retrieve registration for ephemeral ID %+v", data.EphemeralID)
	}

	u, err := db.GetUserByHash(e.TransmissionRSAHash)
	if err != nil {
		return errors.WithMessagef(err, "Failed to lookup user with tRSA hash %+v", e.TransmissionRSAHash)
	}

	_, err = fc.SendNotification(fcm, u.Token, data)
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
			return errors.WithMessagef(err, "Failed to send notification to user with tRSA hash %+v", u.TransmissionRSAHash)
		}
	}
	return nil
}

// RegisterForNotifications is called by the client, and adds a user registration to our database
func (nb *Impl) RegisterForNotifications(request *pb.NotificationRegisterRequest, auth *connect.Auth) error {
	var err error
	//if !auth.IsAuthenticated {
	//	return errors.New("Cannot register for notifications: client is not authenticated")
	//}
	//if string(request.Token) == "" {
	//	return errors.New("Cannot register for notifications with empty client token")
	//}
	//
	//h, err := hash.NewCMixHash()
	//if err != nil {
	//	return errors.Wrap(err, "Failed to create cmix hash")
	//}
	//_, err = h.Write(request.IntermediaryId)
	//if err != nil {
	//	return errors.Wrap(err, "Failed to write intermediary id to hash")
	//}
	//
	//err = rsa.Verify(auth.Sender.GetPubKey(), hash.CMixHash, h.Sum(nil), request.IIDTransmissionRsaSig, nil)
	//if err != nil {
	//	return errors.Wrap(err, "Failed to verify signature")
	//}

	// Add the user to storage
	u, err := nb.Storage.AddUser(request.IntermediaryId, request.TransmissionRsa,
		request.IIDTransmissionRsaSig, string(request.Token))
	if err != nil {
		return errors.Wrap(err, "Failed to register user with notifications")
	}
	_, epoch := ephemeral.HandleQuantization(time.Now())
	def := nb.inst.GetPartialNdf()
	e, err := nb.Storage.AddLatestEphemeral(u, epoch, uint(def.Get().AddressSpaceSize))
	if err != nil {
		return errors.WithMessage(err, "Failed to add ephemeral ID for user")
	}
	jww.INFO.Printf("Added ephemeral ID %+v for user %+v", e.EphemeralId, u.IntermediaryId)

	return nil
}

// UnregisterForNotifications is called by the client, and removes a user registration from our database
func (nb *Impl) UnregisterForNotifications(request *pb.NotificationUnregisterRequest, auth *connect.Auth) error {
	//if !auth.IsAuthenticated {
	//	return errors.New("Cannot unregister for notifications: client is not authenticated")
	//}
	//
	//h, err := hash.NewCMixHash()
	//if err != nil {
	//	return errors.Wrap(err, "Failed to create cmix hash")
	//}
	//_, err = h.Write(request.IntermediaryId)
	//if err != nil {
	//	return errors.Wrap(err, "Failed to write intermediary id to hash")
	//}
	//
	//err = rsa.Verify(auth.Sender.GetPubKey(), hash.CMixHash, h.Sum(nil), request.IIDTransmissionRsaSig, nil)
	//if err != nil {
	//	return errors.Wrap(err, "Failed to verify signature")
	//}

	err := nb.Storage.DeleteUser(rsa.CreatePublicKeyPem(auth.Sender.GetPubKey()))
	if err != nil {
		return errors.Wrap(err, "Failed to unregister user with notifications")
	}
	return nil
}

// ReceiveNotificationBatch receives the batch of notification data from gateway.
func (nb *Impl) ReceiveNotificationBatch(notifBatch *pb.NotificationBatch, auth *connect.Auth) error {
	if !auth.IsAuthenticated {
		return errors.New("Cannot receive notification data: client is not authenticated")
	}

	fbComm := firebase.NewFirebaseComm()
	for _, notifData := range notifBatch.GetNotifications() {
		err := nb.notifyFunc(notifData, nb.fcm, fbComm, nb.Storage)
		if err != nil {
			return err
		}
	}

	return nil
}

func (nb *Impl) ReceivedNdf() *uint32 {
	return nb.receivedNdf
}
