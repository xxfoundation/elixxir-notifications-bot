////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Poll the network for the NDF. Users should create an ndf Poller with:
//     poller := NewNdfPoller(Protocom object, permissioning host)
// and subsequently call poller.GetNdf() to get a new copy of the NDF.
//
// Use the "PollingConn" interface in functions so you can mock the NDF for
// testing.

package io

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/notificationBot"
	"gitlab.com/xx_network/comms/connect"
)

// PollingConn is an object that implements the PollNdf Function
// and allows it to be mocked for testing.
type PollingConn interface {
	PollNdf(ndfHash []byte) (*pb.NDF, error)
}

// NdfPoller is a regular connection to the permissioning server, created
// with a protocomms object.
type NdfPoller struct {
	permHost *connect.Host
	pc       *notificationBot.Comms
}

// NewNdfPoller creates a new permconn object with a host and protocomms id.
func NewNdfPoller(pc *notificationBot.Comms, pHost *connect.Host) NdfPoller {
	return NdfPoller{
		pc:       pc,
		permHost: pHost,
	}
}

// PollNdf gets the NDF from the Permissioning server.
func (p NdfPoller) PollNdf(ndfHash []byte) (*pb.NDF, error) {
	permHost := p.permHost
	return p.pc.PollNdf(permHost, ndfHash)
}
