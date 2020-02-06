////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// This file contains all code necessary for polling the NDF from the permissioning server

package notifications

import (
	"crypto/sha256"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/connect"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/id"
	"gitlab.com/elixxir/primitives/ndf"
	"strings"
)

var noNDFErr = errors.Errorf("Failed to get ndf from permissioning")

// We use an interface here inorder to allow us to mock the getHost and RequestNDF in the notifcationsBot.Comms for testing
type notificationComms interface {
	GetHost(hostId string) (*connect.Host, bool)
	RequestNdf(host *connect.Host, message *pb.NDFHash) (*pb.NDF, error)
}

// PollNdf, attempts to connect to the permissioning server to retrieve the latest ndf for the notifications bot
func PollNdf(currentDef *ndf.NetworkDefinition, comms notificationComms) (*ndf.NetworkDefinition, error) {
	//Hash the notifications bot ndf for comparison with registration's ndf
	hash := sha256.New()
	ndfBytes := currentDef.Serialize()
	hash.Write(ndfBytes)
	ndfHash := hash.Sum(nil)

	//Put the hash in a message
	msg := &pb.NDFHash{Hash: ndfHash}

	regHost, ok := comms.GetHost(id.PERMISSIONING)
	if !ok {
		return nil, errors.New("Failed to find permissioning host")
	}

	//Send the hash to registration
	response, err := comms.RequestNdf(regHost, msg)
	if err != nil {
		errMsg := errors.Errorf("Failed to get ndf from permissioning: %v", err)
		if  strings.Contains(errMsg.Error(), noNDFErr.Error()) {
			jww.WARN.Println("Continuing without an updated NDF")
			return nil, nil
		}
		return nil, errMsg
	}

	//If there was no error and the response is nil, client's ndf is up-to-date
	if response == nil || response.Ndf == nil {
		jww.DEBUG.Printf("Notification Bot NDF up-to-date")
		return nil, nil
	}

	jww.INFO.Printf("Remote NDF: %s", string(response.Ndf))

	//Otherwise pull the ndf out of the response
	updatedNdf, _, err := ndf.DecodeNDF(string(response.Ndf))
	if err != nil {
		//If there was an error decoding ndf
		errMsg := errors.Errorf("Failed to decode response to ndf: %v", err)
		return nil, errMsg
	}
	return updatedNdf, nil
}
