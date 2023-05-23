////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package providers

import (
	"context"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/notifications-bot/storage"
	"google.golang.org/api/option"
	"strings"
	"time"
)

// fcm struct representing Firebase cloud messaging providers
type fcm struct {
	client *messaging.Client
}

// NewFCM returns an FCM-backed provider interface.
func NewFCM(serviceKeyPath string) (Provider, error) {
	ctx := context.Background()
	opt := option.WithCredentialsFile(serviceKeyPath)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, errors.Errorf("Error initializing app: %v", err)
	}

	cl, err := app.Messaging(ctx)
	if err != nil {
		return nil, errors.Errorf("Error getting Messaging app: %+v", err)
	}

	return &fcm{
		client: cl,
	}, nil
}

// Notify implements the Provider interface for FCM, sending the notifications to the provider.
func (f *fcm) Notify(csv string, target storage.GTNResult) (bool, error) {
	ctx := context.Background()
	ttl := 7 * 24 * time.Hour
	message := &messaging.Message{
		Data: map[string]string{
			"notificationsTag": csv, // TODO: swap to notificationsTag constant from notifications package (move to avoid circular dep)
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			TTL:      &ttl,
		},
		Token: target.Token,
	}

	resp, err := f.client.Send(ctx, message)
	if err != nil {
		// Check token validity
		validToken := true
		invalidToken := strings.Contains(err.Error(), "400") &&
			strings.Contains(err.Error(), "Invalid registration")

		if strings.Contains(err.Error(), "404") || invalidToken {
			validToken = false
			err = errors.WithMessagef(err, "Failed to notify user with Transmission RSA hash %+v due to invalid token", target.TransmissionRSAHash)
		} else {
			err = errors.WithMessagef(err, "Failed to notify user with Transmission RSA hash %+v", target.TransmissionRSAHash)
		}

		return validToken, err
	}
	jww.DEBUG.Printf("Notified ephemeral ID %+v [%+v] via fcm and received response %+v", target.EphemeralId, target.Token, resp)
	return true, nil
}
