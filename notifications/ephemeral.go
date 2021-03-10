package notifications

import (
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"time"
)

const offsetPhase = ephemeral.Period / ephemeral.NumOffsets
const creationLead = 5 * time.Minute
const deletionDelay = (-5 * time.Minute) + time.Duration(ephemeral.Period)

func (nb *Impl) EphIdCreator() {
	//add the current in progress ephoch if it does not exist
	lastEph, err := nb.Storage.GetLatestEphemeral()
	if err != nil {
		jww.WARN.Printf("Failed to get latest ephemeral: %+v", err)
	}
	// Get unix time of last epoch
	lastEpochTime := time.Unix(0, int64(lastEph.Epoch)*offsetPhase)
	// Don't go back further than 24 hours
	if lastEpochTime.Before(time.Now().Add(-24 * time.Hour)) {
		lastEpochTime = time.Now().Add(-24 * time.Hour)
	}
	// increment by offsetPhase up to 5 minutes from now making ephemerals
	for lastEpochTime.Before(time.Now().Add(time.Minute * 5)) {
		lastEpochTime.Add(time.Duration(offsetPhase))
		go nb.AddEphemerals(lastEpochTime)
	}
	//handle the next epoch
	_, epoch := HandleQuantization(lastEpochTime)
	nextTrigger := time.Unix(0, int64(epoch+1)*offsetPhase)
	time.Sleep(time.Until(nextTrigger))
	ticker := time.NewTicker(time.Duration(offsetPhase))
	go nb.AddEphemerals(time.Now().Add(creationLead))
	//handle all future epochs
	for true {
		<-ticker.C
		go nb.AddEphemerals(time.Now().Add(creationLead))
	}
}
func (nb *Impl) AddEphemerals(start time.Time) {
	currentOffset, epoch := HandleQuantization(start)
	err := nb.Storage.AddEphemeralsForOffset(currentOffset, epoch)
	if err != nil {
		jww.WARN.Printf("failed to update ephemerals: %+v", err)
	}
}

func (nb *Impl) EphIdDeleter() {
	//add the current in progress ephoch if it does not exist
	go nb.DeleteEphemerals(time.Now().Add(deletionDelay))

	//handle the next epoch
	_, epoch := HandleQuantization(time.Now())
	nextTrigger := time.Unix(0, int64(epoch+1)*offsetPhase).Add(-deletionDelay)
	time.Sleep(time.Until(nextTrigger))
	ticker := time.NewTicker(time.Duration(offsetPhase))
	go nb.DeleteEphemerals(time.Now().Add(deletionDelay))
	//handle all future epochs
	for true {
		<-ticker.C
		go nb.DeleteEphemerals(time.Now().Add(deletionDelay))
	}
}

func (nb *Impl) DeleteEphemerals(start time.Time) {
	_, currentEpoch := HandleQuantization(start)
	err := nb.Storage.DeleteOldEphemerals(currentEpoch)
	if err != nil {
		jww.WARN.Printf("failed to update ephemerals: %+v", err)
	}
}
func HandleQuantization(start time.Time) (int64, int32) {
	currentOffset := (start.UnixNano() / offsetPhase) % ephemeral.NumOffsets
	epoch := start.UnixNano() / offsetPhase
	return currentOffset, int32(epoch)
}
