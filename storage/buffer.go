////////////////////////////////////////////////////////////////////////////////
// Copyright © 2021 Privategrity Corporation                                  //
//                                                                            //
// All rights reserved.                                                       //
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/primitives/notifications"
	"gitlab.com/xx_network/primitives/id"
	"sync"
	"sync/atomic"
)

// NotificationBuffer struct holds notifications received by the bot that have yet to be sent
// IT uses a sync.Map with a RWMutex to allow swapping maps for faster concurrent read/write access
// Stores lowest and highest rounds to provide ordering when queried
type NotificationBuffer struct {
	lock   sync.RWMutex
	gr     *uint64
	lr     *uint64
	bufMap *sync.Map
}

// NewNotificationBuffer is the constructor for NotificationBuffers.  Initializes maps & sets initial atomic values
func NewNotificationBuffer() *NotificationBuffer {
	gr, lr := uint64(0), uint64(0)

	nb := &NotificationBuffer{
		bufMap: &sync.Map{},
		gr:     &gr,
		lr:     &lr,
	}
	return nb
}

// Swap takes the write lock, replaces the current sync.Map and round values with new ones, and
// (outside the lock) sorts the old map into a map[ephID][]*notifications.Data, where each ephID list is sorted by RID
// NOTE THAT ANY UNSENT NOTIFICATIONS FROM SWAP MUST BE RE-ADDED TO THE BUFFER
func (bnm *NotificationBuffer) Swap() map[int64][]*notifications.Data {
	bnm.lock.Lock()

	// Swap map & reset greatest and least rounds
	var m *sync.Map
	m, bnm.bufMap = bnm.bufMap, &sync.Map{}
	lr := atomic.SwapUint64(bnm.lr, 0)
	gr := atomic.SwapUint64(bnm.gr, 0)

	bnm.lock.Unlock()

	outMap := make(map[int64][]*notifications.Data)

	// Function originally used to range over sync.Map, now called in order by RID on entries using gr and lr
	f := func(key, value interface{}) bool {
		l := value.([]*notifications.Data)
		for _, n := range l {
			nSlice, exists := outMap[n.EphemeralID]
			if exists {
				nSlice = append(nSlice, n)
			} else {
				nSlice = []*notifications.Data{n}
			}
			outMap[n.EphemeralID] = nSlice
		}
		return true
	}

	// Iterate through seen rounds from least to greatest
	for i := lr; i <= gr; i++ {
		rid := id.Round(i)
		nlist, ok := m.Load(rid)
		if !ok {
			jww.DEBUG.Printf("No notification data for round %+v", rid)
			continue
		}
		f(rid, nlist)
	}

	return outMap
}

// Add accepts a list of notification data and an associated round ID
// The list will be inserted to the current sync.Map under the given round ID
// NOTE: THIS WILL OVERWRITE, SHOULD BE CALLED ONCE PER ROUND, OR AGAIN TO REPLACE OVERFLOW NOTIFICATIONS
func (bnm *NotificationBuffer) Add(rid id.Round, l []*notifications.Data) {
	bnm.lock.RLock()
	defer bnm.lock.RUnlock()

	// Update stored round IDs
	bnm.updateRIDs(rid)

	// Store data for round
	bnm.bufMap.Store(rid, l)
}

func (bnm *NotificationBuffer) updateRIDs(rid id.Round) {
	flop := false
	for flop == false {
		gr := atomic.LoadUint64(bnm.gr)
		if gr < uint64(rid) || gr == 0 {
			flop = atomic.CompareAndSwapUint64(bnm.gr, gr, uint64(rid))
		} else {
			flop = true
		}
	}
	flop = false

	for flop == false {
		lr := atomic.LoadUint64(bnm.lr)
		if lr > uint64(rid) || lr == 0 {
			flop = atomic.CompareAndSwapUint64(bnm.lr, lr, uint64(rid))
		} else {
			flop = true
		}
	}
}
