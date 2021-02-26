////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// This file contains the main logic for notifications, including the main implementation and core loop

package notifications

import (
	"crypto/x509"
	"firebase.google.com/go/messaging"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/notificationBot"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/notifications-bot/firebase"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/crypto/tls"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gitlab.com/xx_network/primitives/ndf"
	"gitlab.com/xx_network/primitives/utils"
	"strings"
	"sync"
	"time"
)

// Function type definitions for the main operations (poll and notify)
type PollFunc func(*Impl) ([][]byte, error)
type NotifyFunc func(*messaging.Client, []byte, *firebase.FirebaseComm, *storage.Storage) (string, error)

// Params struct holds info passed in for configuration
type Params struct {
	Address  string
	CertPath string
	KeyPath  string
	FBCreds  string
}

// Local impl for notifications; holds comms, storage object, creds and main functions
type Impl struct {
	Comms            NotificationComms
	Storage          *storage.Storage
	notificationCert *x509.Certificate
	notificationKey  *rsa.PrivateKey
	certFromFile     string
	ndf              *ndf.NetworkDefinition
	pollFunc         PollFunc
	notifyFunc       NotifyFunc
	fcm              *messaging.Client
	gwId             *id.ID

	offsets map[int64]*sync.Once
}

// We use an interface here in order to allow us to mock the getHost and RequestNDF in the notifcationsBot.Comms for testing
type NotificationComms interface {
	GetHost(hostId *id.ID) (*connect.Host, bool)
	AddHost(hid *id.ID, address string, cert []byte, params connect.HostParams) (host *connect.Host, err error)
	RequestNotifications(host *connect.Host) (*pb.UserIdList, error)
	RequestNdf(host *connect.Host, message *pb.NDFHash) (*pb.NDF, error)
}

// Main function for this repo accepts credentials and an impl
// loops continuously, polling for notifications and notifying the relevant users
func (nb *Impl) RunNotificationLoop(loopDuration int, killChan chan struct{}, errChan chan error) {
	fc := firebase.NewFirebaseComm()
	for {
		// Stop execution if killed by channel
		select {
		case <-killChan:
			return
		case <-time.After(time.Millisecond * time.Duration(loopDuration)):
		}

		// Poll for UIDs to notify
		UIDs, err := nb.pollFunc(nb)
		if err != nil {
			errChan <- errors.Wrap(err, "Failed to poll gateway for users to notify")
			return
		}

		for _, id := range UIDs {
			// Attempt to notify a given user (will not error if UID not registered)
			_, err := nb.notifyFunc(nb.fcm, id, fc, nb.Storage)
			if err != nil {
				errChan <- errors.Wrapf(err, "Failed to notify user with ID %+v", id)
				return
			}
		}
	}
}

// StartNotifications creates an Impl from the information passed in
func StartNotifications(params Params, noTLS, noFirebase bool) (*Impl, error) {
	impl := &Impl{
		offsets: map[int64]*sync.Once{},
	}

	var cert, key []byte
	var err error

	// Read in private key
	key, err = utils.ReadFile(params.KeyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read key at %+v", params.KeyPath)
	}
	impl.notificationKey, err = rsa.LoadPrivateKeyFromPem(key)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse notifications server key (%+v)", impl.notificationKey)
	}

	if !noTLS {
		// Read in TLS keys from files
		cert, err = utils.ReadFile(params.CertPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read certificate at %+v", params.CertPath)
		}
		// Set globals for notification server
		impl.certFromFile = string(cert)
		impl.notificationCert, err = tls.LoadCertificate(string(cert))
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse notifications server cert.  "+
				"Notifications cert is %+v", impl.notificationCert)
		}
	}

	// set up stored functions
	impl.pollFunc = pollForNotifications
	impl.notifyFunc = notifyUser

	// Start notification comms server
	handler := NewImplementation(impl)
	comms := notificationBot.StartNotificationBot(&id.NotificationBot, params.Address, handler, cert, key)
	impl.Comms = comms
	// Set up firebase messaging client
	if !noFirebase {
		app, err := firebase.SetupMessagingApp(params.FBCreds)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to setup firebase messaging app")
		}
		impl.fcm = app
	}

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

	return impl
}

// NotifyUser accepts a UID and service key file path.
// It handles the logic involved in retrieving a user's token and sending the notification
func notifyUser(fcm *messaging.Client, uid []byte, fc *firebase.FirebaseComm, db *storage.Storage) (string, error) {
	u, err := db.GetUser(uid)
	if err != nil {
		jww.DEBUG.Printf("No registration found for user with ID %+v", uid)
		// This path is not an error.  if no results are returned, the user hasn't registered for notifications
		return "", nil
	}

	resp, err := fc.SendNotification(fcm, u.Token)
	if err != nil {
		// Catch two firebase errors that we don't want to crash on
		// 403 and 404 indicate that the token stored is incorrect
		// this means rather than crashing we should log and unregister the user
		// Error documentation: https://firebase.google.com/docs/reference/fcm/rest/v1/ErrorCode
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "404") {
			jww.ERROR.Printf("User with ID %+v has invalid token, unregistering...", uid)
			err := db.DeleteUserByHash(u.TransmissionRSAHash)
			if err != nil {
				return "", errors.Wrapf(err, "Failed to remove user registration for uid: %+v", uid)
			}
		} else {
			return "", errors.Wrapf(err, "Failed to send notification to user with ID %+v", uid)
		}
	}
	return resp, nil
}

// pollForNotifications accepts a gateway host and a RequestInterface (a comms object)
// It retrieves a list of user ids to be notified from the gateway
func pollForNotifications(nb *Impl) (uids [][]byte, e error) {
	h, ok := nb.Comms.GetHost(nb.gwId)
	if !ok {
		return nil, errors.New("Could not find gateway host")
	}

	users, err := nb.Comms.RequestNotifications(h)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to retrieve notifications from gateway")
	}

	return users.IDs, nil
}

// RegisterForNotifications is called by the client, and adds a user registration to our database
func (nb *Impl) RegisterForNotifications(request *pb.NotificationRegisterRequest, auth *connect.Auth) error {
	var err error
	if !auth.IsAuthenticated {
		return errors.New("Cannot register for notifications: client is not authenticated")
	}
	if string(request.Token) == "" {
		return errors.New("Cannot register for notifications with empty client token")
	}

	h, err := hash.NewCMixHash()
	if err != nil {
		return errors.Wrap(err, "Failed to create cmix hash")
	}
	_, err = h.Write(request.IntermediaryId)
	if err != nil {
		return errors.Wrap(err, "Failed to write intermediary id to hash")
	}

	err = rsa.Verify(auth.Sender.GetPubKey(), hash.CMixHash, h.Sum(nil), request.IIDTransmissionRsaSig, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to verify signature")
	}

	// Add the user to storage
	u, err := nb.Storage.AddUser(request.IntermediaryId, request.TransmissionRsa,
		request.IIDTransmissionRsaSig, string(request.Token))
	if err != nil {
		return errors.Wrap(err, "Failed to register user with notifications")
	}

	err = nb.Storage.AddLatestEphemeral(u)
	if err != nil {
		return errors.WithMessage(err, "Failed to add ephemeral ID for user")
	}

	nb.TrackOffset(ephemeral.GetOffset(u.IntermediaryId))

	return nil
}

// UnregisterForNotifications is called by the client, and removes a user registration from our database
func (nb *Impl) UnregisterForNotifications(request *pb.NotificationUnregisterRequest, auth *connect.Auth) error {
	if !auth.IsAuthenticated {
		return errors.New("Cannot unregister for notifications: client is not authenticated")
	}

	h, err := hash.NewCMixHash()
	if err != nil {
		return errors.Wrap(err, "Failed to create cmix hash")
	}
	_, err = h.Write(request.IntermediaryId)
	if err != nil {
		return errors.Wrap(err, "Failed to write intermediary id to hash")
	}

	err = rsa.Verify(auth.Sender.GetPubKey(), hash.CMixHash, h.Sum(nil), request.IIDTransmissionRsaSig, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to verify signature")
	}

	err = nb.Storage.DeleteUser(rsa.CreatePublicKeyPem(auth.Sender.GetPubKey()))
	if err != nil {
		return errors.Wrap(err, "Failed to unregister user with notifications")
	}
	return nil
}

// Update stored NDF and add host for gateway to poll
func (nb *Impl) UpdateNdf(ndf *ndf.NetworkDefinition) error {
	gw := ndf.Gateways[len(ndf.Gateways)-1]
	var err error
	nb.gwId, err = id.Unmarshal(ndf.Nodes[len(ndf.Nodes)-1].ID)
	if err != nil {
		return errors.WithMessage(err, "Failed to unmarshal ID")
	}
	_, err = nb.Comms.AddHost(nb.gwId, gw.Address, []byte(gw.TlsCertificate), connect.GetDefaultHostParams())
	if err != nil {
		return errors.Wrap(err, "Failed to add gateway host from NDF")
	}

	nb.ndf = ndf
	return nil
}
