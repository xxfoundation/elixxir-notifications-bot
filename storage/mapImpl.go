////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the implementation of the map backend

package storage

import (
	"errors"
)

// Obtain user from backend
func (m *MapImpl) GetUser(userId string) (*User, error) {
	// Attempt to load from map
	v, found := m.users.Load(userId)
	// Check if it was found, Load function sets it as a bool
	if found == false {
		return nil, errors.New("user could not be found")
	}

	return v.(*User), nil
}

// Delete user from backend
func (m *MapImpl) DeleteUser(userId string) error {
	// Check if user exists, if it doesn't return an error
	_, err := m.GetUser(userId)
	if err != nil {
		return err
	}

	m.users.Delete(userId)

	return nil
}

// Insert or update user into backend
func (m *MapImpl) UpsertUser(user *User) error {
	// Check to see if user exists
	_, err := m.GetUser(user.Id)
	// If we don't get an error from the function, the user does exist
	if err == nil {
		// Delete the user so we can insert a new version
		err = m.DeleteUser(user.Id)
		// Check that the deletion did not error
		if err != nil {
			return err
		}
	}

	// Insert new user
	m.users.Store(user.Id, user)

	return nil
}
