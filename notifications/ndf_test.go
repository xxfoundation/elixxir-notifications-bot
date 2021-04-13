///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package notifications

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"sync"
	"testing"
	"time"
)

type MockPoller struct {
	ndf *pb.NDF
	sync.Mutex
}

func (m MockPoller) PollNdf() (*pb.NDF, error) {
	m.Lock()
	defer m.Unlock()
	return m.ndf, nil
}
func (m MockPoller) UpdateNdf(newNDF *pb.NDF) {
	m.Lock()
	defer m.Unlock()
	m.ndf = newNDF
}

// TestTrackNdf performs a basic test of the trackNdf function
func TestTrackNdf(t *testing.T) {
	// Stopping function for the thread
	quitCh := make(chan bool)

	poller := MockPoller{
		ndf: nil,
	}

	gwUpdates := 0
	gatewayEventHandler := func(ndf pb.NDF) {
		t.Logf("Updating Gateways with new NDF")
		gwUpdates += 1
	}

	go trackNdf(poller, quitCh, gatewayEventHandler)

	select {
	case <-time.After(2 * time.Second):
		t.Errorf("Could not stop NDF Tracking Thread")
	case quitCh <- true:
		break
	}

}
