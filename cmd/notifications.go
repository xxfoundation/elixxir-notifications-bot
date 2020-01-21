////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles creating client registration callbacks for hooking into comms library

package cmd

import (
	"crypto/x509"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/crypto/signature/rsa"
	"gitlab.com/elixxir/crypto/tls"
	"gitlab.com/elixxir/primitives/id"
	"gitlab.com/elixxir/primitives/ndf"
	"gitlab.com/elixxir/primitives/utils"
	"time"
)

type PollFunc func() ([]id.User, error)
type NotifyFunc func([]byte) error

type NotificationsImpl struct {
	//Comms                 *registration.Comms // TODO: replace with notification comms when available
	notificationCert *x509.Certificate
	notificationKey  *rsa.PrivateKey
	certFromFile     string
	pollGateway      *ndf.Gateway // TODO: populate this field
	pollFunc         PollFunc     // TODO: implement function which polls gateway for UID
	notifyFunc       NotifyFunc   // TODO: implement function which notifies user with given UID
}

type Params struct {
	Address       string
	CertPath      string
	KeyPath       string
	publicAddress string
}

func RunNotificationLoop(impl *NotificationsImpl, loopDuration int) {
	for {
		// TODO: fill in body of main loop, should poll gateway and send relevant notifications to firebase
		UIDs, err := impl.pollFunc()
		if err != nil {
			jww.ERROR.Printf("Failed to poll gateway for users to notify: %+v", err)
		}

		for _, id := range UIDs {
			err := impl.notifyFunc(id.Bytes())
			if err != nil {
				jww.ERROR.Printf("Failed to notify user with ID %+v: %+v", id, err)
			}
		}

		time.Sleep(time.Second * time.Duration(loopDuration))
	}
}

func StartNotifications(params Params) *NotificationsImpl {
	impl := &NotificationsImpl{}

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

	return impl
}
