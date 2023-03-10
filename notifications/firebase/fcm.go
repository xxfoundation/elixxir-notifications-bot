////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package firebase

import (
	"firebase.google.com/go/messaging"
	"github.com/pkg/errors"
	"testing"
	"time"

	"golang.org/x/net/context"

	firebase "firebase.google.com/go"

	"google.golang.org/api/option"
)

// function types for use in notificationsbot struct
type SendFunc func(FBSender, string, string) (string, error)

// FirebaseComm is a struct which holds the functions to setup the messaging app and sending notifications
// Using a struct in this manner allows us to properly unit test the NotifyUser function
type FirebaseComm struct {
	SendNotification SendFunc
	*messaging.Client
}

// FBSender is an interface which matches the send function in the messaging app, allowing us to unit test sendNotification
type FBSender interface {
	Send(context.Context, *messaging.Message) (string, error)
}

// NewFirebaseComm create a *FirebaseComm object with the proper setup and send functions
func NewFirebaseComm(cl *messaging.Client) *FirebaseComm {
	return &FirebaseComm{
		SendNotification: sendNotification,
		Client:           cl,
	}
}

// NewMockFirebaseComm FOR TESTING USE ONLY: create a *FirebaseComm object with mocked setup and send funcs
func NewMockFirebaseComm(t *testing.T, sendFunc SendFunc) *FirebaseComm {
	if t == nil {
		panic("This method should only be used in tests")
	}
	return &FirebaseComm{
		SendNotification: sendFunc,
	}
}

// SetupMessagingApp initializes communication with firebase
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

// sendNotification accepts a registration token and service account file
// It gets the proper infrastructure, then builds & sends a notification through the firebase admin API
// returns string, error (string is of dubious use, but is returned for the time being)
func sendNotification(app FBSender, token string, data string) (string, error) {
	ctx := context.Background()
	ttl := 7 * 24 * time.Hour
	message := &messaging.Message{
		Data: map[string]string{
			"notificationsTag": data, // TODO: swap to notificationsTag constant from notifications package (move to avoid circular dep)
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			TTL:      &ttl,
		},
		Token: token,
	}

	resp, err := app.Send(ctx, message)
	if err != nil {
		return resp, errors.Wrapf(err, "Failed to send notification.  Response: %+v", resp)
	}
	return resp, nil
}
