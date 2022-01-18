////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles implementation of the database backend

package storage

import (
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gorm.io/gorm"
)

// Obtain User from backend by primary key
func (impl *DatabaseImpl) GetUser(userId []byte) (*User, error) {
	u := &User{}
	err := impl.db.Take(u, "intermediary_id = ?", userId).Error
	if err != nil {
		return nil, err
	}
	return u, nil
}

// Obtain User from backend by primary key
func (impl *DatabaseImpl) GetUserByHash(transmissionRsaHash []byte) (*User, error) {
	u := &User{}
	err := impl.db.Take(u, "transmission_rsa_hash = ?", transmissionRsaHash).Error
	if err != nil {
		return nil, err
	}
	return u, nil
}

// Delete User from backend by primary key
func (impl *DatabaseImpl) DeleteUserByHash(transmissionRsaHash []byte) error {
	err := impl.db.Delete(&User{
		TransmissionRSAHash: transmissionRsaHash,
	}).Error
	if err != nil {
		return errors.Errorf("Failed to delete user with tRSA hash %s: %+v", transmissionRsaHash, err)
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

func (impl *DatabaseImpl) insertEphemeral(ephemeral *Ephemeral) error {
	return impl.db.Create(&ephemeral).Error
}

func (impl *DatabaseImpl) GetEphemeral(ephemeralId int64) ([]*Ephemeral, error) {
	var result []*Ephemeral
	err := impl.db.Where("ephemeral_id = ?", ephemeralId).Find(&result).Error
	if err != nil {
		return nil, err
	}
	if len(result) < 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return result, nil
}

func (impl *DatabaseImpl) getUsersByOffset(offset int64) ([]*User, error) {
	var result []*User
	err := impl.db.Where(&User{OffsetNum: offset}).Find(&result).Error
	return result, err
}

func (impl *DatabaseImpl) DeleteOldEphemerals(currentEpoch int32) error {
	res := impl.db.Where("epoch < ?", currentEpoch).Delete(&Ephemeral{})
	return res.Error
}

func (impl *DatabaseImpl) GetLatestEphemeral() (*Ephemeral, error) {
	var result []*Ephemeral
	err := impl.db.Order("epoch desc").Limit(1).Find(&result).Error
	if err != nil {
		return nil, err
	}
	if len(result) < 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return result[0], nil
}

// Inserts the given State into Storage if it does not exist
// Or updates the Database State if its value does not match the given State
func (d *DatabaseImpl) UpsertState(state *State) error {
	jww.TRACE.Printf("Attempting to insert State into DB: %+v", state)

	// Build a transaction to prevent race conditions
	return d.db.Transaction(func(tx *gorm.DB) error {
		// Make a copy of the provided state
		newState := *state

		// Attempt to insert state into the Database,
		// or if it already exists, replace state with the Database value
		err := tx.FirstOrCreate(state, &State{Key: state.Key}).Error
		if err != nil {
			return err
		}

		// If state is already present in the Database, overwrite it with newState
		if newState.Value != state.Value {
			return tx.Save(newState).Error
		}

		// Commit
		return nil
	})
}

// Returns a State's value from Storage with the given key
// Or an error if a matching State does not exist
func (d *DatabaseImpl) GetStateValue(key string) (string, error) {
	result := &State{Key: key}
	err := d.db.Take(result).Error
	jww.TRACE.Printf("Obtained State from DB: %+v", result)
	return result.Value, err
}
