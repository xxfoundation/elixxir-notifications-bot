package notificationProvider

import (
	"encoding/base64"
	"github.com/pkg/errors"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	apnstoken "github.com/sideshow/apns2/token"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/notifications-bot/constants"
	"gitlab.com/elixxir/notifications-bot/storage"
	"time"
)

// APNSParams holds config info specific to apple's push notification service
type APNSParams struct {
	KeyPath  string
	KeyID    string
	Issuer   string
	BundleID string
	Dev      bool
}

type APNS struct {
	client *apns2.Client
	topic  string
}

func (a *APNS) Notify(csv string, target storage.GTNResult) (bool, error) {
	notifPayload := payload.NewPayload().AlertTitle(constants.NotificationTitle).AlertBody(
		constants.NotificationBody).MutableContent().Custom(
		constants.NotificationsTag, csv)
	notif := &apns2.Notification{
		CollapseID:  base64.StdEncoding.EncodeToString(target.TransmissionRSAHash),
		DeviceToken: target.Token,
		Expiration:  time.Now().Add(time.Hour * 24 * 7),
		Priority:    apns2.PriorityHigh,
		Payload:     notifPayload,
		PushType:    apns2.PushTypeAlert,
		Topic:       a.topic,
	}
	resp, err := a.client.Push(notif)
	if err != nil {
		return true, errors.WithMessagef(err, "Failed to send notification via APNS: %+v", resp)
		// TODO : Should be re-enabled for specific error cases? deep dive on apns docs may be helpful
		//err := db.DeleteUserByHash(u.TransmissionRSAHash)
		//if err != nil {
		//	return errors.WithMessagef(err, "Failed to remove user registration tRSA hash: %+v", u.TransmissionRSAHash)
		//}
	}
	jww.DEBUG.Printf("Notified ephemeral ID %+v [%+v] via APNS and received response %+v", target.EphemeralId, target.Token, resp)
	return true, nil
}

func NewAPNS(params APNSParams) (*APNS, error) {
	if params.KeyID == "" || params.Issuer == "" || params.BundleID == "" {
		return nil, errors.Errorf("APNS not properly configured: %+v", params)
	}

	authKey, err := apnstoken.AuthKeyFromFile(params.KeyPath)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to load auth key from file")
	}
	token := &apnstoken.Token{
		AuthKey: authKey,
		// KeyID from developer account (Certificates, Identifiers & Profiles -> Keys)
		KeyID: params.KeyID,
		// TeamID from developer account (View Account -> Membership)
		TeamID: params.Issuer,
	}
	apnsClient := apns2.NewTokenClient(token)
	if params.Dev {
		jww.INFO.Printf("Running with dev apns gateway")
		apnsClient.Development()
	} else {
		apnsClient.Production()
	}

	return &APNS{
		client: apnsClient,
		topic:  params.BundleID,
	}, nil
}
