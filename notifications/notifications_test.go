////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package notifications

import (
	"fmt"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/notifications-bot/constants"
	"gitlab.com/elixxir/notifications-bot/notifications/providers"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/elixxir/primitives/notifications"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

var port = 4200

type MockProvider struct {
	donech chan string
}

func (mp *MockProvider) Notify(csv string, target storage.GTNResult) (bool, error) {
	mp.donech <- csv
	return true, nil
}

func TestImpl_SendBatch(t *testing.T) {
	// Init storage
	s, err := storage.NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to make new storage: %+v", err)
	}

	dchan := make(chan string, 10)
	// Init mock firebase comms
	//badsend := func(firebase.FBSender, string, string) (string, error) {
	//	return "", errors.New("Failed")
	//}

	// Create impl
	i := Impl{
		providers: map[string]providers.Provider{},
		Storage:   s,

		roundStore:       sync.Map{},
		maxNotifications: 0,
		maxPayloadBytes:  0,
	}

	i.providers[constants.MessengerAndroid.String()] = &MockProvider{donech: dchan}
	i.providers[constants.MessengerIOS.String()] = &MockProvider{donech: dchan}

	// Identity setup
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to create iid: %+v", err)
	}
	if err != nil {
		t.Errorf("Could not parse precanned time: %v", err.Error())
	}
	_, epoch := ephemeral.HandleQuantization(time.Now())
	_, err = s.RegisterForNotifications(iid, []byte("rsacert"), "fcm:token", constants.MessengerAndroid.String(), epoch, 16)
	if err != nil {
		t.Errorf("Failed to add fake user: %+v", err)
	}
	eph, err := s.GetLatestEphemeral()
	if err != nil {
		t.Fatal(err)
	}
	_, err = i.SendBatch(map[int64][]*notifications.Data{})
	if err != nil {
		t.Errorf("Error on sending empty batch: %+v", err)
	}

	unsent, err := i.SendBatch(map[int64][]*notifications.Data{
		eph.EphemeralId: {{EphemeralID: eph.EphemeralId, RoundID: 3, MessageHash: []byte("hello"), IdentityFP: []byte("identity")}},
	})
	if err != nil {
		t.Errorf("Error on sending small batch: %+v", err)
	}
	if len(unsent) < 1 {
		t.Errorf("Should have received notification back as unsent, instead got %+v", unsent)
	}

	i.maxPayloadBytes = 4096
	i.maxNotifications = 20
	unsent, err = i.SendBatch(map[int64][]*notifications.Data{
		1: {{EphemeralID: eph.EphemeralId, RoundID: 3, MessageHash: []byte("hello"), IdentityFP: []byte("identity")}},
	})
	if err != nil {
		t.Errorf("Error on sending small batch again: %+v", err)
	}
	if len(unsent) > 0 {
		t.Errorf("Should have received notification back as unsent, instead got %+v", unsent)
	}

	timeout := time.NewTicker(3 * time.Second)
	select {
	case <-dchan:
		t.Logf("Received on data chan!")
	case <-timeout.C:
		t.Errorf("Did not receive data before timeout")
	}
}

// Unit test for startnotifications
// tests logic including error cases
func TestStartNotifications(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get working dir: %+v", err)
		return
	}

	params := Params{
		Address:               "0.0.0.0:42010",
		NotificationsPerBatch: 20,
		NotificationRate:      30,
		APNS: providers.APNSParams{
			KeyPath:  "",
			KeyID:    "WQT68265C5",
			Issuer:   "S6JDM2WW29",
			BundleID: "io.xxlabs.messenger",
		},
	}

	params.KeyPath = wd + "/../testutil/cmix.rip.key"
	_, err = StartNotifications(params, false, true)
	if err == nil || !strings.Contains(err.Error(), "failed to read certificate at") {
		t.Errorf("Should have thrown an error for no cert path")
	}

	params.CertPath = wd + "/../testutil/cmix.rip.crt"
	_, err = StartNotifications(params, false, true)
	if err != nil {
		t.Errorf("Failed to start notifications successfully: %+v", err)
	}
}

// unit test for newimplementation
// tests logic and error cases
func TestNewImplementation(t *testing.T) {
	instance := getNewImpl()

	impl := NewImplementation(instance)
	if impl.Functions.RegisterForNotifications == nil || impl.Functions.UnregisterForNotifications == nil {
		t.Errorf("Functions were not properly set")
	}
}

// Happy path.
func TestImpl_ReceiveNotificationBatch(t *testing.T) {
	s, err := storage.NewStorage("", "", "", "", "")
	impl := &Impl{
		Storage:          s,
		roundStore:       sync.Map{},
		maxNotifications: 0,
		maxPayloadBytes:  0,
	}

	notifBatch := &pb.NotificationBatch{
		RoundID: 42,
		Notifications: []*pb.NotificationData{
			{
				EphemeralID: 5,
				IdentityFP:  []byte("IdentityFP"),
				MessageHash: []byte("MessageHash"),
			},
		},
	}

	auth := &connect.Auth{
		IsAuthenticated: true,
	}

	err = impl.ReceiveNotificationBatch(notifBatch, auth)
	if err != nil {
		t.Errorf("ReceiveNotificationBatch() returned an error: %+v", err)
	}

	nbm := impl.Storage.GetNotificationBuffer().Swap()
	if len(nbm[5]) < 1 {
		t.Errorf("Notification was not added to notification buffer: %+v", nbm[5])
	}
}

// func to get a quick new impl using test creds
func getNewImpl() *Impl {
	wd, _ := os.Getwd()
	params := Params{
		NotificationsPerBatch: 20,
		NotificationRate:      30,
		Address:               fmt.Sprintf("0.0.0.0:%d", port),
		KeyPath:               wd + "/../testutil/cmix.rip.key",
		CertPath:              wd + "/../testutil/cmix.rip.crt",
		FBCreds:               "",
	}
	port += 1
	instance, _ := StartNotifications(params, false, true)
	instance.Storage, _ = storage.NewStorage("", "", "", "", "")
	return instance
}
