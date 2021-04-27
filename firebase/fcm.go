////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////
package firebase

import (
	"firebase.google.com/go/messaging"
	"github.com/pkg/errors"
	"gitlab.com/elixxir/comms/mixmessages"
	"testing"

	"golang.org/x/net/context"

	firebase "firebase.google.com/go"

	"google.golang.org/api/option"
)

// function types for use in notificationsbot struct
type SetupFunc func(string) (*messaging.Client, context.Context, error)
type SendFunc func(FBSender, string, *mixmessages.NotificationData) (string, error)

// FirebaseComm is a struct which holds the functions to setup the messaging app and sending notifications
// Using a struct in this manner allows us to properly unit test the NotifyUser function
type FirebaseComm struct {
	SendNotification SendFunc
}

// This interface matches the send function in the messaging app, allowing us to unit test sendNotification
type FBSender interface {
	Send(context.Context, *messaging.Message) (string, error)
}

// Set up a notificationbot object with the proper setup and send functions
func NewFirebaseComm() *FirebaseComm {
	return &FirebaseComm{
		SendNotification: sendNotification,
	}
}

// FOR TESTING USE ONLY: setup a notificationbot object with mocked setup and send funcs
func NewMockFirebaseComm(t *testing.T, sendFunc SendFunc) *FirebaseComm {
	if t == nil {
		panic("This method should only be used in tests")
	}
	return &FirebaseComm{
		SendNotification: sendFunc,
	}
}

// setupApp is a helper function which sets up a connection with firebase
// It returns a messaging client, a context object and an error
func SetupMessagingApp(serviceKeyPath string) (*messaging.Client, error) {
	ctx := context.Background()
	opt := option.WithCredentialsFile(serviceKeyPath)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, errors.Errorf("Error initializing app: %v", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, errors.Errorf("Error getting Messaging app: %+v", err)
	}

	return client, nil
}

// SendNotification accepts a registration token and service account file
// It gets the proper infrastructure, then builds & sends a notification through the firebase admin API
// returns string, error (string is of dubious use, but is returned for the time being)
func sendNotification(app FBSender, token string, data *mixmessages.NotificationData) (string, error) {
	ctx := context.Background()
	//message := &messaging.Message{
	//	Data: map[string]string{
	//		"MessageHash":         base64.StdEncoding.EncodeToString(data.MessageHash),
	//		"IdentityFingerprint": base64.StdEncoding.EncodeToString(data.IdentityFP),
	//	},
	//	APNS: &messaging.APNSConfig{
	//		Payload: &messaging.APNSPayload{
	//			Aps: &messaging.Aps{
	//				Alert: &messaging.ApsAlert{
	//					Title: "You have received an xx message",
	//					Body:  "encrypted",
	//				},
	//				MutableContent: true,
	//				Category:       "SECRET",
	//			},
	//			CustomData: map[string]interface{}{
	//				"MessageHash":         base64.StdEncoding.EncodeToString(data.MessageHash),
	//				"IdentityFingerprint": base64.StdEncoding.EncodeToString(data.IdentityFP),
	//			},
	//		},
	//	},
	//	Token: token,
	//}
	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: "Test Notification",
			Body:  "I'm a notification from the notification bot",
		},
		Token: token,
	}

	resp, err := app.Send(ctx, message)
	if err != nil {
		return resp, errors.Wrapf(err, "Failed to send notification.  Response: %+v", resp)
	}
	return resp, nil
}
