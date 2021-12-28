package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"strconv"
	"sync"
	"sync/atomic"
)

type NotificationBuffer struct {
	bufMap  atomic.Value
	counter *int64
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

func (bnm *NotificationBuffer) Swap(maxNotifications uint) map[int64][]*pb.NotificationData {
	newSM := &sync.Map{}
	m := bnm.bufMap.Swap(newSM).(*sync.Map)

	outMap := make(map[int64][]*pb.NotificationData)
	f := func(_, value interface{}) bool {
		n := value.(*pb.NotificationData)
		nSlice, exists := outMap[n.EphemeralID]
		if exists {
			if uint(len(nSlice)) >= maxNotifications {
				bnm.Add(n)
			} else {
				nSlice = append(nSlice, n)
			}
		} else {
			nSlice = []*pb.NotificationData{n}
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
