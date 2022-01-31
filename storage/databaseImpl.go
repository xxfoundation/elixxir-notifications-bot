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
func (d *DatabaseImpl) GetUser(userId []byte) (*User, error) {
	u := &User{}
	err := d.db.Take(u, "intermediary_id = ?", userId).Error
	if err != nil {
		return nil, err
	}
	return u, nil
}

// Obtain User from backend by primary key
func (d *DatabaseImpl) GetUserByHash(transmissionRsaHash []byte) (*User, error) {
	u := &User{}
	err := d.db.Take(u, "transmission_rsa_hash = ?", transmissionRsaHash).Error
	if err != nil {
		return nil, err
	}
	return u, nil
}

// Delete User from backend by primary key
func (d *DatabaseImpl) DeleteUserByHash(transmissionRsaHash []byte) error {
	err := d.db.Delete(&User{
		TransmissionRSAHash: transmissionRsaHash,
	}).Error
	if err != nil {
		return errors.Errorf("Failed to delete user with tRSA hash %s: %+v", transmissionRsaHash, err)
	}
	return nil
}

// Insert or Update User into backend
func (d *DatabaseImpl) upsertUser(user *User) error {
	newUser := *user

	return d.db.Transaction(func(tx *gorm.DB) error {
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

func (d *DatabaseImpl) GetAllUsers() ([]*User, error) {
	var dest []*User
	return dest, d.db.Find(&dest).Error
}

func (d *DatabaseImpl) GetOrphanedUsers() ([]*User, error) {
	var dest []*User
	return dest, d.db.Find(&dest, "NOT EXISTS (select * from ephemerals where ephemerals.transmission_rsa_hash = users.transmission_rsa_hash)").Error
}

func (d *DatabaseImpl) insertEphemeral(ephemeral *Ephemeral) error {
	return d.db.Create(&ephemeral).Error
}

func (d *DatabaseImpl) GetEphemeral(ephemeralId int64) ([]*Ephemeral, error) {
	var result []*Ephemeral
	err := d.db.Where("ephemeral_id = ?", ephemeralId).Find(&result).Error
	if err != nil {
		return nil, err
	}
	if len(result) < 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return result, nil
}

type GTNResult struct {
	EphemeralId          int64
	Token                string
	TransmissionRSAHash  []byte
	NotificationProvider uint8
}

func (d *DatabaseImpl) GetToNotify(ephemeralIds []int64) ([]GTNResult, error) {
	var result []GTNResult
	raw := "select ephemerals.ephemeral_id, users.transmission_rsa_hash, users.token, users.notification_provider from ephemerals left join users on ephemerals.transmission_rsa_hash = users.transmission_rsa_hash where ephemerals.ephemeral_id in ?;"
	return result, d.db.Raw(raw, ephemeralIds).Scan(&result).Error
}

func (d *DatabaseImpl) getUsersByOffset(offset int64) ([]*User, error) {
	var result []*User
	err := d.db.Where(&User{OffsetNum: offset}).Find(&result).Error
	return result, err
}

func (d *DatabaseImpl) DeleteOldEphemerals(currentEpoch int32) error {
	res := d.db.Where("epoch < ?", currentEpoch).Delete(&Ephemeral{})
	return res.Error
}

func (d *DatabaseImpl) GetLatestEphemeral() (*Ephemeral, error) {
	var result []*Ephemeral
	err := d.db.Order("epoch desc").Limit(1).Find(&result).Error
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
