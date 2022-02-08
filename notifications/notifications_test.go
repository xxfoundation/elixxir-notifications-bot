////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////
package notifications

import (
	"fmt"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/crypto/registration"
	"gitlab.com/elixxir/notifications-bot/notifications/notificationProvider"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/elixxir/primitives/notifications"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/crypto/csprng"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gitlab.com/xx_network/primitives/utils"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

var port = 4200

type MockFB struct {
	c chan string
}

func (mfb *MockFB) Notify(csv string, target storage.GTNResult) (bool, error) {
	mfb.c <- csv
	return true, nil
}

func TestImpl_SendBatch(t *testing.T) {
	// Init storage
	s, err := storage.NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to make new storage: %+v", err)
	}

	dchan := make(chan string, 10)
	// Init mock firebase comms
	//badsend := func(firebase.FBSender, string, string) (string, error) {
	//	return "", errors.New("Failed")
	//}
	fc := &MockFB{dchan}

	// Create impl
	i := Impl{
		Storage:               s,
		notificationProviders: map[notifications.Provider]notificationProvider.Provider{},
		roundStore:            sync.Map{},
		maxNotifications:      0,
		maxPayloadBytes:       0,
	}
	i.notificationProviders[notifications.FCM] = fc

	// Identity setup
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to create iid: %+v", err)
	}
	if err != nil {
		t.Errorf("Could not parse precanned time: %v", err.Error())
	}
	u, err := s.AddUser(iid, []byte("rsacert"), []byte("sig"), "fcm:token", notifications.FCM)
	if err != nil {
		t.Errorf("Failed to add fake user: %+v", err)
	}
	_, e := ephemeral.HandleQuantization(time.Now())
	eph, err := s.AddLatestEphemeral(u, e, 16)
	if err != nil {
		t.Errorf("Failed to add latest ephemeral: %+v", err)
	}
	_, err = i.SendBatch(map[int64][]*notifications.Data{})
	if err != nil {
		t.Errorf("Error on sending empty batch: %+v", err)
	}

	unsent, err := i.SendBatch(map[int64][]*notifications.Data{
		eph.EphemeralId: {{EphemeralID: eph.EphemeralId, RoundID: 3, MessageHash: []byte("hello"), IdentityFP: []byte("identity")}},
	})
	if err != nil {
		t.Errorf("Error on sending small batch: %+v", err)
	}
	if len(unsent) < 1 {
		t.Errorf("Should have received notification back as unsent, instead got %+v", unsent)
	}

	i.maxPayloadBytes = 4096
	i.maxNotifications = 20
	unsent, err = i.SendBatch(map[int64][]*notifications.Data{
		1: {{EphemeralID: eph.EphemeralId, RoundID: 3, MessageHash: []byte("hello"), IdentityFP: []byte("identity")}},
	})
	if err != nil {
		t.Errorf("Error on sending small batch again: %+v", err)
	}
	if len(unsent) > 0 {
		t.Errorf("Should have received notification back as unsent, instead got %+v", unsent)
	}

	timeout := time.NewTicker(3 * time.Second)
	select {
	case <-dchan:
		t.Logf("Received on data chan!")
	case <-timeout.C:
		t.Errorf("Did not receive data before timeout")
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
		Address:               "0.0.0.0:42010",
		NotificationsPerBatch: 20,
		NotificationRate:      30,
		APNS: notificationProvider.APNSParams{
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
		Token:                 "fcm:token",
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

	u, err := impl.Storage.GetUser(iid)
	if err != nil {
		t.Errorf("Could not find registered user: %+v", err)
	}
	storedProvider := notifications.Provider(u.NotificationProvider)
	if storedProvider != notifications.FCM {
		t.Errorf("Unknown notification parsing did not work as expected.  Should have been %s, instead got %s", notifications.FCM.String(), storedProvider.String())
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
	s, err := storage.NewStorage("", "", "", "", "")
	impl := &Impl{
		Storage:          s,
		roundStore:       sync.Map{},
		maxNotifications: 0,
		maxPayloadBytes:  0,
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

	err = impl.ReceiveNotificationBatch(notifBatch, auth)
	if err != nil {
		t.Errorf("ReceiveNotificationBatch() returned an error: %+v", err)
	}

	nbm := impl.Storage.GetNotificationBuffer().Swap()
	if len(nbm[5]) < 1 {
		t.Errorf("Notification was not added to notification buffer: %+v", nbm[5])
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
