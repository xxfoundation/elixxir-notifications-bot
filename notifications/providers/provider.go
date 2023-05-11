package providers

import "gitlab.com/elixxir/notifications-bot/storage"

// Provider interface represents a notifications provider, implementing an
// easy-to-use Notify function for the rest of the repo to call.
type Provider interface {
	// Notify sends a notification and returns the token status and an error
	Notify(csv string, target storage.GTNResult) (bool, error)
}
