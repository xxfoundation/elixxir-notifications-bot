////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package notifications

import (
	"fmt"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/notifications-bot/constants"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	jww.SetStdoutThreshold(jww.LevelTrace)
	connect.TestingOnlyDisableTLS = true
	os.Exit(m.Run())
}

func TestImpl_InitDeleter(t *testing.T) {
	s, err := storage.NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to init storage: %+v", err)
	}
	impl := &Impl{
		Storage: s,
	}
	uid := id.NewIdFromString("deleter_zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to get intermediary ephemeral id: %+v", err)
	}
	if err != nil {
		t.Fatalf("Could not parse precanned time: %v", err.Error())
	}
	_, epoch := ephemeral.HandleQuantization(time.Now().Add(-30 * time.Hour))
	_, err = s.RegisterForNotifications(iid, []byte("trsa"), "token", constants.MessengerIOS.String(), epoch, 16)
	if err != nil {
		t.Errorf("Failed to add user to storage: %+v", err)
	}

	e, err := s.GetLatestEphemeral()
	elist, err := s.GetEphemeral(e.EphemeralId)
	if err != nil {
		t.Errorf("Failed to get latest ephemeral for user: %+v", err)
	}
	if elist == nil {
		t.Error("Did not receive ephemeral for user")
	}
	impl.initDeleter()
	time.Sleep(time.Second * 10)
	elist, err = s.GetEphemeral(e.EphemeralId)
	if err == nil {
		t.Errorf("Ephemeral should have been deleted, did not receive error: %+v", e)
	}
}

func TestImpl_InitCreator(t *testing.T) {
	s, err := storage.NewStorage("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to init storage: %+v", err)
		t.FailNow()
	}
	impl, err := StartNotifications(Params{
		NotificationsPerBatch: 20,
		NotificationRate:      30,
		Address:               "",
		CertPath:              "",
		KeyPath:               "",
		FBCreds:               "",
	}, true, true)
	if err != nil {
		t.Errorf("Failed to create impl: %+v", err)
		t.FailNow()
	}
	impl.Storage = s
	uid := id.NewIdFromString("zezima", id.User, t)
	iid, err := ephemeral.GetIntermediaryId(uid)
	if err != nil {
		t.Errorf("Failed to get intermediary ephemeral id: %+v", err)
	}
	if err != nil {
		t.Errorf("Could not parse precanned time: %v", err.Error())
	}
	_, epoch := ephemeral.HandleQuantization(time.Now())
	u, err := s.RegisterForNotifications(iid, []byte("trsa"), "token", constants.MessengerIOS.String(), epoch, 16)
	if err != nil {
		t.Errorf("Failed to add user to storage: %+v", err)
	}

	u, err = s.GetUser(u.TransmissionRSAHash)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(u.Identities[0].OffsetNum)
	impl.initCreator()
	e, err := s.GetLatestEphemeral()
	if err != nil {
		t.Errorf("Failed to get latest ephemeral: %+v", err)
	}
	if e == nil {
		t.Error("Did not receive ephemeral for user")
	}
}
