package notifications

import (
	"gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/crypto/notifications"
	"gitlab.com/elixxir/crypto/registration"
	rsa2 "gitlab.com/elixxir/crypto/rsa"
	"gitlab.com/elixxir/notifications-bot/constants"
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

func TestImpl_RegisterToken(t *testing.T) {
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
	_, err = impl.Comms.AddHost(&id.Permissioning, "0.0.0.0", permCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Failed to add host: %+v", err)
	}
	permKey, err := utils.ReadFile(wd + "/../testutil/cmix.rip.key")
	if err != nil {
		t.Errorf("Failed to read test key file: %+v", err)
	}
	private, err := rsa2.GetScheme().Generate(csprng.NewSystemRNG(), 4096)
	if err != nil {
		t.Errorf("Failed to create private key: %+v", err)
	}
	public := private.Public()

	crt := public.MarshalPem()
	//uid := id.NewIdFromString("zezima", id.User, t)
	////iid, err := ephemeral.GetIntermediaryId(uid)
	////if err != nil {
	////	t.Errorf("Failed to get intermediary ID: %+v", err)
	////}
	loadedPermKey, err := rsa.LoadPrivateKeyFromPem(permKey)
	if err != nil {
		t.Errorf("Failed to load perm key from bytes: %+v", err)
	}
	ts := time.Now().UnixNano()
	psig, err := registration.SignWithTimestamp(csprng.NewSystemRNG(), loadedPermKey, ts, string(crt))

	token := "testtoken"
	reqTs := time.Now()
	sig, err := notifications.SignToken(private, token, constants.MessengerAndroid.String(), reqTs, notifications.RegisterTokenTag, csprng.NewSystemRNG())

	err = impl.RegisterToken(&mixmessages.RegisterTokenRequest{
		App:                         constants.MessengerAndroid.String(),
		Token:                       token,
		TransmissionRsaPem:          crt,
		RegistrationTimestamp:       ts,
		TransmissionRsaRegistrarSig: []byte("whoops"),
		RequestTimestamp:            reqTs.UnixNano(),
		TokenSignature:              sig,
	})
	if err == nil {
		t.Fatal("Expected error verifying perm sig")
	}

	err = impl.RegisterToken(&mixmessages.RegisterTokenRequest{
		App:                         constants.MessengerAndroid.String(),
		Token:                       token,
		TransmissionRsaPem:          crt,
		RegistrationTimestamp:       ts,
		TransmissionRsaRegistrarSig: psig,
		RequestTimestamp:            reqTs.UnixNano(),
		TokenSignature:              []byte("whoops"),
	})
	if err == nil {
		t.Fatal("Expected error verifying token sig")
	}

	err = impl.RegisterToken(&mixmessages.RegisterTokenRequest{
		App:                         constants.MessengerAndroid.String(),
		Token:                       token,
		TransmissionRsaPem:          crt,
		RegistrationTimestamp:       ts,
		TransmissionRsaRegistrarSig: psig,
		RequestTimestamp:            reqTs.UnixNano(),
		TokenSignature:              sig,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestImpl_RegisterTrackedID(t *testing.T) {
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
	_, err = impl.Comms.AddHost(&id.Permissioning, "0.0.0.0", permCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Failed to add host: %+v", err)
	}
	permKey, err := utils.ReadFile(wd + "/../testutil/cmix.rip.key")
	if err != nil {
		t.Errorf("Failed to read test key file: %+v", err)
	}
	private, err := rsa2.GetScheme().Generate(csprng.NewSystemRNG(), 4096)
	if err != nil {
		t.Errorf("Failed to create private key: %+v", err)
	}
	public := private.Public()

	crt := public.MarshalPem()
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to get intermediary ID: %+v", err)
	}
	loadedPermKey, err := rsa.LoadPrivateKeyFromPem(permKey)
	if err != nil {
		t.Errorf("Failed to load perm key from bytes: %+v", err)
	}
	ts := time.Now().UnixNano()
	psig, err := registration.SignWithTimestamp(csprng.NewSystemRNG(), loadedPermKey, ts, string(crt))

	token := "testtoken"
	reqTs := time.Now()
	tokenSig, err := notifications.SignToken(private, token, constants.MessengerAndroid.String(), reqTs, notifications.RegisterTokenTag, csprng.NewSystemRNG())

	err = impl.RegisterToken(&mixmessages.RegisterTokenRequest{
		App:                         constants.MessengerAndroid.String(),
		Token:                       token,
		TransmissionRsaPem:          crt,
		RegistrationTimestamp:       ts,
		TransmissionRsaRegistrarSig: psig,
		RequestTimestamp:            reqTs.UnixNano(),
		TokenSignature:              tokenSig,
	})
	if err != nil {
		t.Fatal(err)
	}

	iidSig, err := notifications.SignIdentity(private, [][]byte{iid}, reqTs, notifications.RegisterTrackedIDTag, csprng.NewSystemRNG())

	err = impl.RegisterTrackedID(&mixmessages.RegisterTrackedIdRequest{
		Request: &mixmessages.TrackedIntermediaryIdRequest{
			TrackedIntermediaryID: [][]byte{iid},
			TransmissionRsaPem:    crt,
			RequestTimestamp:      reqTs.UnixNano(),
			Signature:             nil,
		},
		RegistrationTimestamp:       reqTs.UnixNano(),
		TransmissionRsaRegistrarSig: psig,
	})
	if err == nil {
		t.Fatal("Expected error verifying tracked ID sig")
	}

	err = impl.RegisterTrackedID(&mixmessages.RegisterTrackedIdRequest{
		Request: &mixmessages.TrackedIntermediaryIdRequest{
			TrackedIntermediaryID: [][]byte{iid},
			TransmissionRsaPem:    crt,
			RequestTimestamp:      reqTs.UnixNano(),
			Signature:             iidSig,
		},
		RegistrationTimestamp:       reqTs.UnixNano(),
		TransmissionRsaRegistrarSig: psig,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestImpl_UnregisterToken(t *testing.T) {
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
	_, err = impl.Comms.AddHost(&id.Permissioning, "0.0.0.0", permCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Failed to add host: %+v", err)
	}
	permKey, err := utils.ReadFile(wd + "/../testutil/cmix.rip.key")
	if err != nil {
		t.Errorf("Failed to read test key file: %+v", err)
	}
	private, err := rsa2.GetScheme().Generate(csprng.NewSystemRNG(), 4096)
	if err != nil {
		t.Errorf("Failed to create private key: %+v", err)
	}
	public := private.Public()

	crt := public.MarshalPem()
	//uid := id.NewIdFromString("zezima", id.User, t)
	////iid, err := ephemeral.GetIntermediaryId(uid)
	////if err != nil {
	////	t.Errorf("Failed to get intermediary ID: %+v", err)
	////}
	loadedPermKey, err := rsa.LoadPrivateKeyFromPem(permKey)
	if err != nil {
		t.Errorf("Failed to load perm key from bytes: %+v", err)
	}
	ts := time.Now().UnixNano()
	psig, err := registration.SignWithTimestamp(csprng.NewSystemRNG(), loadedPermKey, ts, string(crt))

	token := "testtoken"
	reqTs := time.Now()
	sig, err := notifications.SignToken(private, token, constants.MessengerAndroid.String(), reqTs, notifications.RegisterTokenTag, csprng.NewSystemRNG())

	err = impl.RegisterToken(&mixmessages.RegisterTokenRequest{
		App:                         constants.MessengerAndroid.String(),
		Token:                       token,
		TransmissionRsaPem:          crt,
		RegistrationTimestamp:       ts,
		TransmissionRsaRegistrarSig: psig,
		RequestTimestamp:            reqTs.UnixNano(),
		TokenSignature:              sig,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = impl.UnregisterToken(&mixmessages.UnregisterTokenRequest{
		App:                constants.MessengerAndroid.String(),
		Token:              token,
		TransmissionRsaPem: crt,
		RequestTimestamp:   reqTs.UnixNano(),
		TokenSignature:     sig,
	})
	if err == nil {
		t.Fatal("Expected error verifying register signature")
	}

	unregSig, err := notifications.SignToken(private, token, constants.MessengerAndroid.String(), reqTs, notifications.UnregisterTokenTag, csprng.NewSystemRNG())
	err = impl.UnregisterToken(&mixmessages.UnregisterTokenRequest{
		App:                constants.MessengerAndroid.String(),
		Token:              token,
		TransmissionRsaPem: crt,
		RequestTimestamp:   reqTs.UnixNano(),
		TokenSignature:     unregSig,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestImpl_UnregisterTrackedID(t *testing.T) {
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
	_, err = impl.Comms.AddHost(&id.Permissioning, "0.0.0.0", permCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Failed to add host: %+v", err)
	}
	permKey, err := utils.ReadFile(wd + "/../testutil/cmix.rip.key")
	if err != nil {
		t.Errorf("Failed to read test key file: %+v", err)
	}
	private, err := rsa2.GetScheme().Generate(csprng.NewSystemRNG(), 4096)
	if err != nil {
		t.Errorf("Failed to create private key: %+v", err)
	}
	public := private.Public()

	crt := public.MarshalPem()
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to get intermediary ID: %+v", err)
	}
	loadedPermKey, err := rsa.LoadPrivateKeyFromPem(permKey)
	if err != nil {
		t.Errorf("Failed to load perm key from bytes: %+v", err)
	}
	ts := time.Now().UnixNano()
	psig, err := registration.SignWithTimestamp(csprng.NewSystemRNG(), loadedPermKey, ts, string(crt))

	token := "testtoken"
	reqTs := time.Now()
	tokenSig, err := notifications.SignToken(private, token, constants.MessengerAndroid.String(), reqTs, notifications.RegisterTokenTag, csprng.NewSystemRNG())

	err = impl.RegisterToken(&mixmessages.RegisterTokenRequest{
		App:                         constants.MessengerAndroid.String(),
		Token:                       token,
		TransmissionRsaPem:          crt,
		RegistrationTimestamp:       ts,
		TransmissionRsaRegistrarSig: psig,
		RequestTimestamp:            reqTs.UnixNano(),
		TokenSignature:              tokenSig,
	})
	if err != nil {
		t.Fatal(err)
	}

	iidSig, err := notifications.SignIdentity(private, [][]byte{iid}, reqTs, notifications.RegisterTrackedIDTag, csprng.NewSystemRNG())

	err = impl.RegisterTrackedID(&mixmessages.RegisterTrackedIdRequest{
		Request: &mixmessages.TrackedIntermediaryIdRequest{
			TrackedIntermediaryID: [][]byte{iid},
			TransmissionRsaPem:    crt,
			RequestTimestamp:      reqTs.UnixNano(),
			Signature:             nil,
		},
		RegistrationTimestamp:       reqTs.UnixNano(),
		TransmissionRsaRegistrarSig: psig,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = impl.UnregisterTrackedID(&mixmessages.TrackedIntermediaryIdRequest{
		TrackedIntermediaryID: [][]byte{iid},
		TransmissionRsaPem:    crt,
		RequestTimestamp:      reqTs.UnixNano(),
		Signature:             iidSig,
	})
	if err == nil {
		t.Fatal("Expected err attempting to unregister with same sig")
	}

	unregSig, err := notifications.SignIdentity(private, [][]byte{iid}, reqTs, notifications.UnregisterTrackedIDTag, csprng.NewSystemRNG())
	err = impl.UnregisterTrackedID(&mixmessages.TrackedIntermediaryIdRequest{
		TrackedIntermediaryID: [][]byte{iid},
		TransmissionRsaPem:    crt,
		RequestTimestamp:      reqTs.UnixNano(),
		Signature:             unregSig,
	})
	if err != nil {
		t.Fatal(err)
	}
}
