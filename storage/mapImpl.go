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
	for i, u := range m.usersByOffset[user.Offset] {
		if bytes.Compare(transmissionRsaHash, u.TransmissionRSAHash) == 0 {
			m.usersByOffset[user.Offset] = append(m.usersByOffset[user.Offset][:i],
				m.usersByOffset[user.Offset][i+1:]...)
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
	for i, u := range m.usersByOffset[user.Offset] {
		if string(user.TransmissionRSAHash) == string(u.TransmissionRSAHash) {
			found = true
			m.usersByOffset[user.Offset][i] = user
			break
		}
	}
	if !found {
		m.usersByOffset[user.Offset] = append(m.usersByOffset[user.Offset], user)
	}
	m.allUsers = append(m.allUsers, user)

	return nil
}

func (m *MapImpl) GetAllUsers() ([]*User, error) {
	return m.allUsers, nil
}

func (m *MapImpl) upsertEphemeral(ephemeral *Ephemeral) error {
	ephemeral.ID = m.ephIDSeq
	m.ephIDSeq++
	m.ephemeralsByUser[string(ephemeral.TransmissionRSAHash)] = append(m.ephemeralsByUser[string(ephemeral.TransmissionRSAHash)], ephemeral)
	m.allEphemerals[ephemeral.ID] = ephemeral
	return nil
}

func (m *MapImpl) GetEphemeral(transmissionRSAHash []byte) (*Ephemeral, error) {
	elist, ok := m.ephemeralsByUser[string(transmissionRSAHash)]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Could not find ephemeral with transmission RSA hash %+v", transmissionRSAHash))
	}
	return elist[0], nil
}

func (m *MapImpl) getUsersByOffset(offset int64) ([]*User, error) {
	return m.usersByOffset[offset], nil
}

func (m *MapImpl) DeleteOldEphemerals(epoch int32) error {
	for k, es := range m.ephemeralsByUser {
		for i, e := range es {
			if e.Epoch < epoch {
				delete(m.allEphemerals, m.ephemeralsByUser[k][i].ID)
				m.ephemeralsByUser[k] = append(m.ephemeralsByUser[k][:i], m.ephemeralsByUser[k][i+1:]...)
			}
		}
	}
	return nil
}

func (m *MapImpl) GetLatestEphemeral() (*Ephemeral, error) {
	cur := m.ephIDSeq
	var res *Ephemeral
	var ok bool
	for res, ok = m.allEphemerals[cur]; !ok; {
		cur++
	}
	return res, nil
}
