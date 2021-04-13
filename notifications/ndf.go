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
	"gitlab.com/elixxir/crypto/hash"
	"time"
)

// Stopper function that stops the thread on a timeout
type Stopper func(timeout time.Duration) bool

// GatewaysChanged function processes the gateways changed event when detected
// in the NDF
type GatewaysChanged func(ndf pb.NDF)

// InstanceObject is a mock of the instance object...
type InstanceObject interface {
	UpdateGateways(ndf *pb.NDF)
	GetProtoComms() *connect.ProtoComms
	GetPermHost() *connect.Host
}

// TrackNdf kicks off the ndf tracking thread
func TrackNdf(i InstanceObject) Stopper {
	// Handler function for the gateways changed event
	gatewayEventHandler := func(ndf pb.NDF) {
		jww.DEBUG.Printf("Updating Gateways with new NDF")
		// TODO: If this returns an error, print that error if it occurs
		i.UpdateGateways(&ndf)
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
	poller := io.NewNdfPoller(i.GetProtoComms(), i.GetPermHost())

	go trackNdf(poller, quitCh, gatewayEventHandler)

	return quitFn
}

func trackNdf(poller io.PollingConn, quitCh chan bool, gwEvt GatewaysChanged) {
	lastNdfHash := make([]byte, 32)
	pollDelay := 1 * time.Second
	cMixHash, _ := hash.NewCMixHash()
	nonce := make([]byte, 32)
	for {
		jww.TRACE.Printf("Polling for NDF")
		ndf, err := poller.PollNdf()
		if err != nil {
			jww.ERROR.Printf("polling ndf: %+v", err)
			time.Sleep(pollDelay)
		}
		// If the cur Hash differs from the last one, trigger the update
		// event
		// TODO: Improve this to only trigger when gatways are updated
		curNdfHash := ndf.Digest(nonce, cMixHash)
		if bytes.Equal(curNdfHash, lastNdfHash) {
			go gwEvt(*ndf)
		}

		lastNdfHash = curNdfHash

		time.Sleep(pollDelay)
	}
}
