package notifications

import (
	"fmt"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"strconv"
	"time"
)

const offsetPhase = ephemeral.Period / ephemeral.NumOffsets
const creationLead = 5 * time.Minute
const deletionDelay = -(time.Duration(ephemeral.Period) + creationLead)
const ephemeralStateKey = "lastEphemeralOffset"

// EphIdCreator runs as a thread to track ephemeral IDs for users who registered to receive push notifications
func (nb *Impl) EphIdCreator() {
	nb.initCreator()
	ticker := time.NewTicker(time.Duration(offsetPhase))
	go nb.addEphemerals(time.Now().Add(creationLead))
	//handle all future epochs
	for true {
		<-ticker.C
		go nb.addEphemerals(time.Now().Add(creationLead))
	}
}

func (nb *Impl) initCreator() {
	// Retrieve most recent ephemeral from storage
	var lastEpochTime time.Time
	lastEphEpoch, err := nb.Storage.GetStateValue(ephemeralStateKey)
	if err != nil {
		jww.WARN.Printf("Failed to get latest ephemeral: %+v", err)
		lastEpochTime = time.Now().Add(-time.Duration(ephemeral.Period))
	} else {
		lastEpochInt, err := strconv.Atoi(lastEphEpoch)
		if err != nil {
			jww.FATAL.Printf("Failed to convert last epoch to int: %+v", err)
		}
		lastEpochTime = time.Unix(0, int64(lastEpochInt)*offsetPhase) // Epoch time of last ephemeral ID
		// If the last epoch is further back than the ephemeral ID period, only go back one period for generation
		if lastEpochTime.Before(time.Now().Add(-time.Duration(ephemeral.Period))) {
			lastEpochTime = time.Now().Add(-time.Duration(ephemeral.Period))
		}
	}
	// Add all missed ephemeral IDs
	// increment by offsetPhase up to 5 minutes from now making ephemerals
	for endTime := time.Now().Add(creationLead); lastEpochTime.Before(endTime); lastEpochTime = lastEpochTime.Add(time.Duration(offsetPhase)) {
		nb.addEphemerals(lastEpochTime)
	}
	// handle the next epoch
	_, epoch := ephemeral.HandleQuantization(lastEpochTime)
	nextTrigger := time.Unix(0, int64(epoch)*offsetPhase)
	jww.INFO.Println(fmt.Sprintf("Sleeping until next trigger at %+v", nextTrigger))

	orphaned, err := nb.Storage.GetOrphanedUsers()
	if err != nil {
		jww.FATAL.Panicf("Failed to retrieve orphaned users: %+v", err)
	}
	for _, u := range orphaned {
		_, err := nb.Storage.AddLatestEphemeral(u, epoch, uint(nb.inst.GetPartialNdf().Get().AddressSpace[0].Size))
		if err != nil {
			jww.WARN.Printf("Failed to add latest ephemeral for orphaned user %+v: %+v", u.TransmissionRSAHash, err)
		}
	}

	time.Sleep(time.Until(nextTrigger))
}

func (nb *Impl) addEphemerals(start time.Time) {
	currentOffset, epoch := ephemeral.HandleQuantization(start)
	def := nb.inst.GetPartialNdf()
	// FIXME: Does the address space need more logic here?
	err := nb.Storage.AddEphemeralsForOffset(currentOffset, epoch, uint(def.Get().AddressSpace[0].Size), start)
	if err != nil {
		jww.WARN.Printf("failed to update ephemerals: %+v", err)
	}
	err = nb.Storage.UpsertState(&storage.State{
		Key:   ephemeralStateKey,
		Value: strconv.Itoa(int(epoch)),
	})
}

func (nb *Impl) EphIdDeleter() {
	nb.initDeleter()
	ticker := time.NewTicker(time.Duration(offsetPhase))
	//handle all future epochs
	for true {
		<-ticker.C
		go nb.deleteEphemerals(time.Now().Add(deletionDelay))
	}
}

func (nb *Impl) initDeleter() {
	//handle the next epoch
	_, epoch := ephemeral.HandleQuantization(time.Now())
	nextTrigger := time.Unix(0, int64(epoch+1)*offsetPhase)
	// Bring us into phase with ephemeral identity creation
	time.Sleep(time.Until(nextTrigger))
	go nb.deleteEphemerals(time.Now().Add(deletionDelay))
}

func (nb *Impl) deleteEphemerals(start time.Time) {
	_, currentEpoch := ephemeral.HandleQuantization(start)
	err := nb.Storage.DeleteOldEphemerals(currentEpoch)
	if err != nil {
		jww.WARN.Printf("failed to delete ephemerals: %+v", err)
	}
}
