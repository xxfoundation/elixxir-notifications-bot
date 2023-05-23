package notifications

import (
	"gitlab.com/elixxir/notifications-bot/constants"
	"gitlab.com/elixxir/notifications-bot/notifications/providers"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/elixxir/primitives/notifications"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"sync"
	"testing"
	"time"
)

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
