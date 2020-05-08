package testutil

import (
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/elixxir/primitives/id"
)

type MockStorage struct{}

func (ms MockStorage) GetUser(userId *id.ID) (*storage.User, error) {
	return &storage.User{
		Id:    "test",
		Token: "test",
	}, nil
}

// Delete User from backend by primary key
func (ms MockStorage) DeleteUser(userId *id.ID) error {
	return nil
}

// Insert or Update User into backend
func (ms MockStorage) UpsertUser(user *storage.User) error {
	return nil
}
