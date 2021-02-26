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

// Delete User from backend by primary key
func (m *MapImpl) deleteUser(transmissionRsaHash []byte) error {
	u, ok := m.usersByRsaHash[string(transmissionRsaHash)]
	if !ok {
		return nil
	}
	delete(m.usersByRsaHash, string(transmissionRsaHash))
	delete(m.usersById, string(u.IntermediaryId))
	for i, u := range m.allUsers {
		if bytes.Compare(transmissionRsaHash, u.TransmissionRSAHash) == 0 {
			m.allUsers = append(m.allUsers[:i], m.allUsers[i+1:]...)
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
		err := m.deleteUser(user.TransmissionRSAHash)
		if err != nil {
			return err
		}
	}
	// Insert new user
	m.usersByRsaHash[string(user.TransmissionRSAHash)] = user
	m.usersById[string(user.IntermediaryId)] = user
	m.allUsers = append(m.allUsers, user)

	return nil
}

func (m *MapImpl) GetAllUsers() ([]*User, error) {
	return m.allUsers, nil
}

func (m *MapImpl) upsertEphemeral(ephemeral *Ephemeral) error {
	m.ephemerals[string(ephemeral.TransmissionRSAHash)] = ephemeral
	return nil
}

func (m *MapImpl) GetEphemeral(transmissionRSAHash []byte) (*Ephemeral, error) {
	e, ok := m.ephemerals[string(transmissionRSAHash)]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Could not find ephemeral with transmission RSA hash %+v", transmissionRSAHash))
	}
	return e, nil
}

func (m *MapImpl) getUsersByOffset(offset int64) ([]*User, error) {
	return nil, nil
}

func (m *MapImpl) DeleteOldEphemerals(offset int64) error {
	return nil
}
