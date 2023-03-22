////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Handles the implementation of the map backend
package storage

import (
	"encoding/base64"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gorm.io/gorm"
	"sync"
)

type MapImplV1 struct {
	mut              sync.Mutex
	states           map[string]string
	usersById        map[string]*User
	usersByRsaHash   map[string]*User
	usersByOffset    map[int64][]*User
	allUsers         []*User
	allEphemerals    map[int]*Ephemeral
	ephemeralsById   map[int64][]*Ephemeral
	ephemeralsByUser map[string]map[int64]*Ephemeral
	ephIDSeq         int
}

// Struct implementing the Database Interface with an underlying Map
type MapImpl struct {
	mux    sync.Mutex
	states map[string]State

	tokens       map[string]Token
	tokensByUser map[string]map[string]Token

	users map[string]User

	userIdentities map[string]map[string]bool
	identityUsers  map[string]map[string]bool

	identities         map[string]Identity
	identitiesByOffset map[int64][]*Identity

	ephemerals           map[int]Ephemeral
	ephemeralsById       map[int64][]*Ephemeral
	ephemeralsByIdentity map[string][]*Ephemeral
	ephemeralSequence    int
	latest               *Ephemeral
}

// Inserts the given State into Storage if it does not exist
// Or updates the Database State if its value does not match the given State
func (m *MapImpl) UpsertState(state *State) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	m.states[state.Key] = *state
	return nil
}

// Returns a State's value from Storage with the given key
// Or an error if a matching State does not exist
func (m *MapImpl) GetStateValue(key string) (string, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	if val, ok := m.states[key]; ok {
		return val.Value, nil
	}

	// NOTE: Other code depends on this error string
	return "", errors.Errorf("Unable to locate state for key %s", key)
}

func (m *MapImpl) DeleteToken(token string) error {
	toDelete, ok := m.tokens[token]
	if !ok {
		jww.WARN.Printf("Could not find token %s to delete", token)
		return nil
	}
	userId := base64.StdEncoding.EncodeToString(toDelete.TransmissionRSAHash)
	delete(m.tokensByUser[userId], token)

	delete(m.tokens, token)
	return nil
}

func (m *MapImpl) insertUser(user *User) error {
	userId := base64.StdEncoding.EncodeToString(user.TransmissionRSAHash)
	m.users[userId] = *user

	m.userIdentities[userId] = map[string]bool{}

	for _, ident := range user.Identities {
		identityId := base64.StdEncoding.EncodeToString(ident.IntermediaryId)
		m.userIdentities[userId][identityId] = true
		m.identityUsers[identityId][userId] = true
	}

	m.tokensByUser[userId] = map[string]Token{}
	for _, token := range user.Tokens {
		m.tokensByUser[userId][token.Token] = token
		m.tokens[token.Token] = token
	}
	return nil
}
func (m *MapImpl) GetUser(transmissionRsaHash []byte) (*User, error) {
	userId := base64.StdEncoding.EncodeToString(transmissionRsaHash)
	u, ok := m.users[userId]
	if !ok {
		return nil, errors.Wrap(gorm.ErrRecordNotFound, "user could not be found")
	}
	return &u, nil
}

func (m *MapImpl) deleteUser(transmissionRsaHash []byte) error {
	userId := base64.StdEncoding.EncodeToString(transmissionRsaHash)
	delete(m.users, userId)
	return nil
}

func (m *MapImpl) GetAllUsers() ([]*User, error) {
	var userList []*User
	for _, u := range m.users {
		userList = append(userList, &u)
	}

	return userList, nil
}

func (m *MapImpl) getIdentity(iid []byte) (*Identity, error) {
	identityId := base64.StdEncoding.EncodeToString(iid)
	ident, ok := m.identities[identityId]
	if !ok {
		return nil, errors.Wrap(gorm.ErrRecordNotFound, "identity could not be found")
	}
	return &ident, nil
}
func (m *MapImpl) insertIdentity(identity *Identity) error {
	identityId := base64.StdEncoding.EncodeToString(identity.IntermediaryId)
	m.identities[identityId] = *identity
	_, ok := m.identitiesByOffset[identity.OffsetNum]
	if !ok {
		m.identitiesByOffset[identity.OffsetNum] = []*Identity{}
	}
	m.identitiesByOffset[identity.OffsetNum] = append(m.identitiesByOffset[identity.OffsetNum], identity)
	m.identityUsers[identityId] = map[string]bool{}
	return nil
}
func (m *MapImpl) getIdentitiesByOffset(offset int64) ([]*Identity, error) {
	idents, ok := m.identitiesByOffset[offset]
	if !ok {
		return nil, errors.Wrap(gorm.ErrRecordNotFound, "identities could not be found")
	}
	return idents, nil
}
func (m *MapImpl) GetOrphanedIdentities() ([]*Identity, error) {
	var orphaned []*Identity
	for k, v := range m.identities {
		ephs, ok := m.ephemeralsByIdentity[k]
		if ok && len(ephs) == 0 {
			orphaned = append(orphaned, &v)
		}
	}
	return orphaned, nil
}

func (m *MapImpl) insertEphemeral(ephemeral *Ephemeral) error {
	ephemeral.ID = uint(m.ephemeralSequence)
	m.ephemerals[m.ephemeralSequence] = *ephemeral
	_, ok := m.ephemeralsById[ephemeral.EphemeralId]
	if !ok {
		m.ephemeralsById[ephemeral.EphemeralId] = []*Ephemeral{}
	}
	m.ephemeralsById[ephemeral.EphemeralId] = append(m.ephemeralsById[ephemeral.EphemeralId], ephemeral)

	identityId := base64.StdEncoding.EncodeToString(ephemeral.IntermediaryId)
	_, ok = m.ephemeralsByIdentity[identityId]
	if !ok {
		m.ephemeralsByIdentity[identityId] = []*Ephemeral{}
	}
	m.ephemeralsByIdentity[identityId] = append(m.ephemeralsByIdentity[identityId], ephemeral)
	m.ephemeralSequence += 1
	m.latest = ephemeral
	return nil
}

func (m *MapImpl) GetEphemeral(ephemeralId int64) ([]*Ephemeral, error) {
	ephs, ok := m.ephemeralsById[ephemeralId]
	if !ok || len(ephs) == 0 {
		return nil, errors.Wrap(gorm.ErrRecordNotFound, "ephemerals could not be found")
	}
	return ephs, nil
}
func (m *MapImpl) GetLatestEphemeral() (*Ephemeral, error) {
	if m.latest == nil {
		return nil, errors.Wrap(gorm.ErrRecordNotFound, "No latest ephemeral")
	}
	return m.latest, nil
}
func (m *MapImpl) DeleteOldEphemerals(currentEpoch int32) error {
	for _, v := range m.ephemerals {
		if v.Epoch < currentEpoch {
			identityId := base64.StdEncoding.EncodeToString(v.IntermediaryId)
			for i := range m.ephemeralsByIdentity[identityId] {
				if m.ephemeralsByIdentity[identityId][i].ID == v.ID {
					m.ephemeralsByIdentity[identityId] = append(m.ephemeralsByIdentity[identityId][:i], m.ephemeralsByIdentity[identityId][i+1:]...)
					break
				}
			}
			for i := range m.ephemeralsById[v.EphemeralId] {
				if m.ephemeralsById[v.EphemeralId][i].ID == v.ID {
					m.ephemeralsById[v.EphemeralId] = append(m.ephemeralsById[v.EphemeralId][:i], m.ephemeralsById[v.EphemeralId][i+1:]...)
					break
				}
			}
			delete(m.ephemerals, int(v.ID))
		}
	}
	return nil
}
func (m *MapImpl) GetToNotify(ephemeralIds []int64) ([]GTNResult, error) {
	foundTokens := map[string]interface{}{}
	res := []GTNResult{}
	for _, eid := range ephemeralIds {
		ephsToNotify, ok := m.ephemeralsById[eid]
		if !ok {
			continue
		}
		for _, eph := range ephsToNotify {
			identityId := base64.StdEncoding.EncodeToString(eph.IntermediaryId)
			userIDs, ok := m.identityUsers[identityId]
			if !ok {
				continue
			}
			for userId, _ := range userIDs {
				tokens, ok := m.tokensByUser[userId]
				if !ok {
					continue
				}
				for _, token := range tokens {
					if _, ok := foundTokens[token.Token]; ok {
						continue
					}

					foundTokens[token.Token] = struct{}{}
					res = append(res, GTNResult{
						Token:               token.Token,
						TransmissionRSAHash: m.users[userId].TransmissionRSAHash,
						EphemeralId:         eph.EphemeralId,
					})
				}
			}
		}
	}
	return res, nil
}

func (m *MapImpl) unregisterIdentities(u *User, iids []Identity) error {
	userId := base64.StdEncoding.EncodeToString(u.TransmissionRSAHash)
	for _, ident := range iids {
		identityId := base64.StdEncoding.EncodeToString(ident.IntermediaryId)
		delete(m.userIdentities[userId], identityId)
	}
	return nil
}

func (m *MapImpl) unregisterTokens(u *User, tokens []Token) error {
	userId := base64.StdEncoding.EncodeToString(u.TransmissionRSAHash)
	for _, token := range tokens {
		delete(m.tokensByUser[userId], token.Token)
	}
	return nil
}

func (m *MapImpl) registerForNotifications(u *User, identity Identity, token string) error {
	identityId := base64.StdEncoding.EncodeToString(identity.IntermediaryId)
	userId := base64.StdEncoding.EncodeToString(u.TransmissionRSAHash)
	m.userIdentities[userId][identityId] = true
	m.identityUsers[identityId][userId] = true

	t := Token{
		Token:               token,
		TransmissionRSAHash: u.TransmissionRSA,
	}
	m.tokensByUser[userId][token] = t
	m.tokens[token] = t
	return nil
}

func (m *MapImpl) LegacyUnregister(iid []byte) error {
	return errors.New("Legacy unregister not implemented in map backend")
}
