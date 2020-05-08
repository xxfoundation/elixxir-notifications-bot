////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles implementation of the database backend

package storage

import (
	"github.com/pkg/errors"
	"gitlab.com/elixxir/primitives/id"
)

// Obtain User from backend by primary key
func (impl *DatabaseImpl) GetUser(userId *id.ID) (*User, error) {
	u := &User{
		Id: encodeUser(userId),
	}
	err := impl.db.Select(u)
	if err != nil {
		return nil, errors.Errorf("Failed to retrieve user with ID %s: %+v", userId, err)
	}
	return u, nil
}

// Delete User from backend by primary key
func (impl *DatabaseImpl) DeleteUser(userId *id.ID) error {
	err := impl.db.Delete(&User{
		Id: encodeUser(userId),
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
		Set("Token = EXCLUDED.Token").Insert()
	if err != nil {
		return errors.Errorf("Failed to insert user %s: %+v", user.Id, err)
	}
	return nil
}
