////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles creating client registration callbacks for hooking into comms library

package notifications

import (
	"crypto/x509"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/connect"
	"gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/notificationBot"
	"gitlab.com/elixxir/crypto/signature/rsa"
	"gitlab.com/elixxir/crypto/tls"
	"gitlab.com/elixxir/notifications-bot/firebase"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/elixxir/primitives/utils"
	"time"
)

type PollFunc func(*connect.Host, *notificationBot.Comms) ([]string, error)
type NotifyFunc func(string, string, *firebase.FirebaseComm, storage.Storage) (string, error)

type Params struct {
	Address       string
	CertPath      string
	KeyPath       string
	PublicAddress string
}

type Impl struct {
	Comms            *notificationBot.Comms
	Storage          storage.Storage
	notificationCert *x509.Certificate
	notificationKey  *rsa.PrivateKey
	certFromFile     string
	gatewayHost      *connect.Host // TODO: populate this field
	pollFunc         PollFunc      // TODO: implement function which polls gateway for UID
	notifyFunc       NotifyFunc    // TODO: implement function which notifies user with given UID
}

func RunNotificationLoop(fbCreds string, impl *Impl, loopDuration int) {
	fc := firebase.NewFirebaseComm()
	for {
		// TODO: fill in body of main loop, should poll gateway and send relevant notifications to firebase
		UIDs, err := impl.pollFunc(impl.gatewayHost, impl.Comms)
		if err != nil {
			jww.ERROR.Printf("Failed to poll gateway for users to notify: %+v", err)
		}

		for _, id := range UIDs {
			_, err := impl.notifyFunc(id, fbCreds, fc, impl.Storage)
			if err != nil {
				jww.ERROR.Printf("Failed to notify user with ID %+v: %+v", id, err)
			}
		}

		time.Sleep(time.Second * time.Duration(loopDuration))
	}
}

func StartNotifications(params Params, noTLS bool) *Impl {
	impl := &Impl{}

	var cert, key []byte
	var err error

	// Read in private key
	key, err = utils.ReadFile(params.KeyPath)
	if err != nil {
		jww.ERROR.Printf("failed to read key at %+v: %+v", params.KeyPath, err)
	}
	impl.notificationKey, err = rsa.LoadPrivateKeyFromPem(key)
	if err != nil {
		jww.ERROR.Printf("Failed to parse notifications server key: %+v. "+
			"NotificationsKey is %+v",
			err, impl.notificationKey)
	}

	if !noTLS {
		// Read in TLS keys from files
		cert, err = utils.ReadFile(params.CertPath)
		if err != nil {
			jww.ERROR.Printf("failed to read certificate at %+v: %+v", params.CertPath, err)
		}
		// Set globals for notification server
		impl.certFromFile = string(cert)
		impl.notificationCert, err = tls.LoadCertificate(string(cert))
		if err != nil {
			jww.ERROR.Printf("Failed to parse notifications server cert: %+v. "+
				"Notifications cert is %+v",
				err, impl.notificationCert)
		}
	}

	impl.pollFunc = pollForNotifications
	impl.notifyFunc = notifyUser

	return impl
}

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
func notifyUser(uid string, serviceKeyPath string, fc *firebase.FirebaseComm, db storage.Storage) (string, error) {
	u, err := db.GetUser(uid)
	if err != nil {
		return "", errors.Errorf("Failed to get token for UID %+v: %+v", uid, err)
	}

	app, ctx, err := fc.SetupMessagingApp(serviceKeyPath)
	if err != nil {
		return "", errors.Errorf("Failed to setup messaging app: %+v", err)
	}

	resp, err := fc.SendNotification(app, ctx, u.Token)
	if err != nil {
		return "", errors.Errorf("Failed to send notification to user with ID %+v: %+v", uid, err)
	}
	return resp, nil
}

func pollForNotifications(h *connect.Host, comms *notificationBot.Comms) (strings []string, e error) {
	users, err := comms.RequestNotifications(h, &mixmessages.Ping{})
	if err != nil {
		return nil, errors.Errorf("Failed to retrieve notifications from gateway: %+v", err)
	}

	return users.IDs, nil
}

func (nb *Impl) RegisterForNotifications(clientToken []byte, auth *connect.Auth) error {
	// Implement this
	u := &storage.User{
		Id:    auth.Sender.GetId(),
		Token: string(clientToken),
	}
	err := nb.Storage.UpsertUser(u)
	if err != nil {
		return errors.Errorf("Failed to register user with notifications: %+v", err)
	}
	return nil
}

func (nb *Impl) UnregisterForNotifications(auth *connect.Auth) error {
	err := nb.Storage.DeleteUser(auth.Sender.GetId())
	if err != nil {
		return errors.Errorf("Failed to unregister user with notifications: %+v", err)
	}
	return nil
}
