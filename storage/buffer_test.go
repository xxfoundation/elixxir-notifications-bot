package storage

import (
	"gitlab.com/elixxir/primitives/notifications"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"math/rand"
	"testing"
	"time"
)

func TestNotificationBuffer_Sorting(t *testing.T) {
	nb := NewNotificationBuffer()
	uid1 := id.NewIdFromString("zezima", id.User, t)
	eid1, _, _, _ := ephemeral.GetId(uid1, 16, time.Now().UnixNano())
	uid2 := id.NewIdFromString("escaline", id.User, t)
	eid2, _, _, _ := ephemeral.GetId(uid2, 16, time.Now().UnixNano())

	for i := 0; i <= 5; i++ {
		nd := []*notifications.Data{}
		rand.Seed(time.Now().UnixNano())
		min := 2
		max := 5
		numNotifs := rand.Intn(max-min+1) + min
		rid := rand.Intn(500) + 1
		for j := 0; j <= numNotifs; j++ {
			msgHash := make([]byte, 32)
			ifp := make([]byte, 25)
			rand.Read(msgHash)
			rand.Read(ifp)
			var eid int64
			if rid%2 == 0 {
				eid = eid1.Int64()
			} else {
				eid = eid2.Int64()
			}
			nd = append(nd, &notifications.Data{
				EphemeralID: eid,
				RoundID:     uint64(rid),
				IdentityFP:  ifp,
				MessageHash: msgHash,
			})
		}
		nb.Add(id.Round(rid), nd)
	}

	sorted := nb.Swap()

	if nl, ok := sorted[eid1.Int64()]; ok {
		var last uint64
		for _, n := range nl {
			if n.RoundID < last {
				t.Error("Ordering was incorrect")
			}
			last = n.RoundID
		}
	}
	if nl, ok := sorted[eid2.Int64()]; ok {
		var last uint64
		for _, n := range nl {
			if n.RoundID < last {
				t.Error("Ordering was incorrect")
			}
			last = n.RoundID
		}
	}
}
