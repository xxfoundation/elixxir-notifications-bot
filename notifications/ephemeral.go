package notifications

import (
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"sync"
	"time"
)

func (nb *Impl) TrackOffset(offset int64) {
	if _, ok := nb.offsets[offset]; !ok {
		nb.offsets[offset] = &sync.Once{}
	}
	nb.offsets[offset].Do(func() {
		go nb.offsetTracker(offset)
	})
}

func (nb *Impl) offsetTracker(offset int64) {
	for true {
		_, end, _ := ephemeral.GetOffsetBounds(offset, time.Now().UnixNano())
		time.Sleep(time.Until(end) - time.Minute)
		err := nb.Storage.UpdateEphemeralsForOffset(offset, end)
		if err != nil {
			jww.WARN.Printf("failed to update ephemerals: %+v", err)
		}
		time.Sleep(time.Minute)
		err = nb.Storage.DeleteOldEphemerals(offset)
		if err != nil {
			jww.WARN.Printf("Failed to delete old ephemerals: %+v", err)
		}
	}
}
