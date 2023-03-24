// //////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//
//	//
//
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
// //////////////////////////////////////////////////////////////////////////////
package storage

import (
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"testing"
)

func TestStorage_RegisterForNotifications(t *testing.T) {
	s, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to create new storage object: %+v", err)
	}
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to create iid: %+v", err)
	}
	if err != nil {
		t.Errorf("Could not parse precanned time: %v", err.Error())
	}
	_, err = s.RegisterForNotifications(iid, []byte("transmissionrsa"), []byte("signature"), "token", 0, 8)
	if err != nil {
		t.Errorf("Failed to add user: %+v", err)
	}
}

func TestStorage_UnregisterForNotifications(t *testing.T) {
	s, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to create new storage object: %+v", err)
	}
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to create iid: %+v", err)
	}
	if err != nil {
		t.Errorf("Could not parse precanned time: %v", err.Error())
	}
	u, err := s.RegisterForNotifications(iid, []byte("transmissionrsa"), []byte("signature"), "token", 0, 8)
	if err != nil {
		t.Errorf("Failed to add user: %+v", err)
	}
	err = s.UnregisterForNotifications(u.TransmissionRSA, [][]byte{iid}, []string{"token"})
	if err != nil {
		t.Errorf("Failed to delete user: %+v", err)
	}
}

func TestStorage_AddLatestEphemeral(t *testing.T) {
	s, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to create new storage object: %+v", err)
	}
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to create iid: %+v", err)
	}
	if err != nil {
		t.Errorf("Could not parse precanned time: %v", err.Error())
	}
	ident := &Identity{
		IntermediaryId: iid,
		OffsetNum:      ephemeral.GetOffsetNum(ephemeral.GetOffset(iid)),
	}
	err = s.insertIdentity(ident)
	if err != nil {
		t.Errorf("Failed to add user: %+v", err)
	}
	_, err = s.AddLatestEphemeral(ident, 5, 16)
	if err != nil {
		t.Errorf("Failed to add latest ephemeral: %+v", err)
	}
}

func TestStorage_AddEphemeralsForOffset(t *testing.T) {
	_, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to create new storage object: %+v", err)
	}
}
