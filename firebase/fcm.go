package firebase

import (
	"firebase.google.com/go/messaging"
	"github.com/pkg/errors"
	"testing"

	"golang.org/x/net/context"

	firebase "firebase.google.com/go"

	"google.golang.org/api/option"
)

type SetupFunc func(string) (*messaging.Client, context.Context, error)
type SendFunc func(FBSender, context.Context, string) (string, error)

type NotificationsBot struct {
	SetupMessagingApp SetupFunc
	SendNotification  SendFunc
}

type FBSender interface {
	Send(context.Context, *messaging.Message) (string, error)
}

// Set up a notificationbot object with the proper setup and send functions
func NewNotificationsBot() *NotificationsBot {
	return &NotificationsBot{
		SetupMessagingApp: setupMessagingApp,
		SendNotification:  sendNotification,
	}
}

// FOR TESTING USE ONLY: setup a notificationbot object with mocked setup and send funcs
func NewMockNotificationsBot(t *testing.T, setupFunc SetupFunc, sendFunc SendFunc) *NotificationsBot {
	if t == nil {
		panic("This method should only be used in tests")
	}
	return &NotificationsBot{
		SetupMessagingApp: setupFunc,
		SendNotification:  sendFunc,
	}
}

// setupApp is a helper function which sets up a connection with firebase
// It returns a messaging client, a context object and an error
func setupMessagingApp(serviceKeyPath string) (*messaging.Client, context.Context, error) {
	ctx := context.Background()
	opt := option.WithCredentialsFile(serviceKeyPath)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, nil, errors.Errorf("Error initializing app: %v", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, nil, errors.Errorf("Error getting Messaging app: %+v", err)
	}

	return client, ctx, nil
}

// SendNotification accepts a registration token and service account file
// It gets the proper infrastructure, then builds & sends a notification through the firebase admin API
// returns string, error (string is of dubious use, but is returned for the time being)
func sendNotification(app FBSender, ctx context.Context, token string) (string, error) {
	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: "xx Messenger",
			Body:  "You have a new message in the xx Messenger",
		},
		Token: token,
	}

	resp, err := app.Send(ctx, message)
	if err != nil {
		return "", errors.Errorf("Failed to send notification: %+v", err)
	}
	return resp, nil
}

// NotifyUser accepts a UID and service key file path.
// It handles the logic involved in retrieving a user's token and sending the notification
func (nb *NotificationsBot) NotifyUser(uid []byte, serviceKeyPath string) (string, error) {
	// TODO: replace this, should be retreiving token from the database
	token, err := GetTokenByUID(uid)
	if err != nil {
		return "", errors.Errorf("Failed to get token for UID %+v: %+v", uid, err)
	}

	app, ctx, err := nb.SetupMessagingApp(serviceKeyPath)
	if err != nil {
		return "", errors.Errorf("Failed to setup messaging app: %+v", err)
	}

	resp, err := nb.SendNotification(app, ctx, token)
	if err != nil {
		return "", errors.Errorf("Failed to send notification to user with ID %+v: %+v", uid, err)
	}
	return resp, nil
}

// TODO: DELETE TEMP METHOD delete this method once database is finished
func GetTokenByUID(uid []byte) (string, error) {
	return string(uid), nil
}
