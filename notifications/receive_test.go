package notifications

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/xx_network/comms/connect"
	"sync"
	"testing"
)

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
