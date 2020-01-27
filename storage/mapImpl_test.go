package storage

import "testing"

// This file contains testing for mapImpl.go

// This tests getting a user that does exist in the database
func TestMapImpl_GetUser_Happy(t *testing.T) {
	m := &MapImpl{}
	u := User{Id:"test", Token:"token"}
	m.users.Store(u.Id, &u)

	user, err := m.GetUser(u.Id)

	// Check that we got a user back
	if user == nil {
		t.Errorf("TestMapImpl_GetUser_Happy: function did not return a user")
	} else {
		// Perform additional tests on the user var if we're sure it's populated
		if user.Id != u.Id {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned " +
				"user with different ID\n\tGot: %s\n\tExpected: %s", user.Id, u.Id)
		}

		if user.Token != u.Token {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned " +
				"user with different token\n\tGot: %s\n\tExpected: %s", user.Token, u.Token)
		}
	}

	if err != nil {
		t.Errorf("TestMapImpl_GetUser_Happy: function returned an error\n\tGot: %s", err)
	}
}

// This tests getting a user that does *not* exist in the database
func TestMapImpl_GetUser_NoUser(t *testing.T) {
	m := &MapImpl{}
	u := User{Id:"test", Token:"token"}

	user, err := m.GetUser(u.Id)

	if user != nil {
		t.Errorf("TestMapImpl_GetUser_NoUser: function returned a user\n\tGot: %s", user.Id)
	}

	if err == nil {
		t.Errorf("TestMapImpl_GetUser_NoUser: function did not return an error")
	}
}

// This tests deleting a user that does exist in the database
func TestMapImpl_DeleteUser_Happy(t *testing.T) {
	m := &MapImpl{}
	u := User{Id:"test", Token:"token"}
	m.users.Store(u.Id, &u)

	err := m.DeleteUser(u.Id)

	if err != nil {
		t.Errorf("TestMapImpl_DeleteUser_Happy: function returned error\n\tGot: %s", err)
	}

	// Try to oad user from map manually
	_, ok := m.users.Load(u.Id)
	if ok == true {
		t.Errorf("TestMapImpl_DeleteUser_Happy: user existed in database after deletion called")
	}
}

// This tests inserting a user once and verifying we can read it back right
func TestMapImpl_UpsertUser_Happy(t *testing.T) {
	m := &MapImpl{}
	u := User{Id:"test", Token:"token"}

	err := m.UpsertUser(&u)

	if err != nil {
		t.Errorf("TestMapImpl_UpsertUser_Happy: function returned an error\n\tGot: %s", err)
	}

	// Load user from map manually
	user, ok := m.users.Load(u.Id)
	// Check that a user was found
	if ok != true {
		t.Errorf("TestMapImpl_UpsertUser_Happy: loading user from map manually did not return user")
	} else {
		// If a user is found, make sure it's our test user
		if user.(*User).Id != u.Id {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned " +
				"user with different ID\n\tGot: %s\n\tExpected: %s", user.(*User).Id, u.Id)
		}

		if user.(*User).Token != u.Token {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned " +
				"user with different token\n\tGot: %s\n\tExpected: %s", user.(*User).Token, u.Token)
		}
	}
}

// This tests inserting a user *twice* and verifying we can read it back right each time
func TestMapImpl_UpsertUser_HappyTwice(t *testing.T) {
	m := &MapImpl{}
	u := User{Id:"test", Token:"token"}

	err := m.UpsertUser(&u)

	if err != nil {
		t.Errorf("TestMapImpl_UpsertUser_Happy: function returned an error\n\tGot: %s", err)
	}

	// Load user from map manually
	user, ok := m.users.Load(u.Id)
	// Check that a user was found
	if ok != true {
		t.Errorf("TestMapImpl_UpsertUser_Happy: loading user from map manually did not return user")
	} else {
		// If a user is found, make sure it's our test user
		if user.(*User).Id != u.Id {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned " +
				"user with different ID\n\tGot: %s\n\tExpected: %s", user.(*User).Id, u.Id)
		}

		if user.(*User).Token != u.Token {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned " +
				"user with different token\n\tGot: %s\n\tExpected: %s", user.(*User).Token, u.Token)
		}
	}

	// Create user with the same ID but change the token
	u2 := User{Id:"test", Token:"othertoken"}
	err = m.UpsertUser(&u2)

	// Load user from map manually
	user, ok = m.users.Load(u2.Id)
	// Check that a user was found
	if ok != true {
		t.Errorf("TestMapImpl_UpsertUser_Happy: loading user from map manually did not return user")
	} else {
		// If a user is found, make sure it's our test user
		if user.(*User).Id != u2.Id {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned " +
				"user with different ID\n\tGot: %s\n\tExpected: %s", user.(*User).Id, u.Id)
		}

		if user.(*User).Token != u2.Token {
			t.Errorf("TestMapImpl_GetUser_Happy: function returned " +
				"user with different token\n\tGot: %s\n\tExpected: %s", user.(*User).Token, u2.Token)
		}
	}
}