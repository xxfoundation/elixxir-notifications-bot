package notifications

import (
	"fmt"
	"gitlab.com/elixxir/notifications-bot/notifications/providers"
	"gitlab.com/elixxir/notifications-bot/storage"
	"os"
	"strings"
	"testing"
)

var port = 4200

type MockProvider struct {
	donech chan string
}

func (mp *MockProvider) Notify(csv string, target storage.GTNResult) (bool, error) {
	mp.donech <- csv
	return true, nil
}

// Unit test for startnotifications
// tests logic including error cases
func TestStartNotifications(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get working dir: %+v", err)
		return
	}

	params := Params{
		Address:               "0.0.0.0:42010",
		NotificationsPerBatch: 20,
		NotificationRate:      30,
		APNS: providers.APNSParams{
			KeyPath:  "",
			KeyID:    "WQT68265C5",
			Issuer:   "S6JDM2WW29",
			BundleID: "io.xxlabs.messenger",
		},
	}

	params.KeyPath = wd + "/../testutil/cmix.rip.key"
	_, err = StartNotifications(params, false, true)
	if err == nil || !strings.Contains(err.Error(), "failed to read certificate at") {
		t.Errorf("Should have thrown an error for no cert path")
	}

	params.CertPath = wd + "/../testutil/cmix.rip.crt"
	_, err = StartNotifications(params, false, true)
	if err != nil {
		t.Errorf("Failed to start notifications successfully: %+v", err)
	}
}

// unit test for newimplementation
// tests logic and error cases
func TestNewImplementation(t *testing.T) {
	instance := getNewImpl()

	impl := NewImplementation(instance)
	if impl.Functions.RegisterForNotifications == nil || impl.Functions.UnregisterForNotifications == nil {
		t.Errorf("Functions were not properly set")
	}
}

// func to get a quick new impl using test creds
func getNewImpl() *Impl {
	wd, _ := os.Getwd()
	params := Params{
		NotificationsPerBatch: 20,
		NotificationRate:      30,
		Address:               fmt.Sprintf("0.0.0.0:%d", port),
		KeyPath:               wd + "/../testutil/cmix.rip.key",
		CertPath:              wd + "/../testutil/cmix.rip.crt",
		FBCreds:               "",
	}
	port += 1
	instance, _ := StartNotifications(params, false, true)
	instance.Storage, _ = storage.NewStorage("", "", "", "", "")
	return instance
}
