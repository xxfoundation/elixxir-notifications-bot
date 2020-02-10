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
	"gitlab.com/elixxir/comms/connect"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/notificationBot"
	"gitlab.com/elixxir/crypto/signature/rsa"
	"gitlab.com/elixxir/crypto/tls"
	"gitlab.com/elixxir/notifications-bot/firebase"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/elixxir/primitives/id"
	"gitlab.com/elixxir/primitives/ndf"
	"gitlab.com/elixxir/primitives/utils"
	"time"
)

// Function type definitions for the main operations (poll and notify)
type PollFunc func(*Impl) ([]string, error)
type NotifyFunc func(*messaging.Client, string, *firebase.FirebaseComm, storage.Storage) (string, error)

// Params struct holds info passed in for configuration
type Params struct {
	Address       string
	CertPath      string
	KeyPath       string
	PublicAddress string
	FBCreds       string
}

// Local impl for notifications; holds comms, storage object, creds and main functions
type Impl struct {
	Comms            NotificationComms
	Storage          storage.Storage
	notificationCert *x509.Certificate
	notificationKey  *rsa.PrivateKey
	certFromFile     string
	ndf              *ndf.NetworkDefinition
	pollFunc         PollFunc
	notifyFunc       NotifyFunc
	fcm              *messaging.Client
}

// We use an interface here inorder to allow us to mock the getHost and RequestNDF in the notifcationsBot.Comms for testing
type NotificationComms interface {
	GetHost(hostId string) (*connect.Host, bool)
	AddHost(id, address string, cert []byte, disableTimeout, enableAuth bool) (host *connect.Host, err error)
	RequestNotifications(host *connect.Host) (*pb.IDList, error)
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

		UIDs, err := nb.pollFunc(nb)
		if err != nil {
			errChan <- errors.Wrap(err, "Failed to poll gateway for users to notify")
			return
		}

		for _, id := range UIDs {
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
	impl := &Impl{}

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

	impl.pollFunc = pollForNotifications
	impl.notifyFunc = notifyUser

	handler := NewImplementation(impl)

	impl.Comms = notificationBot.StartNotificationBot(id.NOTIFICATION_BOT, params.PublicAddress, handler, cert, key)

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

	impl.Functions.RegisterForNotifications = func(clientToken []byte, auth *connect.Auth) error {
		return instance.RegisterForNotifications(clientToken, auth)
	}

	impl.Functions.UnregisterForNotifications = func(auth *connect.Auth) error {
		return instance.UnregisterForNotifications(auth)
	}

	return impl
}

// NotifyUser accepts a UID and service key file path.
// It handles the logic involved in retrieving a user's token and sending the notification
func notifyUser(fcm *messaging.Client, uid string, fc *firebase.FirebaseComm, db storage.Storage) (string, error) {
	u, err := db.GetUser(uid)
	if err != nil {
		jww.DEBUG.Printf("No registration found for user with ID %+v", uid)
		return "", nil
	}

	resp, err := fc.SendNotification(fcm, u.Token)
	if err != nil {
		return "", errors.Errorf("Failed to send notification to user with ID %+v: %+v", uid, err)
	}
	return resp, nil
}

// pollForNotifications accepts a gateway host and a RequestInterface (a comms object)
// It retrieves a list of user ids to be notified from the gateway
func pollForNotifications(nb *Impl) (strings []string, e error) {
	h, ok := nb.Comms.GetHost("gw")
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
func (nb *Impl) RegisterForNotifications(clientToken []byte, auth *connect.Auth) error {
	// Implement this
	u := &storage.User{
		Id:    auth.Sender.GetId(),
		Token: string(clientToken),
	}
	err := nb.Storage.UpsertUser(u)
	if err != nil {
		return errors.Wrap(err, "Failed to register user with notifications")
	}
	return nil
}

// UnregisterForNotifications is called by the client, and removes a user registration from our database
func (nb *Impl) UnregisterForNotifications(auth *connect.Auth) error {
	err := nb.Storage.DeleteUser(auth.Sender.GetId())
	if err != nil {
		return errors.Wrap(err, "Failed to unregister user with notifications")
	}
	return nil
}

func (nb *Impl) UpdateNdf(ndf *ndf.NetworkDefinition) error {
	gw := ndf.Gateways[len(ndf.Gateways)-1]
	_, err := nb.Comms.AddHost("gw", gw.Address, []byte(gw.TlsCertificate), false, true)
	if err != nil {
		return errors.Wrap(err, "Failed to add gateway host from NDF")
	}

	nb.ndf = ndf
	return nil
}
