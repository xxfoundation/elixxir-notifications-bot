package notifications

import (
	"context"
	"firebase.google.com/go/messaging"
	"github.com/pkg/errors"
	"gitlab.com/elixxir/comms/connect"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/notifications-bot/firebase"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/elixxir/notifications-bot/testutil"
	"gitlab.com/elixxir/primitives/utils"
	"os"
	"strings"
	"testing"
	"time"
)

// Basic test to cover RunNotificationLoop, including error sending
func TestRunNotificationLoop(t *testing.T) {
	impl := getNewImpl()
	impl.pollFunc = func(host *connect.Host, requestInterface RequestInterface) (i []string, e error) {
		return []string{"test1", "test2"}, nil
	}
	impl.notifyFunc = func(s3 string, s2 string, comm *firebase.FirebaseComm, storage storage.Storage) (s string, e error) {
		if s3 == "test1" {
			return "", errors.New("Failed to notify")
		}
		return "good", nil
	}
	killChan := make(chan struct{})
	go func() {
		time.Sleep(10 * time.Second)
		killChan <- struct{}{}
	}()
	impl.RunNotificationLoop("", 3, killChan)
}

// Test notificationbot's notifyuser function
// this mocks the setup and send functions, and only tests the core logic of this function
func TestNotifyUser(t *testing.T) {
	badsetup := func(string) (*messaging.Client, context.Context, error) {
		ctx := context.Background()
		return &messaging.Client{}, ctx, errors.New("Failed")
	}
	setup := func(string) (*messaging.Client, context.Context, error) {
		ctx := context.Background()
		return &messaging.Client{}, ctx, nil
	}
	badsend := func(firebase.FBSender, context.Context, string) (string, error) {
		return "", errors.New("Failed")
	}
	send := func(firebase.FBSender, context.Context, string) (string, error) {
		return "", nil
	}
	fc_badsetup := firebase.NewMockFirebaseComm(t, badsetup, send)
	fc_badsend := firebase.NewMockFirebaseComm(t, setup, badsend)
	fc := firebase.NewMockFirebaseComm(t, setup, send)

	_, err := notifyUser("test", "testpath", fc_badsetup, testutil.MockStorage{})
	if err == nil {
		t.Error("Should have returned an error")
		return
	}

	_, err = notifyUser("test", "testpath", fc_badsend, testutil.MockStorage{})
	if err == nil {
		t.Errorf("Should have returned an error")
	}

	_, err = notifyUser("test", "testpath", fc, testutil.MockStorage{})
	if err != nil {
		t.Errorf("Failed to notify user properly")
	}
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
		Address:       "0.0.0.0:42010",
		PublicAddress: "0.0.0.0:42010",
	}

	n, err := StartNotifications(params, false)
	if err == nil || !strings.Contains(err.Error(), "failed to read key at") {
		t.Errorf("Should have thrown an error for no key path")
	}

	params.KeyPath = wd + "/../testutil/badkey"
	n, err = StartNotifications(params, false)
	if err == nil || !strings.Contains(err.Error(), "Failed to parse notifications server key") {
		t.Errorf("Should have thrown an error bad key")
	}

	params.KeyPath = wd + "/../testutil/cmix.rip.key"
	n, err = StartNotifications(params, false)
	if err == nil || !strings.Contains(err.Error(), "failed to read certificate at") {
		t.Errorf("Should have thrown an error for no cert path")
	}

	params.CertPath = wd + "/../testutil/badkey"
	n, err = StartNotifications(params, false)
	if err == nil || !strings.Contains(err.Error(), "Failed to parse notifications server cert") {
		t.Errorf("Should have thrown an error for bad certificate")
	}

	params.CertPath = wd + "/../testutil/cmix.rip.crt"
	n, err = StartNotifications(params, false)
	if err != nil {
		t.Errorf("Failed to start notifications successfully: %+v", err)
	}
	if n.notificationKey == nil {
		t.Error("Did not set key")
	}
	if n.notificationCert == nil {
		t.Errorf("Did not set cert")
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

// Dummy comms to unit test pollfornotifications
type mockPollComm struct{}

func (m mockPollComm) RequestNotifications(host *connect.Host, message *mixmessages.Ping) (*mixmessages.IDList, error) {
	return &mixmessages.IDList{
		IDs: []string{"test"},
	}, nil
}

type mockPollErrComm struct{}

func (m mockPollErrComm) RequestNotifications(host *connect.Host, message *mixmessages.Ping) (*mixmessages.IDList, error) {
	return nil, errors.New("failed to poll")
}

// Unit test for PollForNotifications
func TestPollForNotifications(t *testing.T) {
	_, err := pollForNotifications(nil, mockPollErrComm{})
	if err == nil {
		t.Errorf("Failed to poll for notifications: %+v", err)
	}

	_, err = pollForNotifications(nil, mockPollComm{})
	if err != nil {
		t.Errorf("Failed to poll for notifications: %+v", err)
	}
}

// Unit test for RegisterForNotifications
func TestImpl_RegisterForNotifications(t *testing.T) {
	impl := getNewImpl()
	impl.Storage = testutil.MockStorage{}
	wd, _ := os.Getwd()
	crt, _ := utils.ReadFile(wd + "/../testutil/cmix.rip.crt")
	host, err := connect.NewHost("test", "0.0.0.0:420", crt, false, false)
	if err != nil {
		t.Errorf("Failed to create dummy host: %+v", err)
	}
	err = impl.RegisterForNotifications([]byte("token"), &connect.Auth{
		IsAuthenticated: true,
		Sender:          host,
	})
	if err != nil {
		t.Errorf("Failed to register for notifications: %+v", err)
	}
}

func TestImpl_UpdateNdf(t *testing.T) {
	impl := getNewImpl()
	emptyNdf := &pb.NDF{}

	impl.UpdateNdf(emptyNdf)

	if( impl.ndf != emptyNdf) {
		t.Logf("Failed to change ndf")
		t.Fail()
	}
}


// Unit test for UnregisterForNotifications
func TestImpl_UnregisterForNotifications(t *testing.T) {
	impl := getNewImpl()
	impl.Storage = testutil.MockStorage{}
	wd, _ := os.Getwd()
	crt, _ := utils.ReadFile(wd + "/../testutil/cmix.rip.crt")
	host, err := connect.NewHost("test", "0.0.0.0:420", crt, false, false)
	if err != nil {
		t.Errorf("Failed to create dummy host: %+v", err)
	}
	err = impl.UnregisterForNotifications(&connect.Auth{
		IsAuthenticated: true,
		Sender:          host,
	})
	if err != nil {
		t.Errorf("Failed to register for notifications: %+v", err)
	}
}

// func to get a quick new impl using test creds
func getNewImpl() *Impl {
	wd, _ := os.Getwd()
	params := Params{
		Address:       "0.0.0.0:4200",
		KeyPath:       wd + "/../testutil/cmix.rip.key",
		CertPath:      wd + "/../testutil/cmix.rip.crt",
		PublicAddress: "0.0.0.0:0",
	}
	instance, _ := StartNotifications(params, false)
	return instance
}


