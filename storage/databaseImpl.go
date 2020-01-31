////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles implementation of the database backend

package storage

import "github.com/pkg/errors"

// Obtain User from backend by primary key
func (impl *DatabaseImpl) GetUser(userId string) (*User, error) {
	u := &User{
		Id: userId,
	}
	err := impl.db.Select(u)
	if err != nil {
		return nil, errors.Errorf("Failed to retrieve user with ID %s: %+v", userId, err)
	}
	return u, nil
}

// Delete User from backend by primary key
func (impl *DatabaseImpl) DeleteUser(userId string) error {
	err := impl.db.Delete(&User{
		Id: userId,
	})
	if err != nil {
		return errors.Errorf("Failed to delete user with ID %s: %+v", userId, err)
	}
	return nil
}

// Insert or Update User into backend
func (impl *DatabaseImpl) UpsertUser(user *User) error {
	_, err := impl.db.Model(user).
		OnConflict("(Id) DO UPDATE").
		Set("Token = EXCLUDED.Token").Returning("").Insert()
	if err != nil {
		return errors.Errorf("Failed to insert user %s: %+v", user.Id, err)
	}
	return nil
}
