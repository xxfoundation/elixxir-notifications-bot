package firebase

import (
	"firebase.google.com/go/messaging"
	"github.com/pkg/errors"

	"golang.org/x/net/context"

	firebase "firebase.google.com/go"

	"google.golang.org/api/option"
)

func setupApp(serviceKeyPath string) (*messaging.Client, context.Context, error) {
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

func SendNotification(token []byte, serviceKeyPath string) (string, error) {
	app, ctx, err := setupApp(serviceKeyPath)
	if err != nil {
		return "", errors.Errorf("Failed to get app messaging client: %+v", err)
	}

	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: "xx Messenger",
			Body:  "You have a new message in the xx Messenger",
		},
		Token: string(token),
	}

	resp, err := app.Send(ctx, message)
	if err != nil {
		return "", errors.Errorf("Failed to send notification: %+v", err)
	}
	return resp, nil
}
