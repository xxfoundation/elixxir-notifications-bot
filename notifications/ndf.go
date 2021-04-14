///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// ndf controls gateway updates from the permissioning server

package notifications

import (
	//"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/notifications-bot/io"
	"gitlab.com/xx_network/comms/connect"
	//"gitlab.com/elixxir/comms/notificationBot"
	"bytes"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/notificationBot"
	"gitlab.com/elixxir/crypto/hash"
	"time"
)

// Stopper function that stops the thread on a timeout
type Stopper func(timeout time.Duration) bool

// GatewaysChanged function processes the gateways changed event when detected
// in the NDF
type GatewaysChanged func(ndf pb.NDF) []byte

// TrackNdf kicks off the ndf tracking thread
func TrackNdf(i *network.Instance, c *notificationBot.Comms) Stopper {
	// Handler function for the gateways changed event
	gatewayEventHandler := func(ndf pb.NDF) []byte {
		jww.DEBUG.Printf("Updating Gateways with new NDF")
		// TODO: If this returns an error, print that error if it occurs
		i.UpdateFullNdf(ndf)
		i.UpdateGatewayConnections()
		return i.GetFullNdf().GetHash()
	}

	// Stopping function for the thread
	quitCh := make(chan bool)
	quitFn := func(timeout time.Duration) bool {
		select {
		case quitCh <- true:
			return true
		case <-time.After(timeout):
			jww.ERROR.Printf("Could not stop NDF Tracking Thread")
			return false
		}
	}

	// Polling object
	permHost := c.GetHost(i.GetPermissioningId())
	poller := io.NewNdfPoller(c, permHost)

	go trackNdf(poller, quitCh, gatewayEventHandler)

	return quitFn
}

func trackNdf(poller io.PollingConn, quitCh chan bool, gwEvt GatewaysChanged) {
	lastNdfHash := make([]byte, 32)
	pollDelay := 1 * time.Second
	cMixHash, _ := hash.NewCMixHash()
	nonce := make([]byte, 32)
	hashCh := make(chan []byte, 1)
	for {
		jww.TRACE.Printf("Polling for NDF")
		ndf, err := poller.PollNdf(lastNdfHash)
		if err != nil {
			jww.ERROR.Printf("polling ndf: %+v", err)
			time.Sleep(pollDelay)
			continue
		}

		// If the cur Hash differs from the last one, trigger the update
		// event
		// TODO: Improve this to only trigger when gatways are updated
		//       this isn't useful right now because gw event handlers
		//       actually update the full ndf each time, so it's a
		//       choice between comparing the full hash or additional
		//       network traffic given the current state of API.
		// FIXME: This is mildly hacky because we rely on the call back
		// to return the ndf hash right now.
		curNdfHash := lastNdfHash
		select {
		case hashUpdate := <-hashCh:
			curNdfHash = hashUpdate
		default:
			break
		}
		if bytes.Equal(curNdfHash, lastNdfHash) {
			// FIXME: we should be able to get hash from the ndf
			// object, but we can't.
			go func() { hashCh <- gwEvt(*ndf) }()
		}

		lastNdfHash = curNdfHash

		time.Sleep(pollDelay)
	}
}
