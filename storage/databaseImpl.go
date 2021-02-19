////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles implementation of the database backend

package storage

import (
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// Obtain User from backend by primary key
func (impl *DatabaseImpl) GetUser(userId []byte) (*User, error) {
	u := &User{}
	err := impl.db.Take(u, "id = ?", userId).Error
	if err != nil {
		return nil, errors.Errorf("Failed to retrieve user with ID %s: %+v", userId, err)
	}
	return u, nil
}

// Delete User from backend by primary key
func (impl *DatabaseImpl) deleteUser(transmissionRsaHash []byte) error {
	err := impl.db.Delete(&User{
		TransmissionRSAHash: transmissionRsaHash,
	}).Error
	if err != nil {
		return errors.Errorf("Failed to delete user with RSA hash %s: %+v", transmissionRsaHash, err)
	}
	return nil
}

// Insert or Update User into backend
func (impl *DatabaseImpl) upsertUser(user *User) error {
	newUser := *user

	return impl.db.Transaction(func(tx *gorm.DB) error {
		err := tx.FirstOrCreate(user, &User{TransmissionRSAHash: user.TransmissionRSAHash}).Error
		if err != nil {
			return err
		}

		if user.Token != newUser.Token {
			return tx.Save(&newUser).Error
		}

		return nil
	})
}

func (impl *DatabaseImpl) GetAllUsers() ([]*User, error) {
	var dest []*User
	return dest, impl.db.Find(&dest).Error
}
