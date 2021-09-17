////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////
package notifications

import (
	"firebase.google.com/go/messaging"
	"fmt"
	"github.com/jonahh-yeti/apns"
	"github.com/pkg/errors"
	"github.com/sideshow/apns2"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/crypto/registration"
	"gitlab.com/elixxir/notifications-bot/firebase"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/crypto/csprng"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gitlab.com/xx_network/primitives/utils"
	"os"
	"reflect"
	"testing"
	"time"
)

var port = 4200

type MockApns struct{}

func (m *MockApns) Send(token string, p apns.Payload, opts ...apns.SendOption) (*apns.Response, error) {
	return nil, nil
}

func (m *MockApns) Push(n *apns2.Notification) (*apns2.Response, error) {
	return nil, nil
}

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
	if err != nil {
		t.Errorf("Could not parse precanned time: %v", err.Error())
	}
	u, err := s.AddUser(iid, []byte("rsacert"), []byte("sig"), ":token")
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
	}, &MockApns{}, nil, fcBadSend, s, "")
	if err == nil {
		t.Errorf("Should have returned an error")
	}

	err = notifyUser(&pb.NotificationData{
		EphemeralID: eph.EphemeralId,
		IdentityFP:  nil,
		MessageHash: nil,
	}, &MockApns{}, nil, fc, s, "")
	if err != nil {
		t.Errorf("Failed to notify user properly")
	}
}

// Unit test for startnotifications
// tests logic including error cases
//func TestStartNotifications(t *testing.T) {
//	wd, err := os.Getwd()
//	if err != nil {
//		t.Errorf("Failed to get working dir: %+v", err)
//		return
//	}
//
//	params := Params{
//		Address: "0.0.0.0:42010",
//		APNS: APNSParams{
//			KeyPath:  wd + "/../testutil/apnsKey.key",
//			KeyID:    "WQT68265C5",
//			Issuer:   "S6JDM2WW29",
//			BundleID: "io.xxlabs.messenger",
//		},
//	}
//
//	params.KeyPath = wd + "/../testutil/cmix.rip.key"
//	_, err = StartNotifications(params, false, true)
//	if err == nil || !strings.Contains(err.Error(), "failed to read certificate at") {
//		t.Errorf("Should have thrown an error for no cert path")
//	}
//
//	params.CertPath = wd + "/../testutil/cmix.rip.crt"
//	_, err = StartNotifications(params, false, true)
//	if err != nil {
//		t.Errorf("Failed to start notifications successfully: %+v", err)
//	}
//}

// unit test for newimplementation
// tests logic and error cases
func TestNewImplementation(t *testing.T) {
	instance := getNewImpl()

	impl := NewImplementation(instance)
	if impl.Functions.RegisterForNotifications == nil || impl.Functions.UnregisterForNotifications == nil {
		t.Errorf("Functions were not properly set")
	}
}

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
	permCert, err := utils.ReadFile(wd + "/../testutil/cmix.rip.crt")
	if err != nil {
		t.Errorf("Failed to read test cert file: %+v", err)
	}
	permKey, err := utils.ReadFile(wd + "/../testutil/cmix.rip.key")
	if err != nil {
		t.Errorf("Failed to read test key file: %+v", err)
	}
	private, err := rsa.GenerateKey(csprng.NewSystemRNG(), 4096)
	if err != nil {
		t.Errorf("Failed to create private key: %+v", err)
	}
	public := private.GetPublic()
	key := rsa.CreatePrivateKeyPem(private)
	crt := rsa.CreatePublicKeyPem(public)
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
	_, err = impl.Comms.AddHost(&id.Permissioning, "0.0.0.0", permCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Failed to add host: %+v", err)
	}
	loadedPermKey, err := rsa.LoadPrivateKeyFromPem(permKey)
	if err != nil {
		t.Errorf("Failed to load perm key from bytes: %+v", err)
	}
	ts := time.Now().UnixNano()
	psig, err := registration.SignWithTimestamp(csprng.NewSystemRNG(), loadedPermKey, ts, string(crt))

	err = impl.RegisterForNotifications(&pb.NotificationRegisterRequest{
		Token:                 "token",
		IntermediaryId:        iid,
		TransmissionRsa:       crt,
		TransmissionSalt:      []byte("salt"),
		TransmissionRsaSig:    psig,
		IIDTransmissionRsaSig: sig,
		RegistrationTimestamp: ts,
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
	permCert, err := utils.ReadFile(wd + "/../testutil/cmix.rip.crt")
	if err != nil {
		t.Errorf("Failed to read test cert file: %+v", err)
	}
	permKey, err := utils.ReadFile(wd + "/../testutil/cmix.rip.key")
	if err != nil {
		t.Errorf("Failed to read test key file: %+v", err)
	}
	private, err := rsa.GenerateKey(csprng.NewSystemRNG(), 4096)
	if err != nil {
		t.Errorf("Failed to create private key: %+v", err)
	}
	public := private.GetPublic()
	key := rsa.CreatePrivateKeyPem(private)
	crt := rsa.CreatePublicKeyPem(public)
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to get intermediary ID: %+v", err)
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
	_, err = impl.Comms.AddHost(&id.Permissioning, "0.0.0.0", permCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Failed to add host: %+v", err)
	}
	h.Reset()
	_, err = h.Write(crt)
	if err != nil {
		t.Errorf("Failed to write to hash: %+v", err)
	}
	loadedPermKey, err := rsa.LoadPrivateKeyFromPem(permKey)
	if err != nil {
		t.Errorf("Failed to load perm key from bytes: %+v", err)
	}
	ts := time.Now().UnixNano()
	psig, err := registration.SignWithTimestamp(csprng.NewSystemRNG(), loadedPermKey, ts, string(crt))

	err = impl.RegisterForNotifications(&pb.NotificationRegisterRequest{
		Token:                 "token",
		IntermediaryId:        iid,
		TransmissionRsa:       crt,
		TransmissionSalt:      []byte("salt"),
		TransmissionRsaSig:    psig,
		IIDTransmissionRsaSig: sig,
		RegistrationTimestamp: ts,
	})
	if err != nil {
		t.Errorf("Failed to register for notifications: %+v", err)
	}
	err = impl.UnregisterForNotifications(&pb.NotificationUnregisterRequest{
		IntermediaryId:        iid,
		IIDTransmissionRsaSig: sig,
	})
	if err != nil {
		t.Errorf("Failed to unregister for notifications: %+v", err)
	}
}

// Happy path.
func TestImpl_ReceiveNotificationBatch(t *testing.T) {
	impl := getNewImpl()
	dataChan := make(chan *pb.NotificationData)
	impl.notifyFunc = func(data *pb.NotificationData, apns ApnsSender, f *messaging.Client, fc *firebase.FirebaseComm, s *storage.Storage, topic string) error {
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
