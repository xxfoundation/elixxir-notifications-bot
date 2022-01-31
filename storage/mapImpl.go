////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the implementation of the map backend

package storage

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gorm.io/gorm"
)

func (m *MapImpl) GetToNotify(ephemeralIds []int64) ([]GTNResult, error) {
	var results []GTNResult
	for _, eid := range ephemeralIds {
		for _, eph := range m.ephemeralsById[eid] {
			u := m.usersByRsaHash[string(eph.TransmissionRSAHash)]
			results = append(results, GTNResult{
				EphemeralId:          eid,
				TransmissionRSAHash:  u.TransmissionRSA,
				Token:                u.Token,
				NotificationProvider: u.NotificationProvider,
			})
		}
	}
	return results, nil
}

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
	m.ephemeralsByUser[string(user.TransmissionRSAHash)] = map[int64]*Ephemeral{} // init user's ephemeral map
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

func (m *MapImpl) GetOrphanedUsers() ([]*User, error) {
	var res []*User
	for _, u := range m.allUsers {
		if len(m.ephemeralsByUser[string(u.TransmissionRSAHash)]) < 1 {
			res = append(res, u)
		}
	}
	return res, nil
}

func (m *MapImpl) insertEphemeral(ephemeral *Ephemeral) error {
	m.ephIDSeq++
	ephemeral.ID = uint(m.ephIDSeq)
	m.ephemeralsById[ephemeral.EphemeralId] = append(m.ephemeralsById[ephemeral.EphemeralId], ephemeral)
	m.allEphemerals[int(ephemeral.ID)] = ephemeral
	m.ephemeralsByUser[string(ephemeral.TransmissionRSAHash)][ephemeral.EphemeralId] = ephemeral
	return nil
}

func (m *MapImpl) GetEphemeral(ephemeralId int64) ([]*Ephemeral, error) {
	e, ok := m.ephemeralsById[ephemeralId]
	if !ok || len(e) < 1 {
		return nil, errors.New(fmt.Sprintf("Could not find ephemeral with transmission RSA hash %+v", ephemeralId))
	}
	return e, nil
}

func (m *MapImpl) getUsersByOffset(offset int64) ([]*User, error) {
	return m.usersByOffset[offset], nil
}

func (m *MapImpl) DeleteOldEphemerals(epoch int32) error {
	for i, elist := range m.ephemeralsById {
		if elist != nil {
			for j, e := range elist {
				if e.Epoch < epoch {
					delete(m.ephemeralsByUser[string(e.TransmissionRSAHash)], e.EphemeralId)
					delete(m.allEphemerals, int(m.ephemeralsById[i][j].ID))
					m.ephemeralsById[i] = append(m.ephemeralsById[i][:j],
						m.ephemeralsById[i][j+1:]...)
				}
			}
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

// Inserts the given State into Storage if it does not exist
// Or updates the Database State if its value does not match the given State
func (m *MapImpl) UpsertState(state *State) error {
	jww.TRACE.Printf("Attempting to insert State into Map: %+v", state)

	m.mut.Lock()
	defer m.mut.Unlock()

	m.states[state.Key] = state.Value
	return nil
}

// Returns a State's value from Storage with the given key
// Or an error if a matching State does not exist
func (m *MapImpl) GetStateValue(key string) (string, error) {
	m.mut.Lock()
	defer m.mut.Unlock()

	if val, ok := m.states[key]; ok {
		jww.TRACE.Printf("Obtained State from Map: %+v", val)
		return val, nil
	}

	// NOTE: Other code depends on this error string
	return "", errors.Errorf("Unable to locate state for key %s", key)
}
