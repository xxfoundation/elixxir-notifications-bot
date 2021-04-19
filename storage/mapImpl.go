////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the implementation of the map backend

package storage

import (
	"bytes"
	"errors"
	"fmt"
	"gorm.io/gorm"
)

// Obtain User from backend by primary key
func (m *MapImpl) GetUser(userId []byte) (*User, error) {
	// Attempt to load from map
	v, found := m.usersById[string(userId)]
	// Check if it was found, Load function sets it as a bool
	if found == false {
		return nil, errors.New("user could not be found")
	}

	return v, nil
}

func (m *MapImpl) GetUserByHash(transmissionRsaHash []byte) (*User, error) {
	// Attempt to load from map
	v, found := m.usersByRsaHash[string(transmissionRsaHash)]
	// Check if it was found, Load function sets it as a bool
	if found == false {
		return nil, errors.New("user could not be found")
	}

	return v, nil
}

// Delete User from backend by primary key
func (m *MapImpl) DeleteUserByHash(transmissionRsaHash []byte) error {
	user, ok := m.usersByRsaHash[string(transmissionRsaHash)]
	if !ok {
		return nil
	}
	delete(m.usersByRsaHash, string(transmissionRsaHash))
	delete(m.usersById, string(user.IntermediaryId))
	for i, u := range m.allUsers {
		if bytes.Compare(transmissionRsaHash, u.TransmissionRSAHash) == 0 {
			m.allUsers = append(m.allUsers[:i], m.allUsers[i+1:]...)
		}
	}
	for i, u := range m.usersByOffset[user.OffsetNum] {
		if bytes.Compare(transmissionRsaHash, u.TransmissionRSAHash) == 0 {
			m.usersByOffset[user.OffsetNum] = append(m.usersByOffset[user.OffsetNum][:i],
				m.usersByOffset[user.OffsetNum][i+1:]...)
		}
	}

	return nil
}

// Insert or Update User into backend
func (m *MapImpl) upsertUser(user *User) error {
	if u, ok := m.usersByRsaHash[string(user.TransmissionRSAHash)]; ok {
		if u.Token == user.Token {
			return nil
		}
		err := m.DeleteUserByHash(user.TransmissionRSAHash)
		if err != nil {
			return err
		}
	}
	// Insert new user
	m.usersByRsaHash[string(user.TransmissionRSAHash)] = user
	m.usersById[string(user.IntermediaryId)] = user
	var found bool
	for i, u := range m.usersByOffset[user.OffsetNum] {
		if string(user.TransmissionRSAHash) == string(u.TransmissionRSAHash) {
			found = true
			m.usersByOffset[user.OffsetNum][i] = user
			break
		}
	}
	if !found {
		m.usersByOffset[user.OffsetNum] = append(m.usersByOffset[user.OffsetNum], user)
	}
	m.allUsers = append(m.allUsers, user)

	return nil
}

func (m *MapImpl) GetAllUsers() ([]*User, error) {
	return m.allUsers, nil
}

func (m *MapImpl) upsertEphemeral(ephemeral *Ephemeral) error {
	m.ephIDSeq++
	ephemeral.ID = uint(m.ephIDSeq)
	m.ephemeralsById[ephemeral.EphemeralId] = ephemeral
	m.allEphemerals[int(ephemeral.ID)] = ephemeral
	return nil
}

func (m *MapImpl) GetEphemeral(ephemeralId int64) (*Ephemeral, error) {
	e, ok := m.ephemeralsById[ephemeralId]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Could not find ephemeral with transmission RSA hash %+v", ephemeralId))
	}
	return e, nil
}

func (m *MapImpl) getUsersByOffset(offset int64) ([]*User, error) {
	return m.usersByOffset[offset], nil
}

func (m *MapImpl) DeleteOldEphemerals(epoch int32) error {
	for i, e := range m.ephemeralsById {
		if e.Epoch < epoch {
			delete(m.allEphemerals, int(m.ephemeralsById[i].ID))
			delete(m.ephemeralsById, i)
		}
	}
	return nil
}

func (m *MapImpl) GetLatestEphemeral() (*Ephemeral, error) {
	e, ok := m.allEphemerals[m.ephIDSeq]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return e, nil
}
