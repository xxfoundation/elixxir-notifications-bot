package notifications

import (
	"errors"
	"fmt"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gorm.io/gorm"
	"time"
)

const offsetPhase = ephemeral.Period / ephemeral.NumOffsets
const creationLead = 5 * time.Minute
const deletionDelay = -(time.Duration(ephemeral.Period) + creationLead)

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
	lastEph, err := nb.Storage.GetLatestEphemeral()
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		jww.WARN.Printf("Failed to get latest ephemeral (no records found): %+v", err)
		lastEpochTime = time.Now().Add(-time.Duration(ephemeral.Period))
	} else if err != nil {
		jww.FATAL.Panicf("Database lookup for latest ephemeral failed: %+v", err)
	} else {
		lastEpochTime = time.Unix(0, int64(lastEph.Epoch)*offsetPhase) // Epoch time of last ephemeral ID
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
	time.Sleep(time.Until(nextTrigger))
}

func (nb *Impl) addEphemerals(start time.Time) {
	currentOffset, epoch := ephemeral.HandleQuantization(start)
	def := nb.inst.GetFullNdf()
	err := nb.Storage.AddEphemeralsForOffset(currentOffset, epoch, uint(def.Get().AddressSpaceSize))
	if err != nil {
		jww.WARN.Printf("failed to update ephemerals: %+v", err)
	}
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
