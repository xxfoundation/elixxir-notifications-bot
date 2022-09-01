////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package notifications

import (
	"bytes"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/testutils"
	"sync"
	"testing"
	"time"
)

type MockPoller struct {
	ndf pb.NDF
	sync.Mutex
}

func (m *MockPoller) PollNdf(ndfHash []byte) (*pb.NDF, error) {
	m.Lock()
	defer m.Unlock()
	return &m.ndf, nil
}
func (m *MockPoller) UpdateNdf(newNDF pb.NDF) {
	m.Lock()
	defer m.Unlock()
	m.ndf = newNDF
}

// TestTrackNdf performs a basic test of the trackNdf function
func TestTrackNdf(t *testing.T) {
	// Stopping function for the thread
	quitCh := make(chan bool)

	startNDF := pb.NDF{Ndf: make([]byte, 10)}
	copy(startNDF.Ndf, testutils.ExampleNDF[0:10])

	newNDF := pb.NDF{Ndf: make([]byte, 10)}
	copy(newNDF.Ndf, testutils.ExampleNDF[0:10])

	poller := &MockPoller{
		ndf: startNDF,
	}

	gwUpdates := 0
	lastNdf := make([]byte, 10)
	gatewayEventHandler := func(ndf pb.NDF) ([]byte, error) {
		t.Logf("Updating Gateways with new NDF")
		t.Logf("%v == %v?", ndf.Ndf, lastNdf)
		if !bytes.Equal(lastNdf, ndf.Ndf) {
			t.Logf("Incrementing counter")
			copy(lastNdf, ndf.Ndf)
			gwUpdates++
		}
		// We control the hash, so we control the update calls...
		ndfHash := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, byte(gwUpdates % 255)}
		return ndfHash, nil
	}

	go trackNdf(poller, quitCh, gatewayEventHandler)

	// 3 changes, starting change
	time.Sleep(100 * time.Millisecond)

	// 2nd change Start -> newNDF
	newNDF.Ndf[5] = byte('a')
	poller.UpdateNdf(newNDF)
	time.Sleep(1100 * time.Millisecond)

	// 3rd change newNDF -> startNDF
	poller.UpdateNdf(startNDF)
	time.Sleep(1100 * time.Millisecond)

	select {
	case quitCh <- true:
		break
	case <-time.After(2 * time.Second):
		t.Errorf("Could not stop NDF Tracking Thread")
	}

	if gwUpdates != 3 {
		t.Errorf("updates not detected, expected 3 got: %d", gwUpdates)
	}

}
