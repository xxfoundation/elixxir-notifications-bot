////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// This file contains the main logic for notifications, including the main implementation and core loop

package notifications

import (
	"crypto/tls"
	"gitlab.com/elixxir/notifications-bot/constants"
	"gitlab.com/elixxir/notifications-bot/notifications/providers"
	"sync"

	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/notificationBot"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/elixxir/primitives/notifications"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/ndf"
	"gitlab.com/xx_network/primitives/netTime"
	"gitlab.com/xx_network/primitives/utils"
	"time"
)

const notificationsTag = "notificationData"

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

		if params.HavenFBCreds != "" {
			impl.providers[constants.HavenAndroid.String()], err = providers.NewFCM(params.HavenFBCreds)
		}
	}

	if params.KeyPath == "" {
		jww.WARN.Println("WARNING: RUNNING WITHOUT APNS")
	} else {
		impl.providers[constants.MessengerIOS.String()], err = providers.NewApns(params.APNS)
	}

	if params.HavenAPNS.KeyPath == "" {
		jww.WARN.Println("WARNING: RUNNING WITHOUT HAVEN APNS")
	} else {
		impl.providers[constants.HavenIOS.String()], err = providers.NewApns(params.HavenAPNS)
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
		return instance.RegisterToken(msg)
	}
	impl.Functions.RegisterTrackedID = func(msg *pb.RegisterTrackedIdRequest) error {
		return instance.RegisterTrackedID(msg.Request)
	}
	impl.Functions.UnregisterToken = func(msg *pb.UnregisterTokenRequest) error {
		return instance.UnregisterToken(msg)
	}
	impl.Functions.UnregisterTrackedID = func(msg *pb.UnregisterTrackedIdRequest) error {
		return instance.UnregisterTrackedID(msg.Request)
	}

	return impl
}

// SendBatch accepts the map of ephemeralID:list[notifications.Data]
// It handles logic for building the CSV & sending to devices
func (nb *Impl) SendBatch(data map[int64][]*notifications.Data) ([]*notifications.Data, error) {
	csvs := map[int64]string{}
	var ephemerals []int64
	var unsent []*notifications.Data
	jww.INFO.Printf("data: %+v", data)
	for i, ilist := range data {
		var overflow, toSend []*notifications.Data
		if len(ilist) > nb.maxNotifications {
			overflow = ilist[nb.maxNotifications:]
			toSend = ilist[:nb.maxNotifications]
		} else {
			toSend = ilist[:]
		}

		notifs, rest := notifications.BuildNotificationCSV(toSend, nb.maxPayloadBytes-len([]byte(notificationsTag)))
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
	tokenValid, err := nb.providers[toNotify.App].Notify(csv, toNotify)
	if err != nil {
		jww.ERROR.Println(err)
		if !tokenValid {
			jww.DEBUG.Printf("User with tRSA hash %+v has invalid token [%+v] for app %s - attempting to remove", toNotify.TransmissionRSAHash, toNotify.Token, toNotify.App)
			err := nb.Storage.DeleteToken(toNotify.Token)
			if err != nil {
				jww.ERROR.Printf("Failed to remove %s token registration tRSA hash %+v: %+v", toNotify.App, toNotify.TransmissionRSAHash, err)
			}
		}
	}
}

// ReceiveNotificationBatch receives the batch of notification data from gateway.
func (nb *Impl) ReceiveNotificationBatch(notifBatch *pb.NotificationBatch, auth *connect.Auth) error {
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

				if len(notifMap) == 0 {
					return
				}

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
