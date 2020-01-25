package notifications

import (
	"context"
	"firebase.google.com/go/messaging"
	"gitlab.com/elixxir/notifications-bot/firebase"
	"gitlab.com/elixxir/notifications-bot/storage"
	"testing"
)

type MockStorage struct{}

func (ms MockStorage) GetUser(userId string) (*storage.User, error) {
	return &storage.User{
		Id:    "test",
		Token: "test",
	}, nil
}

// Delete User from backend by primary key
func (ms MockStorage) DeleteUser(userId string) error {
	return nil
}

// Insert or Update User into backend
func (ms MockStorage) UpsertUser(user *storage.User) error {
	return nil
}

// Test notificationbot's notifyuser function; this mocks the setup and send functions, and only tests the core logic of this function
func TestNotifyUser(t *testing.T) {
	setup := func(string) (*messaging.Client, context.Context, error) {
		ctx := context.Background()
		return &messaging.Client{}, ctx, nil
	}
	send := func(firebase.FBSender, context.Context, string) (string, error) {
		return "", nil
	}
	fc := firebase.NewMockFirebaseComm(t, setup, send)

	_, err := notifyUser("test", "testpath", fc, MockStorage{})
	if err != nil {
		t.Errorf("Failed to notify user properly")
	}
}
