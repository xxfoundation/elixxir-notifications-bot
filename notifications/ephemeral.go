package notifications

import (
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"sync"
	"time"
)

type EphemeralIds struct {
	ids        map[string]*EphemeralUser
	updateChan chan *EphemeralUser
	sync.RWMutex
}

type EphemeralUser struct {
	sync.RWMutex
	u       *storage.User
	id      ephemeral.Id
	tracked bool
}

func (e *EphemeralIds) AddOrUpdate(u *storage.User) error {
	e.Lock()
	defer e.Unlock()

	id, _, end, err := ephemeral.GetIdFromIntermediary(u.Id, 32, time.Now().UnixNano())
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("Failed to get ephemeral ID for IID %+v", u.Id))
	}

	stored, ok := e.ids[string(u.Id)]
	if !ok {
		e.ids[string(u.Id)] = &EphemeralUser{
			u:  u,
			id: id,
		}
		go e.refresh(end, e.ids[string(u.Id)])
	} else {
		stored.id = id
		stored.u = u
		if !stored.tracked {
			go e.refresh(end, stored)
		}
	}

	return nil
}

func (e *EphemeralIds) refresh(end time.Time, user *EphemeralUser) {
	time.Sleep(time.Until(end))
	user.tracked = false
	e.updateChan <- user
}

func (e *EphemeralIds) updateEphemeralIds() {
	for true {
		u := <-e.updateChan
		err := e.AddOrUpdate(u)
		if err != nil {
			jww.ERROR.Printf("Failed to update ephemeral ID for %+v", u)
		}
	}
}

func (nb *Impl) StartEphemeralTracking() error {
	// Start update thread for ephemeral IDs
	go nb.ephemeralIds.updateEphemeralIds()

	// Get all users in DB and add them to the eid map
	users, err := nb.Storage.GetAllUsers()
	if err != nil {
		return err
	}
	for _, u := range users {
		err := nb.ephemeralIds.AddOrUpdate(u)
		if err != nil {
			err = nb.Storage.DeleteUserByHash(u.TransmissionRSAHash)
			if err != nil {
				return errors.WithMessage(err, fmt.Sprintf("Failed to delete user with bad iid %+v: %+v", u, err))
			}
		}
	}
	return nil
}
