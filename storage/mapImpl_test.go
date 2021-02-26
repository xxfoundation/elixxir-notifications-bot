package storage

import (
	"bytes"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"testing"
)

// This file contains testing for mapImpl.go

func TestDatabaseImpl(t *testing.T) {
	s, err := NewStorage("jonahhusson", "", "nbtest", "0.0.0.0", "5432")
	if err != nil {
		t.Errorf("Failed to create db: %+v", err)
		t.FailNow()
	}
	sig := []byte("sig")
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to make iid: %+v", err)
	}
	token1 := "i'm a token"
	_, err = s.AddUser(iid, []byte("rsa"), sig, token1)
	if err != nil {
		t.Errorf("Failed to upsert user: %+v", err)
	}

	u, err := s.GetUser(iid)
	if err != nil {
		t.Errorf("Failed to get user: %+v", err)
	}
	if u.Token != token1 {
		t.Errorf("Expected user with token %s.  Instead got %s.", token1, u.Token)
	}

	token2 := "you're a token"
	_, err = s.AddUser(iid, []byte("rsa"), sig, token2)
	if err != nil {
		t.Errorf("Failed to upsert updated user: %+v", err)
	}

	u, err = s.GetUser(iid)
	if err != nil {
		t.Errorf("Failed to get user: %+v", err)
	}
	if u.Token != token2 {
		t.Errorf("Expected user with token %s.  Instead got %s.", token1, u.Token)
	}

	_, err = s.AddUser([]byte("jakexx360"), []byte("rsa2"), sig, token2)
	if err != nil {
		t.Errorf("Failed to upsert updated user: %+v", err)
	}

	us, err := s.GetAllUsers()
	if err != nil {
		t.Errorf("Failed to get all users: %+v", err)
	}
	if len(us) != 2 {
		t.Errorf("Did not get enough users: %+v", us)
	}

	err = s.deleteUser(u.TransmissionRSAHash)
	if err != nil {
		t.Errorf("Failed to delete user: %+v", err)
	}
}

// This tests getting a user that does exist in the database
func TestMapImpl_GetUser_Happy(t *testing.T) {
	m := &MapImpl{
		usersByRsaHash: map[string]*User{},
		usersById:      map[string]*User{},
	}
	u := &User{IntermediaryId: []byte("test"), Token: "token", TransmissionRSAHash: []byte("hash")}
	m.usersById[string(u.IntermediaryId)] = u
	m.usersByRsaHash[string(u.TransmissionRSAHash)] = u
	m.allUsers = append(m.allUsers, u)

	user, err := m.GetUser(u.IntermediaryId)

	// Check that we got a user back
	if user == nil {
		t.Errorf("TestMapImpl_GetUser_Happy: function did not return a user")
	} else {
		// Perform additional tests on the user var if we're sure it's populated
		if bytes.Compare(user.IntermediaryId, u.IntermediaryId) != 0 {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned "+
				"user with different ID\n\tGot: %s\n\tExpected: %s", user.IntermediaryId, u.IntermediaryId)
		}

		if user.Token != u.Token {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned "+
				"user with different token\n\tGot: %s\n\tExpected: %s", user.Token, u.Token)
		}
	}

	if err != nil {
		t.Errorf("TestMapImpl_GetUser_Happy: function returned an error\n\tGot: %s", err)
	}
}

// This tests getting a user that does *not* exist in the database
func TestMapImpl_GetUser_NoUser(t *testing.T) {
	m := &MapImpl{
		usersByRsaHash: map[string]*User{},
		usersById:      map[string]*User{},
	}
	u := &User{IntermediaryId: []byte("test"), Token: "token", TransmissionRSAHash: []byte("hash")}

	user, err := m.GetUser(u.IntermediaryId)

	if user != nil {
		t.Errorf("TestMapImpl_GetUser_NoUser: function returned a user\n\tGot: %s", user.IntermediaryId)
	}

	if err == nil {
		t.Errorf("TestMapImpl_GetUser_NoUser: function did not return an error")
	}
}

// This tests deleting a user that does exist in the database
func TestMapImpl_DeleteUser_Happy(t *testing.T) {
	m := &MapImpl{
		usersByRsaHash: map[string]*User{},
		usersById:      map[string]*User{},
	}
	u := &User{IntermediaryId: []byte("test"), Token: "token", TransmissionRSAHash: []byte("hash")}
	m.usersById[string(u.IntermediaryId)] = u
	m.usersByRsaHash[string(u.TransmissionRSAHash)] = u
	m.allUsers = append(m.allUsers, u)

	err := m.deleteUser(u.TransmissionRSAHash)

	if err != nil {
		t.Errorf("TestMapImpl_DeleteUser_Happy: function returned error\n\tGot: %s", err)
	}

	// Try to load user from map manually
	_, ok := m.usersById[string(u.IntermediaryId)]
	if ok == true {
		t.Errorf("TestMapImpl_DeleteUser_Happy: user existed in database after deletion called")
	}
	_, ok = m.usersByRsaHash[string(u.TransmissionRSAHash)]
	if ok == true {
		t.Errorf("TestMapImpl_DeleteUser_Happy: user existed in database after deletion called")
	}
	if len(m.allUsers) != 0 {
		t.Errorf("Should have deleted from allUsers")
	}
}

// This tests inserting a user once and verifying we can read it back right
func TestMapImpl_UpsertUser_Happy(t *testing.T) {
	m := &MapImpl{
		usersByRsaHash: map[string]*User{},
		usersById:      map[string]*User{},
	}
	u := User{IntermediaryId: []byte("test"), Token: "token", TransmissionRSAHash: []byte("rsahash")}

	err := m.upsertUser(&u)

	if err != nil {
		t.Errorf("TestMapImpl_UpsertUser_Happy: function returned an error\n\tGot: %s", err)
	}

	// Load user from map manually
	user, ok := m.usersByRsaHash[string(u.TransmissionRSAHash)]
	// Check that a user was found
	if ok != true {
		t.Errorf("TestMapImpl_UpsertUser_Happy: loading user from map manually did not return user")
	} else {
		// If a user is found, make sure it's our test user
		if bytes.Compare(user.IntermediaryId, u.IntermediaryId) != 0 {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned "+
				"user with different ID\n\tGot: %s\n\tExpected: %s", user.IntermediaryId, u.IntermediaryId)
		}

		if user.Token != u.Token {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned "+
				"user with different token\n\tGot: %s\n\tExpected: %s", user.Token, u.Token)
		}
	}
}

// This tests inserting a user *twice* and verifying we can read it back right each time
func TestMapImpl_UpsertUser_HappyTwice(t *testing.T) {
	m := &MapImpl{
		usersByRsaHash: map[string]*User{},
		usersById:      map[string]*User{},
	}
	u := User{IntermediaryId: []byte("test"), Token: "token", TransmissionRSAHash: []byte("RsaHash")}

	err := m.upsertUser(&u)

	if err != nil {
		t.Errorf("TestMapImpl_UpsertUser_Happy: function returned an error\n\tGot: %s", err)
	}

	// Load user from map manually
	user, ok := m.usersByRsaHash[string(u.TransmissionRSAHash)]
	// Check that a user was found
	if ok != true {
		t.Errorf("TestMapImpl_UpsertUser_Happy: loading user from map manually did not return user")
	} else {
		// If a user is found, make sure it's our test user
		if bytes.Compare(user.IntermediaryId, u.IntermediaryId) != 0 {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned "+
				"user with different ID\n\tGot: %s\n\tExpected: %s", user.IntermediaryId, u.IntermediaryId)
		}

		if user.Token != u.Token {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned "+
				"user with different token\n\tGot: %s\n\tExpected: %s", user.Token, u.Token)
		}
	}

	// Create user with the same ID but change the token
	u2 := User{IntermediaryId: []byte("test"), Token: "othertoken", TransmissionRSAHash: []byte("othertransmissionrsahash")}
	err = m.upsertUser(&u2)

	// Load user from map manually
	user, ok = m.usersByRsaHash[string(u2.TransmissionRSAHash)]
	// Check that a user was found
	if ok != true {
		t.Errorf("TestMapImpl_UpsertUser_Happy: loading user from map manually did not return user")
	} else {
		// If a user is found, make sure it's our test user
		if bytes.Compare(user.IntermediaryId, u2.IntermediaryId) != 0 {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned "+
				"user with different ID\n\tGot: %s\n\tExpected: %s", user.IntermediaryId, u.IntermediaryId)
		}

		if user.Token != u2.Token {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned "+
				"user with different token\n\tGot: %s\n\tExpected: %s", user.Token, u2.Token)
		}
	}
}
