package storage

import (
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"testing"
	"time"
)

func TestDatabase(t *testing.T) {
	s, err := NewStorage("", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	addressSpace := uint8(16)

	var toNotify []int64
	_, epoch := ephemeral.HandleQuantization(time.Now())

	id1 := id.NewIdFromString("mr_peanutbutter", id.User, t)
	iid1, err := ephemeral.GetIntermediaryId(id1)
	if err != nil {
		t.Fatal(err)
	}
	eph, _, _, err := ephemeral.GetId(id1, uint(addressSpace), time.Now().UnixNano())
	if err != nil {
		t.Fatal(err)
	}
	toNotify = append(toNotify, eph.Int64())

	trsa := []byte("trsa")
	sig1 := []byte("First signature")
	token1 := "apnstoken01"
	token2 := "fcm:token02"
	token3 := "apnstoken03"

	id2 := id.NewIdFromString("lex_luthor", id.User, t)
	iid2, err := ephemeral.GetIntermediaryId(id2)
	if err != nil {
		t.Fatal(err)
	}
	eph, _, _, err = ephemeral.GetId(id2, uint(addressSpace), time.Now().UnixNano())
	if err != nil {
		t.Fatal(err)
	}
	toNotify = append(toNotify, eph.Int64())

	id3 := id.NewIdFromString("spooderman", id.User, t)
	iid3, err := ephemeral.GetIntermediaryId(id3)
	if err != nil {
		t.Fatal(err)
	}
	eph, _, _, err = ephemeral.GetId(id3, uint(addressSpace), time.Now().UnixNano())
	if err != nil {
		t.Fatal(err)
	}
	toNotify = append(toNotify, eph.Int64())

	trsa2 := []byte("trsa2")
	id4 := id.NewIdFromString("mr. morales", id.User, t)
	iid4, err := ephemeral.GetIntermediaryId(id4)
	if err != nil {
		t.Fatal(err)
	}
	eph, _, _, err = ephemeral.GetId(id4, uint(addressSpace), time.Now().UnixNano())
	if err != nil {
		t.Fatal(err)
	}
	toNotify = append(toNotify, eph.Int64())
	token4 := "fcm:token04"

	// Register new user, token & identity
	_, err = s.RegisterForNotifications(iid1, trsa, sig1, token1, epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}
	gtnList, err := s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	if len(gtnList) != 1 {
		t.Fatal("Got wrong gtnlist")
	}

	// Second user, register for same identity
	_, err = s.RegisterForNotifications(iid1, trsa2, sig1, token4, epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	gtnList, err = s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	if len(gtnList) != 2 {
		t.Fatal("Got wrong gtnlist")
	}

	// Call same register for first user
	_, err = s.RegisterForNotifications(iid1, trsa, sig1, token1, epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	gtnList, err = s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(gtnList)
	if len(gtnList) != 2 {
		t.Fatal("Got wrong gtnlist")
	}

	// Add new identity
	_, err = s.RegisterForNotifications(iid2, trsa, sig1, token1, epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	gtnList, err = s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(gtnList)
	if len(gtnList) != 2 {
		t.Fatal("Got wrong gtnlist")
	}

	// Add new token
	_, err = s.RegisterForNotifications(iid2, trsa, sig1, token2, epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	gtnList, err = s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	if len(gtnList) != 3 {
		t.Fatal("Got wrong gtnlist")
	}

	// Add new token & identity
	_, err = s.RegisterForNotifications(iid3, trsa, sig1, token3, epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	gtnList, err = s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	if len(gtnList) != 4 {
		t.Fatal("Got wrong gtnlist")
	}

	// Second user with new identity
	_, err = s.RegisterForNotifications(iid4, trsa2, sig1, token4, epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	gtnList, err = s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	if len(gtnList) != 4 {
		t.Fatal("Got wrong gtnlist")
	}

}
