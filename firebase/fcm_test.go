package firebase

import (
	"context"
	"firebase.google.com/go/messaging"
	"testing"
)

type MockSender struct{}

const token = "foIh7-NdlksspjDwT8O5kT:APA91bEQUCFeAadkIE-T3fHqAIIYwZm8lks0wQRIp5oh0qtMtjHcPjQhVZ3IDntZlv7PYAcHvDeu_7ncI8GcAlKama7YjzSLO9MgtAjxZMFivVfzQb-BD-6u0-MrJNR6XoOB9YX059ZB"

func (MockSender) Send(ctx context.Context, app *messaging.Message) (string, error) {
	return "test", nil
}

// This tests the function which sends a notification to firebase.
// Note: this requires you to have a valid token & service credentials
func TestSendNotification(t *testing.T) {
	ctx := context.Background()
	app := MockSender{}

	_, err := sendNotification(app, ctx, token)
	if err != nil {
		t.Error(err.Error())
	}
}

/*
 * This function can't be unit tested without mocking firebase's infrastructure to a degree that is counterproductive
func TestSetupMessagingApp(t *testing.T) {
	dir, _ := os.Getwd()
	_, _, err := setupMessagingApp(dir+"/../creds/serviceAccountKey.json")
	if err != nil {
		t.Errorf("Failed to setup messaging app: %+v", err)
	}
}
*/

// Test notificationbot's notifyuser function; this mocks the setup and send functions, and only tests the core logic of this function
func TestNotificationsBot_NotifyUser(t *testing.T) {
	setup := func(string) (*messaging.Client, context.Context, error) {
		ctx := context.Background()
		return &messaging.Client{}, ctx, nil
	}
	send := func(FBSender, context.Context, string) (string, error) {
		return "", nil
	}
	nb := NewMockNotificationsBot(t, setup, send)
	_, err := nb.NotifyUser([]byte("test"), "testpath")
	if err != nil {
		t.Errorf("Failed to notify user properly")
	}
}