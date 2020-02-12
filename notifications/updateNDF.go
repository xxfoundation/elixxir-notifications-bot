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
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/id"
	"gitlab.com/elixxir/primitives/ndf"
	"strings"
)

var noNDFErr = errors.Errorf("Permissioning server does not have an ndf to give to client")

// PollNdf, attempts to connect to the permissioning server to retrieve the latest ndf for the notifications bot
func PollNdf(currentDef *ndf.NetworkDefinition, comms NotificationComms) (*ndf.NetworkDefinition, error) {
	//Hash the notifications bot ndf for comparison with registration's ndf
	var ndfHash []byte
	if currentDef != nil {
		hash := sha256.New()
		ndfBytes := currentDef.Serialize()
		hash.Write(ndfBytes)
		ndfHash = hash.Sum(nil)
	}

	//Put the hash in a message
	msg := &pb.NDFHash{Hash: ndfHash} // TODO: this should be a helper somewhere

	regHost, ok := comms.GetHost(id.PERMISSIONING)
	if !ok {
		return nil, errors.New("Failed to find permissioning host")
	}

	//Send the hash to registration
	response, err := comms.RequestNdf(regHost, msg)
	if err != nil {
		errMsg := errors.Wrap(err, "Failed to get ndf from permissioning")
		if strings.Contains(errMsg.Error(), noNDFErr.Error()) {
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
