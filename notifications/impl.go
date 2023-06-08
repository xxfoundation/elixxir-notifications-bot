////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Package notifications contains the core logic for interacting with the notifications bot.
//
// This includes registering users, receiving notifications, and sending to providers.

package notifications

import (
	"crypto/tls"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/notificationBot"
	"gitlab.com/elixxir/notifications-bot/constants"
	"gitlab.com/elixxir/notifications-bot/notifications/providers"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/ndf"
	"gitlab.com/xx_network/primitives/netTime"
	"gitlab.com/xx_network/primitives/utils"
	"sync"
)

// Impl for notifications; holds comms, storage object, creds and main functions
type Impl struct {
	Comms            *notificationBot.Comms
	Storage          *storage.Storage
	inst             *network.Instance
	receivedNdf      *uint32
	roundStore       sync.Map
	maxNotifications int
	maxPayloadBytes  int

	providers map[string]providers.Provider

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
		providers:        map[string]providers.Provider{},
		receivedNdf:      &receivedNdf,
		maxNotifications: params.NotificationsPerBatch,
		maxPayloadBytes:  params.MaxNotificationPayload,
	}

	// Set up firebase messaging client
	if !noFirebase {
		impl.providers[constants.MessengerAndroid.String()], err = providers.NewFCM(params.FBCreds)
		if err != nil {
			jww.WARN.Printf("Failed to start firebase provider for %s", constants.MessengerAndroid)
		}

		if params.HavenFBCreds != "" {
			impl.providers[constants.HavenAndroid.String()], err = providers.NewFCM(params.HavenFBCreds)
			if err != nil {
				jww.WARN.Printf("Failed to start firebase provider for %s", constants.HavenAndroid)
			}
		}
	}

	if params.KeyPath == "" {
		jww.WARN.Println("WARNING: RUNNING WITHOUT APNS")
	} else {
		impl.providers[constants.MessengerIOS.String()], err = providers.NewApns(params.APNS)
		if err != nil {
			jww.WARN.Printf("Failed to start APNS provider for %s", constants.MessengerIOS)
		}
	}

	if params.HavenAPNS.KeyPath == "" {
		jww.WARN.Println("WARNING: RUNNING WITHOUT HAVEN APNS")
	} else {
		impl.providers[constants.HavenIOS.String()], err = providers.NewApns(params.HavenAPNS)
		if err != nil {
			jww.WARN.Printf("Failed to start APNS provider for %s", constants.HavenIOS)
		}
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

	go func() {
		if params.HttpsKeyPath == "" || params.HttpsCertPath == "" {
			jww.WARN.Println("Running without HTTPS")
			return
		}
		httpsCertificate, err := tls.LoadX509KeyPair(params.HttpsCertPath, params.HttpsKeyPath)
		if err != nil {
			jww.ERROR.Printf("Failed to load https certificate: %+v", err)
			return
		}
		err = comms.ServeHttps(httpsCertificate)
		if err != nil {
			jww.ERROR.Printf("Failed to serve HTTPS: %+v", err)
		}
	}()

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
	impl.Functions.RegisterToken = func(msg *pb.RegisterTokenRequest) error {
		err := instance.RegisterToken(msg)
		if err != nil {
			jww.ERROR.Printf("Failed to RegisterToken: %+v", err)
		}
		return err
	}
	impl.Functions.RegisterTrackedID = func(msg *pb.RegisterTrackedIdRequest) error {
		err := instance.RegisterTrackedID(msg)
		if err != nil {
			jww.ERROR.Printf("Failed to RegisterTrackedID: %+v", err)
		}
		return err
	}
	impl.Functions.UnregisterToken = func(msg *pb.UnregisterTokenRequest) error {
		err := instance.UnregisterToken(msg)
		if err != nil {
			jww.ERROR.Printf("Failed to UnregisterToken: %+v", err)
		}
		return err
	}
	impl.Functions.UnregisterTrackedID = func(msg *pb.UnregisterTrackedIdRequest) error {
		err := instance.UnregisterTrackedID(msg.Request)
		if err != nil {
			jww.ERROR.Printf("Failed to UnregisterTrackedID: %+v", err)
		}
		return err
	}

	return impl
}
