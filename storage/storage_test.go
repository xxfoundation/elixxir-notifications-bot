package storage

import (
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
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
	u, err := s.AddUser(iid, []byte("transmissionrsa"), []byte("signature"), "token")
	if err != nil {
		t.Errorf("Failed to add user: %+v", err)
	}
	err = s.AddLatestEphemeral(u, 5)
	if err != nil {
		t.Errorf("Failed to add latest ephemeral: %+v", err)
	}
}

func TestStorage_UpdateEphemeralsForOffset(t *testing.T) {
	_, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to create new storage object: %+v", err)
	}
}
