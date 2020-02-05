////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////
package notifications

import (
	"crypto/sha256"
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/connect"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/id"
	"gitlab.com/elixxir/primitives/ndf"
)

var noNDFErr = errors.New("Failed to get ndf from permissioning: rpc error: code = Unknown desc = Permissioning server does not have an ndf to give to Notification Bot")

type pollCommInterface interface {
	GetHost(hostId string) (*connect.Host, bool)
	RequestNdf(host *connect.Host, message *pb.NDFHash) (*pb.NDF, error)
}

// PollNdf, attempts to connect to the permissioning server to retrieve the latest ndf for the notifications bot
func PollNdf(currentDef *ndf.NetworkDefinition, comms pollCommInterface) (*ndf.NetworkDefinition, error) {
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
		errMsg := fmt.Sprintf("Failed to get ndf from permissioning: %v", err)
		if errMsg == noNDFErr.Error() {
			jww.WARN.Println("Continuing without an updated NDF")
			return nil, nil
		}
		return nil, errors.New(errMsg)
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
		errMsg := fmt.Sprintf("Failed to decode response to ndf: %v", err)
		return nil, errors.New(errMsg)
	}
	return updatedNdf, nil
}
