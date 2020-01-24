////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles implementation of the database backend

package storage

// Obtain User from backend by primary key
func (*DatabaseImpl) GetUser(userId string) (*User, error) {
	// TODO: Implement
	return nil, nil
}

// Delete User from backend by primary key
func (*DatabaseImpl) DeleteUser(userId string) error {
	// TODO: Implement
	return nil
}

// Insert or Update User into backend
func (*DatabaseImpl) UpsertUser(user *User) error {
	// TODO: Implement
	return nil
}
