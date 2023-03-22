////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	"encoding/base64"
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

// Create a new Storage object wrapping a database interface
// Returns a Storage object and error
func NewStorage(username, password, dbName, address, port string) (*Storage, error) {
	db, err := newDatabase(username, password, dbName, address, port)
	nb := NewNotificationBuffer()
	storage := &Storage{db, nb}
	return storage, err
}

func (s *Storage) RegisterForNotifications(iid, transmissionRSA, signature []byte, token string, epoch int32, addressSpace uint8) (*User, error) {
	h, err := hash.NewCMixHash()
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to create cmix hash")
	}
	_, err = h.Write(transmissionRSA)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to hash transmission RSA")
	}
	transmissionRSAHash := h.Sum(nil)

	identity, err := s.getIdentity(iid)
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
				Signature:           signature,
				Tokens: []Token{
					{Token: token, TransmissionRSAHash: transmissionRSAHash},
				}, Identities: []Identity{*identity},
			}
			return u, s.insertUser(u)
		} else {
			return nil, err
		}
	}

	return u, s.registerForNotifications(u, *identity, token)
}

func (s *Storage) UnregisterForNotifications(transmissionRSA []byte, iids [][]byte, tokens []string) error {
	h, err := hash.NewCMixHash()
	if err != nil {
		return errors.WithMessage(err, "Failed to create cmix hash")
	}
	_, err = h.Write(transmissionRSA)
	if err != nil {
		return errors.WithMessage(err, "Failed to hash transmission RSA")
	}
	transmissionRSAHash := h.Sum(nil)

	u, err := s.GetUser(transmissionRSAHash)
	if err != nil {
		return errors.WithMessagef(err, "Could not find user with transmission RSA hash %s", base64.StdEncoding.EncodeToString(transmissionRSAHash))
	}

	var identitiesToRemove []Identity
	for _, iid := range iids {
		identitiesToRemove = append(identitiesToRemove, Identity{IntermediaryId: iid})
	}

	err = s.unregisterIdentities(u, identitiesToRemove)
	if err != nil {
		return errors.WithMessagef(err, "Failed to unregister identities from user with transmission RSA hash %s", base64.StdEncoding.EncodeToString(transmissionRSAHash))
	}

	var tokensToRemove []Token
	for _, token := range tokens {
		// TODO: this will not work due to complications from multi-device support & changing tokens
		tokensToRemove = append(tokensToRemove, Token{Token: token})
	}

	err = s.unregisterTokens(u, tokensToRemove)
	if err != nil {
		return errors.WithMessagef(err, "Failed to unregister tokens from user with transmission RSA hash %s", base64.StdEncoding.EncodeToString(transmissionRSAHash))
	}

	return nil
}

//
//func (s *Storage) AddUser(iid, transmissionRSA, signature []byte, token string) (*User, error) {
//	h, err := hash.NewCMixHash()
//	if err != nil {
//		return nil, errors.WithMessage(err, "Failed to create cmix hash")
//	}
//	_, err = h.Write(transmissionRSA)
//	if err != nil {
//		return nil, errors.WithMessage(err, "Failed to hash transmission RSA")
//	}
//	u := &User{
//		IntermediaryId:      iid,
//		TransmissionRSAHash: h.Sum(nil),
//		TransmissionRSA:     transmissionRSA,
//		Signature:           signature,
//		OffsetNum:           ephemeral.GetOffsetNum(ephemeral.GetOffset(iid)),
//		Token:               token,
//	}
//	jww.INFO.Printf("Adding user %+v with token %s", u.TransmissionRSAHash, token)
//	return u, s.upsertUser(u)
//}

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

//
//func (s *Storage) deleteUser(transmissionRSA []byte) error {
//	h, err := hash.NewCMixHash()
//	if err != nil {
//		return errors.WithMessage(err, "Failed to create cmix hash")
//	}
//	_, err = h.Write(transmissionRSA)
//	if err != nil {
//		return errors.WithMessage(err, "Failed to hash transmission RSA")
//	}
//	return s.DeleteUserByHash(h.Sum(nil))
//}

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
