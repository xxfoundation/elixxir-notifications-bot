////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	"fmt"
	"github.com/pkg/errors"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gorm.io/gorm"
	"time"
)

type Storage struct {
	database
	notificationBuffer *NotificationBuffer
}

// NewStorage creates a new Storage object with the given connection parameters
func NewStorage(username, password, dbName, address, port string) (*Storage, error) {
	db, err := newDatabase(username, password, dbName, address, port)
	nb := NewNotificationBuffer()
	storage := &Storage{db, nb}
	return storage, err
}

// RegisterToken registers a token to a user based on their transmission RSA
func (s *Storage) RegisterToken(token, app string, transmissionRSA []byte) error {
	transmissionRSAHash, err := getHash(transmissionRSA)
	if err != nil {
		return errors.WithMessage(err, "Failed to hash transmisssion RSA")
	}

	u, err := s.GetUser(transmissionRSAHash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			u = &User{
				TransmissionRSAHash: transmissionRSAHash,
				TransmissionRSA:     transmissionRSA,
				Tokens: []Token{
					{Token: token, TransmissionRSAHash: transmissionRSAHash},
				},
			}
			return s.insertUser(u)
		} else {
			return err
		}
	}

	return s.database.insertToken(Token{
		App:                 app,
		Token:               token,
		TransmissionRSAHash: transmissionRSAHash,
	})
}

// UnregisterToken token unregisters a token from the user with the passed in RSA
func (s *Storage) UnregisterToken(token string, transmissionRSA []byte) error {
	transmissionRSAHash, err := getHash(transmissionRSA)
	if err != nil {
		return errors.WithMessage(err, "Failed to hash transmisssion RSA")
	}

	u, err := s.GetUser(transmissionRSAHash)
	if err != nil {
		// TODO: should this return an error?
		return nil
	}

	err = s.database.unregisterTokens(u, []Token{{Token: token}})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return nil
}

// RegisterTrackedID registers a tracked ID for the user with the passed in RSA
func (s *Storage) RegisterTrackedID(iidList [][]byte, transmissionRSA []byte, epoch int32, addressSpace uint8) error {
	transmissionRSAHash, err := getHash(transmissionRSA)
	if err != nil {
		return errors.WithMessage(err, "Failed to hash transmisssion RSA")
	}

	u, err := s.GetUser(transmissionRSAHash)
	if err != nil {
		return errors.WithMessage(err, "Cannot register tracked ID to unregistered user")
	}

	var ids []Identity
	for _, iid := range iidList {
		identity, err := s.GetIdentity(iid)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				identity = &Identity{
					IntermediaryId: iid,
					OffsetNum:      ephemeral.GetOffsetNum(ephemeral.GetOffset(iid)),
				}
				err = s.insertIdentity(identity)
				if err != nil {
					return err
				}
				_, err = s.AddLatestEphemeral(identity, epoch, uint(addressSpace))
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
		ids = append(ids, *identity)
	}

	return s.database.registerTrackedIdentities(*u, ids)
}

// UnregisterTrackedIDs unregisters a tracked id from the user with the passed in RSA
func (s *Storage) UnregisterTrackedIDs(trackedIdList [][]byte, transmissionRSA []byte) error {
	transmissionRSAHash, err := getHash(transmissionRSA)
	if err != nil {
		return errors.WithMessage(err, "Failed to hash transmisssion RSA")
	}

	u, err := s.GetUser(transmissionRSAHash)
	if err != nil {
		// TODO: should this return an error?
		return nil
	}

	var ids []Identity
	for _, i := range trackedIdList {
		ids = append(ids, Identity{IntermediaryId: i})
	}

	err = s.database.unregisterIdentities(u, ids)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return nil
}

// RegisterForNotifications registers a user with the passed in transmissionRSA
// to receive notifications on the identity with intermediary id iid, with the passed in token
func (s *Storage) RegisterForNotifications(iid, transmissionRSA []byte, token, app string, epoch int32, addressSpace uint8) (*User, error) {
	transmissionRSAHash, err := getHash(transmissionRSA)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to hash transmisssion RSA")
	}
	identity, err := s.GetIdentity(iid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			identity = &Identity{
				IntermediaryId: iid,
				OffsetNum:      ephemeral.GetOffsetNum(ephemeral.GetOffset(iid)),
			}
			err = s.insertIdentity(identity)
			if err != nil {
				return nil, err
			}
			_, err = s.AddLatestEphemeral(identity, epoch, uint(addressSpace))
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	u, err := s.GetUser(transmissionRSAHash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			u = &User{
				TransmissionRSAHash: transmissionRSAHash,
				TransmissionRSA:     transmissionRSA,
				Tokens: []Token{
					{Token: token, TransmissionRSAHash: transmissionRSAHash, App: app},
				}, Identities: []Identity{*identity},
			}
			return u, s.insertUser(u)
		} else {
			return nil, err
		}
	}

	return u, s.registerForNotifications(u, *identity, Token{Token: token, App: app, TransmissionRSAHash: transmissionRSAHash})
}

// AddLatestEphemeral generates an ephemeral ID for the passed in identity and adds it to storage
func (s *Storage) AddLatestEphemeral(i *Identity, epoch int32, size uint) (*Ephemeral, error) {
	now := time.Now()
	eid, _, _, err := ephemeral.GetIdFromIntermediary(i.IntermediaryId, size, now.UnixNano())
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to get ephemeral id for user")
	}

	e := &Ephemeral{
		IntermediaryId: i.IntermediaryId,
		EphemeralId:    eid.Int64(),
		Epoch:          epoch,
		Offset:         i.OffsetNum,
	}
	err = s.insertEphemeral(e)
	if err != nil {
		return nil, err
	}

	eid2, _, _, err := ephemeral.GetIdFromIntermediary(i.IntermediaryId, size, now.Add(5*time.Minute).UnixNano())
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to get ephemeral id for user")
	}
	if eid2.Int64() != eid.Int64() {
		e := &Ephemeral{
			IntermediaryId: i.IntermediaryId,
			EphemeralId:    eid2.Int64(),
			Epoch:          epoch + 1,
			Offset:         i.OffsetNum,
		}
		fmt.Printf("Adding ephemeral: %+v\n", e)
		err = s.insertEphemeral(e)
		if err != nil {
			return nil, err
		}
	}

	return e, err
}

// AddEphemeralsForOffset generates new ephemerals for all identities with the given offset, using the passed in parameters
func (s *Storage) AddEphemeralsForOffset(offset int64, epoch int32, size uint, t time.Time) error {
	identities, err := s.getIdentitiesByOffset(offset)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.WithMessage(err, "Failed to get users for given offset")
	}
	if len(identities) > 0 {
		fmt.Println(fmt.Sprintf("Adding ephemerals for identities: %+v", identities))
	}
	for _, i := range identities {
		eid, _, _, err := ephemeral.GetIdFromIntermediary(i.IntermediaryId, size, t.UnixNano())
		if err != nil {
			return errors.WithMessage(err, "Failed to get eid for user")
		}
		err = s.insertEphemeral(&Ephemeral{
			IntermediaryId: i.IntermediaryId,
			EphemeralId:    eid.Int64(),
			Epoch:          epoch,
			Offset:         offset,
		})
		if err != nil {
			return errors.WithMessage(err, "Failed to insert ephemeral ID for user")
		}
	}
	return nil
}

func (s *Storage) GetNotificationBuffer() *NotificationBuffer {
	return s.notificationBuffer
}

func getHash(transmissionRSA []byte) (transmissionRSAHash []byte, err error) {
	h, err := hash.NewCMixHash()
	if err != nil {
		return
	}
	_, err = h.Write(transmissionRSA)
	if err != nil {
		return
	}
	transmissionRSAHash = h.Sum(nil)
	return
}
