package notifications

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/crypto/registration"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/crypto/csprng"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gitlab.com/xx_network/primitives/utils"
	"os"
	"testing"
	"time"
)

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
