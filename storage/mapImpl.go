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

// Obtain User from backend by primary key
func (m *MapImpl) GetUser(userId string) (*User, error) {
	// Attempt to load from map
	v, found := m.users.Load(userId)
	// Check if it was found, Load function sets it as a bool
	if found == false {
		return nil, errors.New("user could not be found")
	}

	return v.(*User), nil
}

// Delete User from backend by primary key
func (m *MapImpl) DeleteUser(userId string) error {
	m.users.Delete(userId)

	return nil
}

// Insert or Update User into backend
func (m *MapImpl) UpsertUser(user *User) error {
	// Insert new user
	m.users.Store(user.Id, user)

	return nil
}
