////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package testutil

import "gitlab.com/elixxir/notifications-bot/storage"

type MockStorage struct{}

func (ms MockStorage) GetUser(userId []byte) (*storage.User, error) {
	return &storage.User{
		IntermediaryId: []byte("test"),
		Token:          "test",
	}, nil
}

// Delete User from backend by primary key
func (ms MockStorage) DeleteUser(userId []byte) error {
	return nil
}

// Insert or Update User into backend
func (ms MockStorage) UpsertUser(user *storage.User) error {
	return nil
}
