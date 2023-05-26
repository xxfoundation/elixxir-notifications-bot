package storage

import (
	"fmt"
	"gitlab.com/elixxir/notifications-bot/constants"
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

	// Register user 1 with token 1 and identity 1
	_, err = s.RegisterForNotifications(iid1, trsa, token1, constants.MessengerIOS.String(), epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	// User1:
	//  Tokens: 1
	//  Identities: 1
	gtnList, err := s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	if len(gtnList) != 1 {
		t.Fatalf("Got wrong gtnlist: %+v", gtnList)
	}

	// Register user 2 with token 4 and identity 1
	_, err = s.RegisterForNotifications(iid1, trsa2, token4, constants.MessengerAndroid.String(), epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	// User1:
	//  Tokens: 1
	//  Identities: 1
	// User2:
	//  Tokens: 4
	//  Identities: 1
	gtnList, err = s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	if len(gtnList) != 2 {
		t.Fatal("Got wrong gtnlist")
	}

	// Call identitcal registration on user 1 (no change)
	_, err = s.RegisterForNotifications(iid1, trsa, token1, constants.MessengerIOS.String(), epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	// User1:
	//  Tokens: 1
	//  Identities: 1
	// User2:
	//  Tokens: 4
	//  Identities: 1
	gtnList, err = s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(gtnList)
	if len(gtnList) != 2 {
		t.Fatal("Got wrong gtnlist")
	}

	// Register user 1 with identity 2 (still on token 1)
	_, err = s.RegisterForNotifications(iid2, trsa, token1, constants.MessengerIOS.String(), epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	// User1:
	//  Tokens: 1
	//  Identities: 1, 2
	// User2:
	//  Tokens: 4
	//  Identities: 1
	gtnList, err = s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(gtnList)
	if len(gtnList) != 3 {
		t.Fatal("Got wrong gtnlist")
	}

	// Register user 1 with token 2
	_, err = s.RegisterForNotifications(iid2, trsa, token2, constants.MessengerAndroid.String(), epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	// User1:
	//  Tokens: 1, 2
	//  Identities: 1, 2
	// User2:
	//  Tokens: 4
	//  Identities: 1
	gtnList, err = s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	if len(gtnList) != 5 {
		t.Fatal("Got wrong gtnlist")
	}

	// Register user 1 with token3 and identity3
	_, err = s.RegisterForNotifications(iid3, trsa, token3, constants.MessengerIOS.String(), epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	// User1:
	//  Tokens: 1, 2, 3
	//  Identities: 1, 2, 3
	// User2:
	//  Tokens: 4
	//  Identities: 1
	gtnList, err = s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	if len(gtnList) != 10 {
		t.Fatalf("Got wrong gtnlist: %+v", gtnList)
	}

	// Register user 2 with identity 4
	_, err = s.RegisterForNotifications(iid4, trsa2, token4, constants.MessengerAndroid.String(), epoch, addressSpace)
	if err != nil {
		t.Fatal(err)
	}

	// User1:
	//  Tokens: 1, 2, 3
	//  Identities: 1, 2, 3
	// User2:
	//  Tokens: 4
	//  Identities: 1, 4
	gtnList, err = s.GetToNotify(toNotify)
	if err != nil {
		t.Fatal(err)
	}
	if len(gtnList) != 11 {
		t.Fatalf("Got wrong gtnlist: %+v", gtnList)
	}

	gtnList, err = s.GetToNotify([]int64{toNotify[0]})
	if len(gtnList) != 4 {
		fmt.Println(toNotify[0])
		t.Log(len(gtnList))
		t.Fatalf("Got wrong gtnlist: %+v", gtnList)
	}
}
