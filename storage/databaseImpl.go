////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Handles implementation of the database backend

package storage

import (
	"bytes"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gorm.io/gorm"
)

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

func (d *DatabaseImpl) DeleteToken(token string) error {
	return d.db.Model(&Token{
		Token: token,
	}).Delete("token = ?", token).Error
}

// Insert or Update User into backend
func (d *DatabaseImpl) insertUser(user *User) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		ret := &User{}
		err := tx.FirstOrCreate(ret, user).Error
		if err != nil {
			return err
		}

		if bytes.Equal(ret.TransmissionRSAHash, user.TransmissionRSAHash) {
			return tx.Save(&user).Error
		}

		return nil
	})
}

// Obtain User from backend by primary key
func (d *DatabaseImpl) GetUser(transmissionRsaHash []byte) (*User, error) {
	u := &User{}
	err := d.db.Preload("Identities").Preload("Tokens").Take(u, "transmission_rsa_hash = ?", transmissionRsaHash).Error
	if err != nil {
		return nil, err
	}
	return u, nil
}

// Delete User from backend by primary key
func (d *DatabaseImpl) deleteUser(transmissionRsaHash []byte) error {
	err := d.db.Delete(&User{
		TransmissionRSAHash: transmissionRsaHash,
	}).Error
	if err != nil {
		return errors.Errorf("Failed to delete user with tRSA hash %s: %+v", transmissionRsaHash, err)
	}
	return nil
}

func (d *DatabaseImpl) GetAllUsers() ([]*User, error) {
	var dest []*User
	return dest, d.db.Find(&dest).Error
}

// Obtain Identity from backend by primary key
func (d *DatabaseImpl) getIdentity(iid []byte) (*Identity, error) {
	i := &Identity{}
	err := d.db.Take(i, "intermediary_id = ?", iid).Error
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (d *DatabaseImpl) insertIdentity(identity *Identity) error {
	return d.db.Create(identity).Error
}

func (d *DatabaseImpl) getIdentitiesByOffset(offset int64) ([]*Identity, error) {
	var result []*Identity
	err := d.db.Where(&Identity{OffsetNum: offset}).Find(&result).Error
	return result, err
}

func (d *DatabaseImpl) GetOrphanedIdentities() ([]*Identity, error) {
	var dest []*Identity
	return dest, d.db.Find(&dest, "NOT EXISTS (select * from ephemerals where ephemerals.intermediary_id = identities.intermediary_id)").Error
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
	Token               string
	TransmissionRSAHash []byte
	EphemeralId         int64
}

func (d *DatabaseImpl) GetToNotify(ephemeralIds []int64) ([]GTNResult, error) {
	var result []GTNResult
	raw := "Select distinct on (tokens.token) token, t3.transmission_rsa_hash, t3.ephemeral_id from tokens left join (Select users.transmission_rsa_hash, t2.ephemeral_id from users right join (Select t1.ephemeral_id, user_identities.user_transmission_rsa_hash as transmission_rsa_hash from user_identities left join (Select ephemerals.ephemeral_id, identities.intermediary_id from ephemerals inner join identities on ephemerals.intermediary_id = identities.intermediary_id where ephemerals.ephemeral_id in ?) as t1 on t1.intermediary_id = user_identities.identity_intermediary_id) as t2 on users.transmission_rsa_hash = t2.transmission_rsa_hash) as t3 on tokens.transmission_rsa_hash = t3.transmission_rsa_hash;"
	return result, d.db.Raw(raw, ephemeralIds).Scan(&result).Error
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

func (d *DatabaseImpl) registerForNotifications(u *User, identity Identity, token string) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Model(u).Association("Identities").Append(&identity)
		if err != nil {
			return err
		}

		err = tx.Model(u).Association("Tokens").Append(&Token{
			Token:               token,
			TransmissionRSAHash: u.TransmissionRSAHash,
		})
		if err != nil {
			return err
		}
		return nil
	})
}

func (d *DatabaseImpl) unregisterIdentities(u *User, iids []Identity) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&u).Association("Identities").Delete(iids)
		if err != nil {
			return err
		}
		for _, iid := range iids {
			var count int64
			err = tx.Table("user_identities").Where("identity_intermediary_id = ?", iid.IntermediaryId).Count(&count).Error
			if err != nil {
				return err
			}
			if count == 0 {
				err = tx.Delete(iid).Error
				if err != nil {
					return err
				}
			}

			err = tx.Table("user_identities").Where("user_transmission_rsa_hash = ?", iid.IntermediaryId).Count(&count).Error
			if err != nil {
				return err
			}
			if count == 0 {
				err = tx.Delete(u).Error
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (d *DatabaseImpl) unregisterTokens(u *User, tokens []Token) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		for _, t := range tokens {
			tx.Delete(t)
		}
		err := tx.Model(&u).Association("Tokens").Delete(tokens)
		if err != nil {
			return err
		}

		count := tx.Model(u).Association("Tokens").Count()

		if count == 0 {
			err = tx.Delete(&u).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *DatabaseImpl) LegacyUnregister(iid []byte) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		err := tx.Table("user_identities").Where("identity_intermediary_id = ?", iid).Count(&count).Error
		if err != nil {
			return err
		}
		if count > 1 {
			return errors.Errorf("legacyUnregister can only be called for identities with a single associated user")
		}
		var trsaHash []byte
		err = tx.Raw("select user_transmission_rsa_hash from user_identities where identity_intermediary_id = ?", iid).Scan(trsaHash).Error
		if err != nil {

		}

		err = d.db.Delete(&Identity{IntermediaryId: iid}).Error
		if err != nil {
			return err
		}
		err = d.db.Delete(&User{TransmissionRSAHash: trsaHash}).Error
		if err != nil {
			return err
		}
		return nil
	})
}
