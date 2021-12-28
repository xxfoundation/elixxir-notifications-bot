package storage

import (
	"bytes"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"strconv"
	"sync"
	"sync/atomic"
)

type NotificationBuffer struct {
	bufMap  atomic.Value
	counter *int64
}

type NotificationCSV struct {
	Csv   *bytes.Buffer
	count uint
}

func NewNotificationBuffer() *NotificationBuffer {
	u := int64(0)
	sm := sync.Map{}
	nb := &NotificationBuffer{
		counter: &u,
		bufMap:  atomic.Value{},
	}
	nb.bufMap.Store(&sm)
	return nb
}

func (bnm *NotificationBuffer) Swap(maxNotifications uint, maxSize int) map[int64]NotificationCSV {
	newSM := &sync.Map{}
	m := bnm.bufMap.Swap(newSM).(*sync.Map)

	outMap := make(map[int64]NotificationCSV)
	f := func(_, value interface{}) bool {
		n := value.(*pb.NotificationData)
		nSlice, exists := outMap[n.EphemeralID]
		if exists {
			if nSlice.count >= maxNotifications || nSlice.Csv.Len() >= maxSize {
				bnm.Add(n)
			} else {
				var ok bool
				nSlice.Csv, ok = pb.UpdateNotificationCSV(n, nSlice.Csv, maxSize)
				if !ok {
					bnm.Add(n)
				}
			}
		} else {
			nSlice = NotificationCSV{
				Csv:   &bytes.Buffer{},
				count: 0,
			}
		}
		outMap[n.EphemeralID] = nSlice
		return true
	}

	m.Range(f)

	return outMap
}

func (bnm *NotificationBuffer) Add(n *pb.NotificationData) {
	c := atomic.AddInt64(bnm.counter, 1)
	bnm.bufMap.Load().(*sync.Map).Store(strconv.FormatInt(c, 16), n)
}
