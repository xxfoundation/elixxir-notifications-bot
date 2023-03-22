package storage

import (
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"testing"
	"time"
)

func TestDatabase(t *testing.T) {
	s, err := NewStorage("jonahhusson", "", "cmix", "0.0.0.0", "5432")
	if err != nil {
		t.Fatal(err)
	}

	addressSpace := uint8(16)

	id1 := id.NewIdFromString("mr_peanutbutter", id.User, t)
	iid1, err := ephemeral.GetIntermediaryId(id1)
	if err != nil {
		t.Fatal(err)
	}
	trsa := []byte("trsa")
	sig1 := []byte("First signature")
	token1 := "apnstoken01"
	token2 := "fcm:token02"

	id2 := id.NewIdFromString("lex_luthor", id.User, t)
	iid2, err := ephemeral.GetIntermediaryId(id2)
	if err != nil {
		t.Fatal(err)
	}

	_, epoch := ephemeral.HandleQuantization(time.Now())

	u1, err := s.RegisterForNotifications(iid1, trsa, sig1, token1, epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	u2, err := s.RegisterForNotifications(iid1, trsa, sig1, token2, epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.RegisterForNotifications(iid2, trsa, sig1, token2, epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.GetUser(u1.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(u1)
	t.Log(u2)

	err = s.UnregisterForNotifications(trsa, [][]byte{}, []string{token1})
	if err != nil {
		t.Fatal(err)
	}

	gtnList, err := s.GetToNotify([]int64{10806, 7970})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(gtnList)
}
