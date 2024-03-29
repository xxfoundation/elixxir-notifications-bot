////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// ndf controls gateway updates from the permissioning server

package notifications

import (
	//"github.com/pkg/errors"
	"bytes"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/notifications-bot/io"
	"sync/atomic"
	"time"
)

// Stopper function that stops the thread on a timeout
type Stopper func(timeout time.Duration) bool

// GatewaysChanged function processes the gateways changed event when detected
// in the NDF
type GatewaysChanged func(ndf pb.NDF) ([]byte, error)

// TrackNdf kicks off the ndf tracking thread
func (nb *Impl) TrackNdf() {
	// Handler function for the gateways changed event
	gatewayEventHandler := func(ndf pb.NDF) ([]byte, error) {
		jww.DEBUG.Printf("Updating Gateways with new NDF")
		// TODO: If this returns an error, print that error if it occurs
		err := nb.inst.UpdatePartialNdf(&ndf)
		if err != nil {
			return nil, errors.WithMessage(err, "Failed to update partial NDF")
		}
		err = nb.inst.UpdateGatewayConnections()
		if err != nil {
			return nil, errors.WithMessage(err, "Failed to update gateway connections")
		}
		atomic.SwapUint32(nb.receivedNdf, 1)
		return nb.inst.GetPartialNdf().GetHash(), nil
	}

	// Stopping function for the thread
	quitCh := make(chan bool)
	nb.ndfStopper = func(timeout time.Duration) bool {
		select {
		case quitCh <- true:
			return true
		case <-time.After(timeout):
			jww.ERROR.Printf("Could not stop NDF Tracking Thread")
			return false
		}
	}

	// Polling object
	permHost, _ := nb.Comms.GetHost(nb.inst.GetPermissioningId())
	poller := io.NewNdfPoller(nb.Comms, permHost)

	go trackNdf(poller, quitCh, gatewayEventHandler)
}

func trackNdf(poller io.PollingConn, quitCh chan bool, gwEvt GatewaysChanged) {
	pollDelay := 1 * time.Second
	hashCh := make(chan []byte, 1)
	lastNdf := pb.NDF{Ndf: []byte{}}
	lastNdfHash := []byte{}
	for {
		jww.TRACE.Printf("Polling for NDF")

		// FIXME: This is mildly hacky because we rely on the call back
		// to return the ndf hash right now.
		select {
		case newHash := <-hashCh:
			lastNdfHash = newHash
		default:
			break
		}

		ndf, err := poller.PollNdf(lastNdfHash)
		if err != nil {
			jww.ERROR.Printf("polling ndf: %+v", err)
			ndf = nil
		}

		// If the cur differs from the last one, trigger the update
		// event
		// TODO: Improve this to only trigger when gatways are updated
		//       this isn't useful right now because gw event handlers
		//       actually update the full ndf each time, so it's a
		//       choice between comparing the full hash or additional
		//       network traffic given the current state of API.
		if ndf != nil && len(ndf.Ndf) > 0 && !bytes.Equal(ndf.Ndf, lastNdf.Ndf) {
			// FIXME: we should be able to get hash from the ndf
			// object, but we can't.
			go func() {
				h, err := gwEvt(*ndf)
				if err != nil {
					jww.ERROR.Println(err)
					return
				}
				hashCh <- h
			}()
			lastNdf = *ndf
		}

		select {
		case <-quitCh:
			jww.DEBUG.Printf("Exiting trackNDF thread...")
			return
		case <-time.After(pollDelay):
			continue
		}
	}
}

func (nb *Impl) ReceivedNdf() *uint32 {
	return nb.receivedNdf
}

func (nb *Impl) Cleaner() {
	cleanF := func(key, val interface{}) bool {
		t := val.(time.Time)
		if time.Since(t) > (5 * time.Minute) {
			nb.roundStore.Delete(key)
		}
		return true
	}

	cleanTicker := time.NewTicker(time.Minute * 10)

	for {
		select {
		case <-cleanTicker.C:
			nb.roundStore.Range(cleanF)
		}
	}
}
