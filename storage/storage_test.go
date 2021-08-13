package storage

import (
	"git.xx.network/xx_network/primitives/id"
	"git.xx.network/xx_network/primitives/id/ephemeral"
	"testing"
)

func TestStorage_AddUser(t *testing.T) {
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
	_, err = s.AddUser(iid, []byte("transmissionrsa"), []byte("signature"), "token")
	if err != nil {
		t.Errorf("Failed to add user: %+v", err)
	}
}

func TestStorage_DeleteUser(t *testing.T) {
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
	u, err := s.AddUser(iid, []byte("transmissionrsa"), []byte("signature"), "token")
	if err != nil {
		t.Errorf("Failed to add user: %+v", err)
	}
	err = s.DeleteUser(u.TransmissionRSA)
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
	u, err := s.AddUser(iid, []byte("transmissionrsa"), []byte("signature"), "token")
	if err != nil {
		t.Errorf("Failed to add user: %+v", err)
	}
	_, err = s.AddLatestEphemeral(u, 5, 16)
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
