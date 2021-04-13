///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Poll the network for the NDF. Users should create an ndf Poller with:
//     poller := NewNdfPoller(Protocom object, permissioning host)
// and subsequently call poller.GetNdf() to get a new copy of the NDF.
//
// Use the "PollingConn" interface in functions so you can mock the NDF for
// testing.

package io

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/xx_network/comms/connect"
	"google.golang.org/grpc"
)

// PollingConn is an object that implements the PollNdf Function
// and allows it to be mocked for testing.
type PollingConn interface {
	PollNdf() (*pb.NDF, error)
}

// NdfPoller is a regular connection to the permissioning server, created
// with a protocomms object.
type NdfPoller struct {
	permHost *connect.Host
	pc       *connect.ProtoComms
}

// NewNdfPoller creates a new permconn object with a host and protocomms id.
func NewNdfPoller(pc *connect.ProtoComms, permHost *connect.Host) NdfPoller {
	return NdfPoller{
		pc:       pc,
		permHost: permHost,
	}
}

// PollNdf gets the NDF from the Permissioning server.
func (p NdfPoller) PollNdf() (*pb.NDF, error) {
	permHost := p.permHost
	// Create the Send Function
	f := func(conn *grpc.ClientConn) (*any.Any, error) {
		// Set up the context
		ctx, cancel := connect.MessagingContext()
		defer cancel()

		// We use an empty NDF Hash to request an NDF
		ndfRequest := &pb.NDFHash{Hash: make([]byte, 0)}

		// Send the message
		clientConn := pb.NewRegistrationClient(conn)
		resultMsg, err := clientConn.PollNdf(ctx, ndfRequest)
		if err != nil {
			return nil, errors.New(err.Error())
		}
		return ptypes.MarshalAny(resultMsg)
	}

	// Execute the Send function
	jww.TRACE.Printf("Sending Request Ndf message...")
	resultMsg, err := p.pc.Send(permHost, f)
	if err != nil {
		return nil, err
	}

	result := &pb.NDF{}
	err = ptypes.UnmarshalAny(resultMsg, result)
	return result, err
}
