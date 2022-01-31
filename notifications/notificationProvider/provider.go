package notificationProvider

import "gitlab.com/elixxir/notifications-bot/storage"

type Provider interface {
	// Notify sends a notification and returns the token status and an error
	Notify(csv string, target storage.GTNResult) (bool, error)
}
