// //////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//
//	//
//
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
// //////////////////////////////////////////////////////////////////////////////
package storage

import (
	"gitlab.com/elixxir/notifications-bot/constants"
	"gitlab.com/xx_network/crypto/csprng"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"testing"
	"time"
)

func TestStorage_RegisterToken(t *testing.T) {
	s, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create new storage object: %+v", err)
	}

	token := "TestToken"
	app := "HavenIOS"
	trsaPrivate, err := rsa.GenerateKey(csprng.NewSystemRNG(), 512)
	if err != nil {
		t.Fatal(err)
	}
	pub := rsa.CreatePublicKeyPem(trsaPrivate.GetPublic())

	err = s.RegisterToken(token, app, pub)
	if err != nil {
		t.Fatalf("Failed to register token: %+v", err)
	}

	err = s.RegisterToken(token, app, pub)
	if err != nil {
		t.Fatalf("Duplicate register token returned unexpected error: %+v", err)
	}
}

func TestStorage_RegisterTrackedID(t *testing.T) {
	s, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create new storage object: %+v", err)
	}

	token := "TestToken"
	app := "HavenIOS"
	trsaPrivate, err := rsa.GenerateKey(csprng.NewSystemRNG(), 512)
	if err != nil {
		t.Fatal(err)
	}
	pub := rsa.CreatePublicKeyPem(trsaPrivate.GetPublic())
	testId, err := id.NewRandomID(csprng.NewSystemRNG(), id.User)
	if err != nil {
		t.Fatalf("Failed to generate test ID: %+v", err)
	}
	iid, err := ephemeral.GetIntermediaryId(testId)
	if err != nil {
		t.Fatalf("Failed to generate intermediary ID: %+v", err)
	}
	_, epoch := ephemeral.HandleQuantization(time.Now())

	err = s.RegisterToken(token, app, pub)
	if err != nil {
		t.Fatalf("Failed to register token: %+v", err)
	}

	err = s.RegisterTrackedID([][]byte{iid}, pub, epoch, 16)
	if err != nil {
		t.Fatalf("Received error registering identity: %+v", err)
	}

	err = s.RegisterTrackedID([][]byte{iid}, pub, epoch, 16)
	if err != nil {
		t.Fatalf("Received unexpected error on duplicate identity registration: %+v", err)
	}
}

func TestStorage_UnregisterToken(t *testing.T) {
	s, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create new storage object: %+v", err)
	}

	token := "TestToken"
	otherToken := "TestToken2"
	app := "HavenIOS"
	trsaPrivate, err := rsa.GenerateKey(csprng.NewSystemRNG(), 512)
	if err != nil {
		t.Fatal(err)
	}
	pub := rsa.CreatePublicKeyPem(trsaPrivate.GetPublic())

	err = s.UnregisterToken(token, pub)
	if err != nil {
		t.Fatalf("Received error on unregister with nothing inserted: %+v", err)
	}

	err = s.RegisterToken(token, app, pub)
	if err != nil {
		t.Fatalf("Failed to register token: %+v", err)
	}

	err = s.UnregisterToken(otherToken, pub)
	if err != nil {
		t.Fatalf("Received error on unregister when token not inserted: %+v", err)
	}

	err = s.RegisterToken(otherToken, app, pub)
	if err != nil {
		t.Fatalf("Failed to register second token: %+v", err)
	}

	trsaHash, err := getHash(pub)
	if err != nil {
		t.Fatalf("Failed to get trsa hash: %+v", err)
	}
	u, err := s.GetUser(trsaHash)
	if err != nil {
		t.Fatalf("Failed to get user: %+v", err)
	}

	if len(u.Tokens) != 2 {
		t.Fatalf("Did not receive expected tokens on user")
	}

	err = s.UnregisterToken(token, pub)
	if err != nil {
		t.Fatalf("Received error on unregister: %+v", err)
	}

	u, err = s.GetUser(trsaHash)
	if err != nil {
		t.Fatalf("Failed to get user after token deletion: %+v", err)
	}

	if len(u.Tokens) != 1 {
		t.Fatalf("Tokens on user should have been reduced to 1")
	}

	err = s.UnregisterToken(otherToken, pub)
	if err != nil {
		t.Fatalf("Received error on second token unregister: %+v", err)
	}

	u, err = s.GetUser(trsaHash)
	if err != nil {
		t.Fatalf("User should still exist after unregister, instead got: %+v", err)
	}

}

func TestStorage_UnregisterTrackedID(t *testing.T) {
	s, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create new storage object: %+v", err)
	}

	token := "TestToken"
	app := "HavenIOS"
	trsaPrivate, err := rsa.GenerateKey(csprng.NewSystemRNG(), 512)
	if err != nil {
		t.Fatal(err)
	}
	pub := rsa.CreatePublicKeyPem(trsaPrivate.GetPublic())
	testId, err := id.NewRandomID(csprng.NewSystemRNG(), id.User)
	if err != nil {
		t.Fatalf("Failed to generate test ID: %+v", err)
	}
	iid, err := ephemeral.GetIntermediaryId(testId)
	if err != nil {
		t.Fatalf("Failed to generate IID: %+v", err)
	}
	testId2, err := id.NewRandomID(csprng.NewSystemRNG(), id.User)
	if err != nil {
		t.Fatalf("Failed to generate test ID: %+v", err)
	}
	iid2, err := ephemeral.GetIntermediaryId(testId2)
	if err != nil {
		t.Fatalf("Failed to generate IID: %+v", err)
	}
	_, epoch := ephemeral.HandleQuantization(time.Now())

	err = s.UnregisterTrackedIDs([][]byte{iid}, pub)
	if err != nil {
		t.Fatalf("Error on unregister tracked ID with nothing inserted: %+v", err)
	}

	err = s.RegisterToken(token, app, pub)
	if err != nil {
		t.Fatalf("Failed to register token: %+v", err)
	}

	err = s.UnregisterTrackedIDs([][]byte{iid}, pub)
	if err != nil {
		t.Fatalf("Error on unregister tracked ID with user inserted, but no tracked IDs: %+v", err)
	}

	err = s.RegisterToken(token, app, pub)
	if err != nil {
		t.Fatalf("Failed to register token: %+v", err)
	}

	err = s.RegisterTrackedID([][]byte{iid}, pub, epoch, 16)
	if err != nil {
		t.Fatalf("Received error registering identity: %+v", err)
	}

	err = s.UnregisterTrackedIDs([][]byte{iid2}, pub)
	if err != nil {
		t.Fatalf("Error on unregister untracked ID: %+v", err)
	}

	err = s.RegisterTrackedID([][]byte{iid2}, pub, epoch, 16)
	if err != nil {
		t.Fatalf("Received error registering identity: %+v", err)
	}

	trsaHash, err := getHash(pub)
	if err != nil {
		t.Fatalf("Failed to get trsa hash: %+v", err)
	}
	u, err := s.GetUser(trsaHash)
	if err != nil {
		t.Fatalf("Failed to get user: %+v", err)
	}

	if len(u.Identities) != 2 {
		t.Fatalf("Did not receive expected identities for user")
	}

	err = s.UnregisterTrackedIDs([][]byte{iid}, pub)
	if err != nil {
		t.Fatalf("Failed to unregister tracked ID: %+v", err)
	}

	u, err = s.GetUser(trsaHash)
	if err != nil {
		t.Fatalf("Failed to get user after first delete: %+v", err)
	}

	if len(u.Identities) != 1 {
		t.Fatalf("Identity was not properly deleted")
	}

	err = s.UnregisterTrackedIDs([][]byte{iid2}, pub)
	if err != nil {
		t.Fatalf("Failed to unregister tracked ID: %+v", err)
	}

	u, err = s.GetUser(trsaHash)
	if err != nil {
		t.Fatalf("User should still exist after unregister, instead got: %+v", err)
	}
	if len(u.Tokens) != 1 {
		t.Fatalf("User tokens should be unaffected by unregistering ID")
	}
}

func TestStorage_RegisterForNotifications(t *testing.T) {
	s, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to create new storage object: %+v", err)
	}
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to create iid: %+v", err)
	}
	if err != nil {
		t.Errorf("Could not parse precanned time: %v", err.Error())
	}
	_, err = s.RegisterForNotifications(iid, []byte("transmissionrsa"), "token", constants.MessengerIOS.String(), 0, 8)
	if err != nil {
		t.Errorf("Failed to add user: %+v", err)
	}
}

func TestStorage_AddLatestEphemeral(t *testing.T) {
	s, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to create new storage object: %+v", err)
	}
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to create iid: %+v", err)
	}
	if err != nil {
		t.Errorf("Could not parse precanned time: %v", err.Error())
	}
	ident := &Identity{
		IntermediaryId: iid,
		OffsetNum:      ephemeral.GetOffsetNum(ephemeral.GetOffset(iid)),
	}
	err = s.insertIdentity(ident)
	if err != nil {
		t.Errorf("Failed to add user: %+v", err)
	}
	_, err = s.AddLatestEphemeral(ident, 5, 16)
	if err != nil {
		t.Errorf("Failed to add latest ephemeral: %+v", err)
	}
}

func TestStorage_AddEphemeralsForOffset(t *testing.T) {
	_, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to create new storage object: %+v", err)
	}
}
