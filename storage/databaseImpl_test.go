package storage

import (
	"bytes"
	"errors"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/xx_network/crypto/csprng"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gorm.io/gorm"
	"testing"
)

func TestDatabaseImpl_UpsertState(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_UpsertState", "", "")
	if err != nil {
		t.Fatal(err)
	}
	expectedState := &State{
		Key:   "state_key",
		Value: "state_val",
	}
	err = db.UpsertState(expectedState)
	if err != nil {
		t.Fatal(err)
	}

	retrievedState, err := db.GetStateValue(expectedState.Key)
	if err != nil {
		t.Fatal(err)
	}

	if retrievedState != expectedState.Value {
		t.Fatalf("Did not get expected state value\n\tExpected: %s\n\tReceived: %s\n", expectedState.Value, retrievedState)
	}

	expectedState2 := &State{
		Key:   expectedState.Key,
		Value: "state_value_two",
	}
	err = db.UpsertState(expectedState2)
	if err != nil {
		t.Fatal(err)
	}

	retrievedState, err = db.GetStateValue(expectedState.Key)
	if err != nil {
		t.Fatal(err)
	}

	if retrievedState != expectedState2.Value {
		t.Fatalf("State value did not change after upsert\n\tExpected: %s\n\tReceived: %s\n", expectedState2.Value, retrievedState)
	}
}

func TestDatabaseImpl_GetStateValue(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_GetStateValue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	expectedState := &State{
		Key:   "state_key",
		Value: "state_val",
	}

	retrievedState, err := db.GetStateValue(expectedState.Key)
	if err == nil {
		t.Fatalf("Should have received error when state not inserted, instead got %s", retrievedState)
	}
	if retrievedState != "" {
		t.Fatal("Should not have received data for state not yet inserted")
	}

	err = db.UpsertState(expectedState)
	if err != nil {
		t.Fatal(err)
	}

	retrievedState, err = db.GetStateValue(expectedState.Key)
	if err != nil {
		t.Fatal(err)
	}

	if retrievedState != expectedState.Value {
		t.Fatalf("Did not receive expected value\n\tExpected: %s\n\tReceived: %s\n", expectedState.Value, retrievedState)
	}
}

func TestDatabaseImpl_DeleteToken(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_DeleteToken", "", "")
	if err != nil {
		t.Fatal(err)
	}

	identity := generateTestIdentity(t)
	u := generateTestUser(t)

	err = db.insertUser(u)
	if err != nil {
		t.Fatal(err)
	}

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	token := "apnstoken01"
	err = db.registerForNotifications(u, identity, token)
	if err != nil {
		t.Fatal(err)
	}

	receivedUser, err := db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}

	if len(receivedUser.Tokens) != 1 {
		t.Fatalf("User should have %d tokens registered, instead had %d", 1, len(receivedUser.Tokens))
	}

	err = db.DeleteToken(token)
	if err != nil {
		t.Fatal(err)
	}

	receivedUser, err = db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}

	if len(receivedUser.Tokens) != 0 {
		t.Fatalf("User should have %d tokens registered, instead had %d", 1, len(receivedUser.Tokens))
	}
}

func TestDatabaseImpl_insertUser(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_insertUser", "", "")
	if err != nil {
		t.Fatal(err)
	}

	u := generateTestUser(t)

	err = db.insertUser(u)
	if err != nil {
		t.Fatal(err)
	}

	receivedUser, err := db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(u.Signature, receivedUser.Signature) || !bytes.Equal(u.TransmissionRSA, receivedUser.TransmissionRSA) || !bytes.Equal(u.TransmissionRSAHash, receivedUser.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user data\n\tExpected: %+v\n\tReceived: %+v\n", u, receivedUser)
	}

	err = db.insertUser(u)
	if err != nil {
		t.Fatal(err)
	}

	receivedUser, err = db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(u.Signature, receivedUser.Signature) || !bytes.Equal(u.TransmissionRSA, receivedUser.TransmissionRSA) || !bytes.Equal(u.TransmissionRSAHash, receivedUser.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user data\n\tExpected: %+v\n\tReceived: %+v\n", u, receivedUser)
	}
}

func TestDatabaseImpl_GetUser(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_GetUser", "", "")
	if err != nil {
		t.Fatal(err)
	}

	u := generateTestUser(t)

	receivedUser, err := db.GetUser(u.TransmissionRSAHash)
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("Expected gorm.ErrRecordNotFound when no user exists, instead got %+v", err)
	}

	err = db.insertUser(u)
	if err != nil {
		t.Fatal(err)
	}

	receivedUser, err = db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(u.Signature, receivedUser.Signature) || !bytes.Equal(u.TransmissionRSA, receivedUser.TransmissionRSA) || !bytes.Equal(u.TransmissionRSAHash, receivedUser.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user data\n\tExpected: %+v\n\tReceived: %+v\n", u, receivedUser)
	}
}

func TestDatabaseImpl_deleteUser(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_deleteUser", "", "")
	if err != nil {
		t.Fatal(err)
	}

	u := generateTestUser(t)

	receivedUser, err := db.GetUser(u.TransmissionRSAHash)
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("Expected gorm.ErrRecordNotFound when no user exists, instead got %+v", err)
	}

	err = db.insertUser(u)
	if err != nil {
		t.Fatal(err)
	}

	receivedUser, err = db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(u.Signature, receivedUser.Signature) || !bytes.Equal(u.TransmissionRSA, receivedUser.TransmissionRSA) || !bytes.Equal(u.TransmissionRSAHash, receivedUser.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user data\n\tExpected: %+v\n\tReceived: %+v\n", u, receivedUser)
	}

	err = db.deleteUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}

	receivedUser, err = db.GetUser(u.TransmissionRSAHash)
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("Expected gorm.ErrRecordNotFound when no user exists, instead got %+v", err)
	}
}

func TestDatabaseImpl_GetAllUsers(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_GetAllUsers", "", "")
	if err != nil {
		t.Fatal(err)
	}

	startUsers, err := db.GetAllUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(startUsers) != 0 {
		t.Fatalf("Did not receive expected user count\n\tExpected: %d\n\tReceived: %d\n", 0, len(startUsers))
	}

	expectedUsers := 5
	for i := 1; i <= expectedUsers; i++ {
		u := generateTestUser(t)
		err = db.insertUser(u)
		if err != nil {
			t.Fatal(err)
		}

		receivedUsers, err := db.GetAllUsers()
		if err != nil {
			t.Fatal(err)
		}
		if len(receivedUsers) != i {
			t.Fatalf("Did not receive expected user count\n\tExpected: %d\n\tReceived: %d\n", i, len(receivedUsers))
		}
	}

}

func TestDatabaseImpl_getIdentity(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_getIdentity", "", "")
	if err != nil {
		t.Fatal(err)
	}

	identity := generateTestIdentity(t)

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	receivedIdentity, err := db.getIdentity(identity.IntermediaryId)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(receivedIdentity.IntermediaryId, identity.IntermediaryId) || receivedIdentity.OffsetNum != identity.OffsetNum {
		t.Fatalf("Did not receive expected identity data\n\tExpected: %+v\n\tReceived: %+v\n", identity, receivedIdentity)
	}
}

func TestDatabaseImpl_getIdentitiesByOffset(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_getIdentitiesByOffset", "", "")
	if err != nil {
		t.Fatal(err)
	}

	identity := generateTestIdentity(t)

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	offsetIdentities, err := db.getIdentitiesByOffset(identity.OffsetNum)
	if err != nil {
		t.Fatal(err)
	}
	if len(offsetIdentities) != 1 {
		t.Fatalf("Did not receive expected offset identities")
	}

	offsetIdentities, err = db.getIdentitiesByOffset(identity.OffsetNum + 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(offsetIdentities) != 0 {
		t.Fatalf("Did not receive expected offset identities")
	}

}

func TestDatabaseImpl_GetOrphanedIdentities(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_GetOrphanedIdentities", "", "")
	if err != nil {
		t.Fatal(err)
	}

	identity := generateTestIdentity(t)

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	orphaned, err := db.GetOrphanedIdentities()
	if err != nil {
		t.Fatal(err)
	}
	if len(orphaned) != 1 {
		t.Fatalf("Did not receive expected count of orphaned identities\n\tExpected: %+v\n\tReceived: %+v\n", 1, len(orphaned))
	}

	identity2 := generateTestIdentity(t)

	err = db.insertIdentity(&identity2)
	if err != nil {
		t.Fatal(err)
	}

	orphaned, err = db.GetOrphanedIdentities()
	if err != nil {
		t.Fatal(err)
	}
	if len(orphaned) != 2 {
		t.Fatalf("Did not receive expected count of orphaned identities\n\tExpected: %+v\n\tReceived: %+v\n", 2, len(orphaned))
	}

	err = db.insertEphemeral(&Ephemeral{
		ID:             0,
		IntermediaryId: identity.IntermediaryId,
		Offset:         ephemeral.GetOffset(identity.IntermediaryId),
		EphemeralId:    123,
		Epoch:          123,
	})
	if err != nil {
		t.Fatal(err)
	}

	orphaned, err = db.GetOrphanedIdentities()
	if err != nil {
		t.Fatal(err)
	}
	if len(orphaned) != 1 {
		t.Fatalf("Did not receive expected count of orphaned identities\n\tExpected: %+v\n\tReceived: %+v\n", 1, len(orphaned))
	}
}

func TestDatabaseImpl_insertEphemeral(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_insertEphemeral", "", "")
	if err != nil {
		t.Fatal(err)
	}

	identity := generateTestIdentity(t)

	e1 := &Ephemeral{
		ID:             0,
		IntermediaryId: identity.IntermediaryId,
		Offset:         ephemeral.GetOffset(identity.IntermediaryId),
		EphemeralId:    123,
		Epoch:          123,
	}

	err = db.insertEphemeral(e1)
	if err == nil {
		t.Fatal("Should fail to insert ephemeral with no associated identity")
	}

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	err = db.insertEphemeral(e1)
	if err != nil {
		t.Fatal(err)
	}

}

func TestDatabaseImpl_GetEphemeral(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_GetEphemeral", "", "")
	if err != nil {
		t.Fatal(err)
	}

	identity := generateTestIdentity(t)

	e1 := &Ephemeral{
		ID:             0,
		IntermediaryId: identity.IntermediaryId,
		Offset:         ephemeral.GetOffset(identity.IntermediaryId),
		EphemeralId:    123,
		Epoch:          123,
	}

	ephs, err := db.GetEphemeral(e1.EphemeralId)
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal(err)
	}
	if len(ephs) != 0 {
		t.Fatalf("Did not receive expected ephemerals\n\tExpected: %+v\n\tReceived: %+v\n", ephs[0], e1)
	}

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	err = db.insertEphemeral(e1)
	if err != nil {
		t.Fatal(err)
	}

	ephs, err = db.GetEphemeral(e1.EphemeralId)
	if err != nil {
		t.Fatal(err)
	}
	if len(ephs) != 1 || ephs[0].EphemeralId != e1.EphemeralId {
		t.Fatalf("Did not receive expected ephemerals\n\tExpected: %+v\n\tReceived: %+v\n", ephs[0], e1)
	}
}

func TestDatabaseImpl_DeleteOldEphemerals(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_DeleteOldEphemerals", "", "")
	if err != nil {
		t.Fatal(err)
	}

	identity := generateTestIdentity(t)

	e1 := &Ephemeral{
		ID:             0,
		IntermediaryId: identity.IntermediaryId,
		Offset:         ephemeral.GetOffset(identity.IntermediaryId),
		EphemeralId:    123,
		Epoch:          123,
	}

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	err = db.insertEphemeral(e1)
	if err != nil {
		t.Fatal(err)
	}

	ephs, err := db.GetEphemeral(e1.EphemeralId)
	if err != nil {
		t.Fatal(err)
	}
	if len(ephs) != 1 || ephs[0].EphemeralId != e1.EphemeralId {
		t.Fatalf("Did not receive expected ephemerals\n\tExpected: %+v\n\tReceived: %+v\n", ephs[0], e1)
	}

	err = db.DeleteOldEphemerals(e1.Epoch)
	if err != nil {
		t.Fatal(err)
	}

	ephs, err = db.GetEphemeral(e1.EphemeralId)
	if err != nil {
		t.Fatal(err)
	}
	if len(ephs) != 1 || ephs[0].EphemeralId != e1.EphemeralId {
		t.Fatalf("Did not receive expected ephemerals\n\tExpected: %+v\n\tReceived: %+v\n", ephs[0], e1)
	}

	err = db.DeleteOldEphemerals(e1.Epoch + 1)
	if err != nil {
		t.Fatal(err)
	}

	ephs, err = db.GetEphemeral(e1.EphemeralId)
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("Did not receive expected gorm.ErrRecordNotFound")
	}
	if len(ephs) != 0 {
		t.Fatalf("Did not receive expected ephemerals\n\tExpected: %+v\n\tReceived: %+v\n", ephs[0], e1)
	}
}

func TestDatabaseImpl_GetLatestEphemeral(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_GetLatestEphemeral", "", "")
	if err != nil {
		t.Fatal(err)
	}

	identity := generateTestIdentity(t)

	e1 := &Ephemeral{
		IntermediaryId: identity.IntermediaryId,
		Offset:         ephemeral.GetOffset(identity.IntermediaryId),
		EphemeralId:    123,
		Epoch:          123,
	}

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	err = db.insertEphemeral(e1)
	if err != nil {
		t.Fatal(err)
	}

	latest, err := db.GetLatestEphemeral()
	if err != nil {
		t.Fatal(err)
	}
	if latest.EphemeralId != e1.EphemeralId {
		t.Fatalf("Did not receive expected ephemeral\n\tExpected: %d\n\tReceived: %d\n", e1.ID, latest.ID)
	}

	e2 := &Ephemeral{
		IntermediaryId: identity.IntermediaryId,
		Offset:         ephemeral.GetOffset(identity.IntermediaryId),
		EphemeralId:    124,
		Epoch:          123,
	}

	err = db.insertEphemeral(e2)
	if err != nil {
		t.Fatal(err)
	}

	latest, err = db.GetLatestEphemeral()
	if err != nil {
		t.Fatal(err)
	}
	if latest.EphemeralId != e2.EphemeralId {
		t.Fatalf("Did not receive expected ephemeral\n\tExpected: %d\n\tReceived: %d\n", e2.ID, latest.ID)
	}

	e3 := &Ephemeral{
		IntermediaryId: identity.IntermediaryId,
		Offset:         ephemeral.GetOffset(identity.IntermediaryId),
		EphemeralId:    124,
		Epoch:          122,
	}

	err = db.insertEphemeral(e3)
	if err != nil {
		t.Fatal(err)
	}

	latest, err = db.GetLatestEphemeral()
	if err != nil {
		t.Fatal(err)
	}
	if latest.EphemeralId != e2.EphemeralId {
		t.Fatalf("Did not receive expected ephemeral\n\tExpected: %d\n\tReceived: %d\n", e2.ID, latest.ID)
	}

	e4 := &Ephemeral{
		IntermediaryId: identity.IntermediaryId,
		Offset:         ephemeral.GetOffset(identity.IntermediaryId),
		EphemeralId:    126,
		Epoch:          125,
	}

	err = db.insertEphemeral(e4)
	if err != nil {
		t.Fatal(err)
	}

	latest, err = db.GetLatestEphemeral()
	if err != nil {
		t.Fatal(err)
	}
	if latest.EphemeralId != e4.EphemeralId {
		t.Fatalf("Did not receive expected ephemeral\n\tExpected: %d\n\tReceived: %d\n", e4.ID, latest.ID)
	}

	e5 := &Ephemeral{
		IntermediaryId: identity.IntermediaryId,
		Offset:         ephemeral.GetOffset(identity.IntermediaryId),
		EphemeralId:    127,
		Epoch:          121,
	}

	err = db.insertEphemeral(e5)
	if err != nil {
		t.Fatal(err)
	}

	latest, err = db.GetLatestEphemeral()
	if err != nil {
		t.Fatal(err)
	}
	if latest.EphemeralId != e4.EphemeralId {
		t.Fatalf("Did not receive expected ephemeral\n\tExpected: %d\n\tReceived: %d\n", e4.ID, latest.ID)
	}
}

func TestDatabaseImpl_registerForNotifications(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_registerForNotifications", "", "")
	if err != nil {
		t.Fatal(err)
	}

	identity := generateTestIdentity(t)
	u := generateTestUser(t)

	err = db.insertUser(u)
	if err != nil {
		t.Fatal(err)
	}

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	ru, err := db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ru.Tokens) != 0 || len(ru.Identities) != 0 || !bytes.Equal(ru.TransmissionRSAHash, u.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user\n\tExpected: %+v\n\t: Receiveid: %+v\n", u, ru)
	}

	token := "apnstoken02"
	err = db.registerForNotifications(u, identity, token)
	if err != nil {
		t.Fatal(err)
	}

	identity2 := generateTestIdentity(t)
	u2 := generateTestUser(t)

	err = db.insertUser(u2)
	if err != nil {
		t.Fatal(err)
	}

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	token2 := "fcm:token2"
	err = db.registerForNotifications(u2, identity2, token2)
	if err != nil {
		t.Fatal(err)
	}

	ru, err = db.GetUser(u2.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ru.Tokens) != 1 || len(ru.Identities) != 1 || !bytes.Equal(ru.TransmissionRSAHash, u2.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user\n\tExpected: %+v\n\t: Receiveid: %+v\n", u2, ru)
	}

	err = db.registerForNotifications(u, identity2, token)
	if err != nil {
		t.Fatal(err)
	}
	ru, err = db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ru.Tokens) != 1 || len(ru.Identities) != 2 || !bytes.Equal(ru.TransmissionRSAHash, u.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user\n\tExpected: %+v\n\t: Receiveid: %+v\n", u2, ru)
	}

	err = db.registerForNotifications(u, identity2, token2)
	if err != nil {
		t.Fatal(err)
	}
	ru, err = db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ru.Tokens) != 2 || len(ru.Identities) != 2 || !bytes.Equal(ru.TransmissionRSAHash, u.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user\n\tExpected: %+v\n\t: Receiveid: %+v\n", u, ru)
	}

	err = db.registerForNotifications(u2, identity, token)
	if err != nil {
		t.Fatal(err)
	}
	ru, err = db.GetUser(u2.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ru.Tokens) != 2 || len(ru.Identities) != 2 || !bytes.Equal(ru.TransmissionRSAHash, u2.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user\n\tExpected: %+v\n\t: Receiveid: %+v\n", u, ru)
	}
}

func TestDatabaseImpl_unregisterIdentities(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_unregisterIdentities", "", "")
	if err != nil {
		t.Fatal(err)
	}

	u := generateTestUser(t)
	identity := generateTestIdentity(t)

	err = db.insertUser(u)
	if err != nil {
		t.Fatal(err)
	}

	err = db.unregisterIdentities(u, []Identity{identity})
	if err != nil {
		t.Fatalf("Should not return error even if identity doesn't exist: %+v", err)
	}

	err = db.insertUser(u)
	if err != nil {
		t.Fatal(err)
	}

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	token := "apnstoken02"
	err = db.registerForNotifications(u, identity, token)
	if err != nil {
		t.Fatal(err)
	}

	identity2 := generateTestIdentity(t)

	token2 := "fcm:token2"
	err = db.registerForNotifications(u, identity2, token2)
	if err != nil {
		t.Fatal(err)
	}

	ru, err := db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ru.Tokens) != 2 || len(ru.Identities) != 2 || !bytes.Equal(ru.TransmissionRSAHash, u.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user\n\tExpected: %+v\n\t: Receiveid: %+v\n", u, ru)
	}

	err = db.unregisterIdentities(u, []Identity{identity})
	if err != nil {
		t.Fatalf("Failed to unregister identity: %+v", err)
	}

	ru, err = db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ru.Tokens) != 2 || len(ru.Identities) != 1 || !bytes.Equal(ru.TransmissionRSAHash, u.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user\n\tExpected: %+v\n\t: Receiveid: %+v\n", u, ru)
	}
}

func TestDatabaseImpl_unregisterTokens(t *testing.T) {
	db, err := newDatabase("", "", "TestDatabaseImpl_unregisterTokens", "", "")
	if err != nil {
		t.Fatal(err)
	}

	identity := generateTestIdentity(t)
	u := generateTestUser(t)

	err = db.insertUser(u)
	if err != nil {
		t.Fatal(err)
	}

	token := "apnstoken02"
	err = db.unregisterTokens(u, []Token{Token{Token: token}})
	if err != nil {
		t.Fatalf("Should not return error even if identity doesn't exist: %+v", err)
	}

	_, err = db.GetUser(u.TransmissionRSAHash)
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("Expected gorm.ErrRecordNotFound when no user exists, instead got %+v", err)
	}

	err = db.insertUser(u)
	if err != nil {
		t.Fatal(err)
	}

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	err = db.registerForNotifications(u, identity, token)
	if err != nil {
		t.Fatal(err)
	}

	identity2 := generateTestIdentity(t)

	err = db.insertIdentity(&identity)
	if err != nil {
		t.Fatal(err)
	}

	token2 := "fcm:token2"
	err = db.registerForNotifications(u, identity2, token2)
	if err != nil {
		t.Fatal(err)
	}

	ru, err := db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ru.Tokens) != 2 || len(ru.Identities) != 2 || !bytes.Equal(ru.TransmissionRSAHash, u.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user\n\tExpected: %+v\n\t: Receiveid: %+v\n", u, ru)
	}

	err = db.unregisterTokens(u, []Token{Token{Token: token}})
	if err != nil {
		t.Fatalf("Failed to unregister token: %+v", err)
	}
	ru, err = db.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ru.Tokens) != 1 || len(ru.Identities) != 2 || !bytes.Equal(ru.TransmissionRSAHash, u.TransmissionRSAHash) {
		t.Fatalf("Did not receive expected user\n\tExpected: %+v\n\t: Receiveid: %+v\n", u, ru)
	}

	err = db.unregisterTokens(u, []Token{Token{Token: token2}})
	if err != nil {
		t.Fatalf("Failed to unregister token: %+v", err)
	}
	_, err = db.GetUser(u.TransmissionRSAHash)
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("Expected gorm.ErrRecordNotFound when no user exists, instead got %+v", err)
	}

	_, err = db.getIdentity(identity.IntermediaryId)
	if err != nil {
		t.Fatalf("Failed to get identity: %+v", err)
	}
}

//func TestDatabaseImpl_LegacyUnregister(t *testing.T) {
//	db, err := newDatabase("", "", "", "", "")
//	if err != nil {
//		t.Fatal(err)
//	}
//}

func generateTestIdentity(t *testing.T) Identity {
	uid, err := id.NewRandomID(csprng.NewSystemRNG(), id.User)
	if err != nil {
		t.Fatal(err)
	}
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Fatal(err)
	}
	identity := Identity{
		IntermediaryId: iid,
		OffsetNum:      ephemeral.GetOffsetNum(ephemeral.GetOffset(iid)),
	}
	return identity
}
func generateTestUser(t *testing.T) *User {
	trsa, err := rsa.GenerateKey(csprng.NewSystemRNG(), 512)
	if err != nil {
		t.Fatal(err)
	}
	h := hash.CMixHash.New()
	h.Write(trsa.GetPublic().Bytes())
	sig := []byte("fake signature")
	u := &User{
		TransmissionRSAHash: h.Sum(nil),
		TransmissionRSA:     trsa.GetPublic().Bytes(),
		Signature:           sig,
	}
	return u
}
