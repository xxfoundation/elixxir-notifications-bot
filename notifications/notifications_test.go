////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////
package notifications

import (
	"firebase.google.com/go/messaging"
	"fmt"
	"github.com/pkg/errors"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/notifications-bot/firebase"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/crypto/csprng"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gitlab.com/xx_network/primitives/ndf"
	"gitlab.com/xx_network/primitives/utils"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

var port = 4200

// Test notificationbot's notifyuser function
// this mocks the setup and send functions, and only tests the core logic of this function
func TestNotifyUser(t *testing.T) {
	badsend := func(firebase.FBSender, string, *pb.NotificationData) (string, error) {
		return "", errors.New("Failed")
	}
	send := func(firebase.FBSender, string, *pb.NotificationData) (string, error) {
		return "", nil
	}
	fcBadSend := firebase.NewMockFirebaseComm(t, badsend)
	fc := firebase.NewMockFirebaseComm(t, send)

	s, err := storage.NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to make new storage: %+v", err)
	}
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to create iid: %+v", err)
	}
	u, err := s.AddUser(iid, []byte("rsacert"), []byte("sig"), "token")
	if err != nil {
		t.Errorf("Failed to add fake user: %+v", err)
	}
	_, e := ephemeral.HandleQuantization(time.Now())
	eph, err := s.AddLatestEphemeral(u, e, 16)
	if err != nil {
		t.Errorf("Failed to add latest ephemeral: %+v", err)
	}

	err = notifyUser(&pb.NotificationData{
		EphemeralID: eph.EphemeralId,
		IdentityFP:  nil,
		MessageHash: nil,
	}, nil, fcBadSend, s)
	if err == nil {
		t.Errorf("Should have returned an error")
	}

	err = notifyUser(&pb.NotificationData{
		EphemeralID: eph.EphemeralId,
		IdentityFP:  nil,
		MessageHash: nil,
	}, nil, fc, s)
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
		Address: "0.0.0.0:42010",
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

// Dummy comms to unit test pollfornotifications
type mockPollComm struct{}

func (m mockPollComm) RequestNotifications(host *connect.Host) (*pb.UserIdList, error) {
	return &pb.UserIdList{
		IDs: [][]byte{[]byte("test")},
	}, nil
}
func (m mockPollComm) GetHost(hostId *id.ID) (*connect.Host, bool) {
	return &connect.Host{}, true
}
func (m mockPollComm) AddHost(hid *id.ID, address string, cert []byte, params connect.HostParams) (host *connect.Host, err error) {
	return nil, nil
}
func (m mockPollComm) RequestNdf(host *connect.Host, message *pb.NDFHash) (*pb.NDF, error) {
	return nil, nil
}

type mockPollErrComm struct{}

func (m mockPollErrComm) RequestNotifications(host *connect.Host) (*pb.UserIdList, error) {
	return nil, errors.New("failed to poll")
}
func (m mockPollErrComm) GetHost(hostId *id.ID) (*connect.Host, bool) {
	return nil, false
}
func (m mockPollErrComm) AddHost(id *id.ID, address string, cert []byte, params connect.HostParams) (host *connect.Host, err error) {
	return nil, nil
}
func (m mockPollErrComm) RequestNdf(host *connect.Host, message *pb.NDFHash) (*pb.NDF, error) {
	return nil, nil
}
func (m mockPollErrComm) RetrieveNdf(currentDef *ndf.NetworkDefinition) (*ndf.NetworkDefinition, error) {
	return nil, nil
}

// // Unit test for PollForNotifications
// func TestPollForNotifications(t *testing.T) {
// 	impl := &Impl{
// 		Comms: mockPollComm{},
// 		gwId:  id.NewIdFromString("test", id.Gateway, t),
// 	}
// 	errImpl := &Impl{
// 		Comms: mockPollErrComm{},
// 		gwId:  id.NewIdFromString("test", id.Gateway, t),
// 	}
// 	_, err := pollForNotifications(errImpl)
// 	if err == nil {
// 		t.Errorf("Failed to poll for notifications: %+v", err)
// 	}
//
// 	_, err = pollForNotifications(impl)
// 	if err != nil {
// 		t.Errorf("Failed to poll for notifications: %+v", err)
// 	}
// }

// Unit test for RegisterForNotifications
func TestImpl_RegisterForNotifications(t *testing.T) {
	impl := getNewImpl()
	var err error
	impl.Storage, err = storage.NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to create storage: %+v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get working dir: %+v", err)
	}
	crt, err := utils.ReadFile(wd + "/../testutil/cmix.rip.crt")
	if err != nil {
		t.Errorf("Failed to read test cert file: %+v", err)
	}
	key, err := utils.ReadFile(wd + "/../testutil/cmix.rip.key")
	if err != nil {
		t.Errorf("Failed to read test key file: %+v", err)
	}
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to make iid: %+v", err)
	}
	h, err := hash.NewCMixHash()
	if err != nil {
		t.Errorf("Failed to make cmix hash: %+v", err)
	}
	_, err = h.Write(iid)
	if err != nil {
		t.Errorf("Failed to write to hash: %+v", err)
	}
	pk, err := rsa.LoadPrivateKeyFromPem(key)
	if err != nil {
		t.Errorf("Failed to load pk from pem: %+v", err)
	}
	sig, err := rsa.Sign(csprng.NewSystemRNG(), pk, hash.CMixHash, h.Sum(nil), nil)
	if err != nil {
		t.Errorf("Failed to sign: %+v", err)
	}
	host, err := connect.NewHost(id.NewIdFromString("test", id.User, t), "0.0.0.0:420", crt, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Failed to create dummy host: %+v", err)
	}
	err = impl.RegisterForNotifications(&pb.NotificationRegisterRequest{
		Token:                 []byte("token"),
		IntermediaryId:        iid,
		TransmissionRsa:       []byte("trsa"),
		TransmissionSalt:      []byte("salt"),
		TransmissionRsaSig:    []byte("sig"),
		IIDTransmissionRsaSig: sig,
	}, &connect.Auth{
		IsAuthenticated: true,
		Sender:          host,
	})
	if err != nil {
		t.Errorf("Failed to register for notifications: %+v", err)
	}
}

// Unit test for UnregisterForNotifications
func TestImpl_UnregisterForNotifications(t *testing.T) {
	impl := getNewImpl()
	var err error
	impl.Storage, err = storage.NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to create storage: %+v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get working dir: %+v", err)
	}
	crt, err := utils.ReadFile(wd + "/../testutil/cmix.rip.crt")
	if err != nil {
		t.Errorf("Failed to read test cert file: %+v", err)
	}
	key, err := utils.ReadFile(wd + "/../testutil/cmix.rip.key")
	if err != nil {
		t.Errorf("Failed to reat test key file: %+v", err)
	}
	iid := []byte("zezima")
	h, err := hash.NewCMixHash()
	if err != nil {
		t.Errorf("Failed to make cmix hash: %+v", err)
	}
	_, err = h.Write(iid)
	if err != nil {
		t.Errorf("Failed to write to hash: %+v", err)
	}
	pk, err := rsa.LoadPrivateKeyFromPem(key)
	if err != nil {
		t.Errorf("Failed to load pk from pem: %+v", err)
	}
	sig, err := rsa.Sign(csprng.NewSystemRNG(), pk, hash.CMixHash, h.Sum(nil), nil)
	if err != nil {
		t.Errorf("Failed to sign: %+v", err)
	}

	host, err := connect.NewHost(id.NewIdFromString("test", id.User, t), "0.0.0.0:420", crt, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Failed to create dummy host: %+v", err)
	}
	err = impl.UnregisterForNotifications(&pb.NotificationUnregisterRequest{
		IntermediaryId:        iid,
		IIDTransmissionRsaSig: sig,
	}, &connect.Auth{
		IsAuthenticated: true,
		Sender:          host,
	})
	if err != nil {
		t.Errorf("Failed to register for notifications: %+v", err)
	}
}

// Happy path.
func TestImpl_ReceiveNotificationBatch(t *testing.T) {
	impl := getNewImpl()
	dataChan := make(chan *pb.NotificationData)
	impl.notifyFunc = func(data *pb.NotificationData, f *messaging.Client, fc *firebase.FirebaseComm, s *storage.Storage) error {
		go func() { dataChan <- data }()
		return nil
	}

	notifBatch := &pb.NotificationBatch{
		RoundID: 42,
		Notifications: []*pb.NotificationData{
			{
				EphemeralID: 5,
				IdentityFP:  []byte("IdentityFP"),
				MessageHash: []byte("MessageHash"),
			},
		},
	}

	auth := &connect.Auth{
		IsAuthenticated: true,
	}

	err := impl.ReceiveNotificationBatch(notifBatch, auth)
	if err != nil {
		t.Errorf("ReceiveNotificationBatch() returned an error: %+v", err)
	}

	select {
	case result := <-dataChan:
		if !reflect.DeepEqual(notifBatch.Notifications[0], result) {
			t.Errorf("Failed to receive expected NotificationData."+
				"\nexpected: %s\nreceived: %s", notifBatch.Notifications[0], result)
		}
	case <-time.NewTimer(50 * time.Millisecond).C:
		t.Error("Timed out while waiting for NotificationData.")
	}
}

// func to get a quick new impl using test creds
func getNewImpl() *Impl {
	wd, _ := os.Getwd()
	params := Params{
		Address:  fmt.Sprintf("0.0.0.0:%d", port),
		KeyPath:  wd + "/../testutil/cmix.rip.key",
		CertPath: wd + "/../testutil/cmix.rip.crt",
		FBCreds:  "",
	}
	port += 1
	instance, _ := StartNotifications(params, false, true)
	return instance
}
