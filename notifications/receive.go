////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package notifications

import (
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/notifications"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/primitives/id"
	"time"
)

// ReceiveNotificationBatch receives the batch of notification data from gateway.
func (nb *Impl) ReceiveNotificationBatch(notifBatch *pb.NotificationBatch, auth *connect.Auth) error {
	rid := notifBatch.RoundID

	_, loaded := nb.roundStore.LoadOrStore(rid, time.Now())
	if loaded {
		jww.DEBUG.Printf("Dropping duplicate notification batch for round %+v", notifBatch.RoundID)
		return nil
	}

	jww.INFO.Printf("Received notification batch for round %+v", notifBatch.RoundID)

	buffer := nb.Storage.GetNotificationBuffer()
	data := processNotificationBatch(notifBatch)
	buffer.Add(id.Round(notifBatch.RoundID), data)

	return nil
}

func processNotificationBatch(l *pb.NotificationBatch) []*notifications.Data {
	var res []*notifications.Data
	for _, item := range l.Notifications {
		res = append(res, &notifications.Data{
			EphemeralID: item.EphemeralID,
			RoundID:     l.RoundID,
			IdentityFP:  item.IdentityFP,
			MessageHash: item.MessageHash,
		})
	}
	return res
}
