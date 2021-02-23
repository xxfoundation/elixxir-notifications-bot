package notifications

import (
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"time"
)

func (nb *Impl) refresh(end time.Time, user *storage.User) {
	time.Sleep(time.Until(end))
	nb.ephemeralUpdates <- user
}

func (nb *Impl) updateEphemeralIds() {
	for true {
		u := <-nb.ephemeralUpdates
		err := nb.AddEphemeralID(u)
		if err != nil {
			jww.ERROR.Println(fmt.Sprintf("Error adding new ephemeral ID: %+v", err))
		}
	}
}

func (nb *Impl) StartEphemeralTracking() error {
	// Start update thread for ephemeral IDs
	go nb.updateEphemeralIds()

	// Get all users in DB and add them to the eid map
	users, err := nb.Storage.GetAllUsers()
	if err != nil {
		return err
	}
	for _, u := range users {
		err = nb.AddEphemeralID(u)
		if err != nil {
			return errors.WithMessage(err, "Failed to add ephemeral ID for user")
		}
	}
	return nil
}

func (nb *Impl) AddEphemeralID(u *storage.User) error {
	eph, end, err := getUpdatedEphemeral(u)
	if err != nil {
		return errors.WithMessage(err, "Failed to get ephemeral for user")
	}
	err = nb.Storage.UpsertEphemeral(eph)
	if err != nil {
		return errors.Wrap(err, "Failed to add user to ephemeral ID tracking")
	}
	go nb.refresh(end, u)
	return nil
}

func getUpdatedEphemeral(u *storage.User) (*storage.Ephemeral, time.Time, error) {
	id, _, end, err := ephemeral.GetIdFromIntermediary(u.IntermediaryId, 32, time.Now().UnixNano())
	if err != nil {
		return nil, time.Time{}, errors.WithMessage(err, fmt.Sprintf("Failed to get ephemeral ID for IID %+v", u.IntermediaryId))
	}
	return &storage.Ephemeral{
		TransmissionRSAHash: u.TransmissionRSAHash,
		EphemeralId:         id[:],
		Epoch:               end.UnixNano(),
		User:                *u,
	}, end, nil
}
