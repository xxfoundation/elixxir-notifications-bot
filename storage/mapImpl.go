////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the implementation of the map backend

package storage

// Obtain User from backend by primary key
func (*MapImpl) GetUser(userId string) (*User, error) {
	// TODO: Implement
	return nil, nil
}

// Delete User from backend by primary key
func (*MapImpl) DeleteUser(userId string) error {
	// TODO: Implement
	return nil
}

// Insert or Update User into backend
func (*MapImpl) UpsertUser(user *User) error {
	// TODO: Implement
	return nil
}
