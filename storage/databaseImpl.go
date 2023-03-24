////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Handles implementation of the database backend

package storage

import (
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Inserts the given State into Storage if it does not exist
// Or updates the Database State if its value does not match the given State
func (d *DatabaseImpl) UpsertState(state *State) error {
	jww.TRACE.Printf("Attempting to insert State into DB: %+v", state)

	// Build a transaction to prevent race conditions
	return d.db.Transaction(func(tx *gorm.DB) error {
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).Create(state).Error
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
	return d.db.Where("token = ?", token).Delete(&Token{Token: token}).Error
}

// Insert or Update User into backend
func (d *DatabaseImpl) insertUser(user *User) error {
	return d.db.Clauses(clause.OnConflict{DoNothing: true}).Create(user).Error
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
	return d.db.Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(identity).Error
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
	err := d.db.Transaction(func(tx *gorm.DB) error {
		t1 := tx.Table("identities").Select("ephemerals.ephemeral_id, identities.intermediary_id").Joins("inner join ephemerals on ephemerals.intermediary_id = identities.intermediary_id").Where("ephemerals.ephemeral_id in ?", ephemeralIds)
		t2 := tx.Table("user_identities").Select("t1.ephemeral_id, user_identities.user_transmission_rsa_hash as transmission_rsa_hash").Joins("left join (?) t1 on t1.intermediary_id = user_identities.identity_intermediary_id", t1)
		t3 := tx.Model(&User{}).Select("users.transmission_rsa_hash, t2.ephemeral_id").Joins("right join (?) as t2 on users.transmission_rsa_hash = t2.transmission_rsa_hash", t2)
		return tx.Model(&Token{}).Distinct().Select("tokens.token, t3.transmission_rsa_hash, t3.ephemeral_id").Joins("left join (?) as t3 on tokens.transmission_rsa_hash = t3.transmission_rsa_hash", t3).Scan(&result).Error
	})
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

func (d *DatabaseImpl) registerForNotifications(u *User, identity Identity, token string) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Model(u).Association("Identities").Append(&identity)
		if err != nil {
			return errors.WithMessage(err, "Failed to register identity")
		}

		err = tx.Model(u).Association("Tokens").Append(&Token{
			Token:               token,
			TransmissionRSAHash: u.TransmissionRSAHash,
		})
		if err != nil {
			return errors.WithMessage(err, "Failed to register token")
		}
		return nil
	})
}

func (d *DatabaseImpl) unregisterIdentities(u *User, iids []Identity) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&u).Association("Identities").Delete(iids)
		if err != nil {
			return errors.WithMessage(err, "Failed to break association")
		}
		for _, iid := range iids {
			var count int64
			err = tx.Table("user_identities").Where("identity_intermediary_id = ?", iid.IntermediaryId).Count(&count).Error
			if err != nil {
				return errors.WithMessagef(err, "Failed count user_identities for identity %+v", iid.IntermediaryId)
			}
			if count == 0 {
				err = tx.Delete(iid).Error
				if err != nil {
					return errors.WithMessage(err, "Failed to delete identity")
				}
			}

			err = tx.Table("user_identities").Where("user_transmission_rsa_hash = ?", u.TransmissionRSAHash).Count(&count).Error
			if err != nil {
				return errors.WithMessagef(err, "Failed to count user_identities for user %+v", u.TransmissionRSAHash)
			}
			if count == 0 {
				err = tx.Delete(u).Error
				if err != nil {
					return errors.WithMessage(err, "Failed to delete user")
				}
			}
		}
		return nil
	})
}

func (d *DatabaseImpl) unregisterTokens(u *User, tokens []Token) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		for _, t := range tokens {
			err := tx.Delete(t).Error
			if err != nil {
				return errors.WithMessage(err, "Failed to delete token")
			}
		}

		count := tx.Model(u).Association("Tokens").Count()

		if count == 0 {
			err := tx.Model(&u).Association("Identities").Clear()
			if err != nil {
				return errors.WithMessage(err, "Failed to prep user for delete")
			}
			err = tx.Delete(&u).Error
			if err != nil {
				return errors.WithMessage(err, "Failed to delete user")
			}
		}
		return nil
	})
}

func (d *DatabaseImpl) LegacyUnregister(iid []byte) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		var res Identity
		err := tx.Preload("Users").Find(&res, "intermediary_id = ?", iid).Error
		if err != nil {
			return err
		}
		if len(res.Users) > 1 {
			return errors.Errorf("legacyUnregister can only be called for identities with a single associated user")
		}

		err = tx.Model(&Identity{IntermediaryId: iid}).Association("Users").Clear()
		if err != nil {
			return errors.WithMessage(err, "Failed to break association")
		}

		err = tx.Delete(&Identity{IntermediaryId: iid}).Error
		if err != nil {
			return errors.WithMessage(err, "Failed to delete identity")
		}
		err = tx.Delete(&User{TransmissionRSAHash: res.Users[0].TransmissionRSAHash}).Error
		if err != nil {
			return errors.WithMessage(err, "Failed to delete user")
		}
		return nil
	})
}
