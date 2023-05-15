////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package notifications

import (
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/elixxir/primitives/notifications"
	"gitlab.com/xx_network/primitives/id"
	"time"
)

const notificationsTag = "notificationData"

// Sender is a long-running thread which sends out received notifications to
// the appropriate providers every sendFreq seconds.
func (nb *Impl) Sender(sendFreq int) {
	sendTicker := time.NewTicker(time.Duration(sendFreq) * time.Second)
	for {
		select {
		case <-sendTicker.C:
			go func() {
				// Retreive & swap notification buffer
				notifBuf := nb.Storage.GetNotificationBuffer()
				notifMap := notifBuf.Swap()

				if len(notifMap) == 0 {
					return
				}

				unsent := map[uint64][]*notifications.Data{}
				rest, err := nb.SendBatch(notifMap)
				if err != nil {
					jww.ERROR.Printf("Failed to send notification batch: %+v", err)
					// If we fail to run SendBatch, put everything back in unsent
					for _, elist := range notifMap {
						for _, n := range elist {
							unsent[n.RoundID] = append(unsent[n.RoundID], n)
						}
					}
				} else {
					// Loop through rest and add to unsent map
					for _, n := range rest {
						unsent[n.RoundID] = append(unsent[n.RoundID], n)
					}
				}
				// Re-add unsent notifications to the buffer
				for rid, nd := range unsent {
					notifBuf.Add(id.Round(rid), nd)
				}
			}()
		}
	}
}

// SendBatch accepts the map of ephemeralID:list[notifications.Data]
// It handles logic for building the CSV & sending to devices
func (nb *Impl) SendBatch(data map[int64][]*notifications.Data) ([]*notifications.Data, error) {
	csvs := map[int64]string{}
	var ephemerals []int64
	var unsent []*notifications.Data
	jww.INFO.Printf("data: %+v", data)
	for i, ilist := range data {
		var overflow, toSend []*notifications.Data
		if len(ilist) > nb.maxNotifications {
			overflow = ilist[nb.maxNotifications:]
			toSend = ilist[:nb.maxNotifications]
		} else {
			toSend = ilist[:]
		}

		notifs, rest := notifications.BuildNotificationCSV(toSend, nb.maxPayloadBytes-len([]byte(notificationsTag)))
		overflow = append(overflow, rest...)
		csvs[i] = string(notifs)
		ephemerals = append(ephemerals, i)
		unsent = append(unsent, overflow...)
	}
	toNotify, err := nb.Storage.GetToNotify(ephemerals)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to get list of tokens to notify")
	}
	for i := range toNotify {
		go func(res storage.GTNResult) {
			nb.notify(csvs[res.EphemeralId], res)
		}(toNotify[i])
	}
	return unsent, nil
}

// notify is a helper function which handles sending notifications to either APNS or firebase
func (nb *Impl) notify(csv string, toNotify storage.GTNResult) {
	provider, ok := nb.providers[toNotify.App]
	if !ok {
		jww.ERROR.Printf("Could not find provider for app %s", toNotify.App)
		return
	}
	tokenValid, err := provider.Notify(csv, toNotify)
	if err != nil {
		jww.ERROR.Println(err)
		if !tokenValid {
			jww.DEBUG.Printf("User with tRSA hash %+v has invalid token [%+v] for app %s - attempting to remove", toNotify.TransmissionRSAHash, toNotify.Token, toNotify.App)
			err := nb.Storage.DeleteToken(toNotify.Token)
			if err != nil {
				jww.ERROR.Printf("Failed to remove %s token registration tRSA hash %+v: %+v", toNotify.App, toNotify.TransmissionRSAHash, err)
			}
		}
	}
}
