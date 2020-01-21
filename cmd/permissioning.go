////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles creating node registration callbacks for hooking into comms library

package cmd

import (
	"crypto/sha256"
	"encoding/json"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/crypto/tls"
	"gitlab.com/elixxir/primitives/ndf"
	"gitlab.com/elixxir/primitives/utils"
	"gitlab.com/elixxir/registration/certAuthority"
	"gitlab.com/elixxir/registration/database"
	"time"
)

// Handle registration attempt by a Node
func (m *RegistrationImpl) RegisterNode(ID []byte, ServerAddr, ServerTlsCert,
	GatewayAddr, GatewayTlsCert, RegistrationCode string) error {

	// Check that the node hasn't already been registered
	nodeInfo, err := database.PermissioningDb.GetNode(RegistrationCode)
	if err != nil {
		errMsg := errors.Errorf(
			"Registration code %+v is invalid or not currently enabled: %+v", RegistrationCode, err)
		jww.ERROR.Printf("%+v", errMsg)
		return errMsg
	}
	if nodeInfo.Id != nil {
		errMsg := errors.Errorf(
			"Node with registration code %+v has already been registered", RegistrationCode)
		jww.ERROR.Printf("%+v", errMsg)
		return errMsg
	}

	// Load the node and gateway certs
	nodeCertificate, err := tls.LoadCertificate(ServerTlsCert)
	if err != nil {
		errMsg := errors.Errorf("Failed to load node certificate: %v", err)
		jww.ERROR.Printf("%v", errMsg)
		return errMsg
	}
	gatewayCertificate, err := tls.LoadCertificate(GatewayTlsCert)
	if err != nil {
		errMsg := errors.Errorf("Failed to load gateway certificate: %v", err)
		jww.ERROR.Printf("%v", errMsg)
		return errMsg
	}

	// Sign the node and gateway certs
	signedNodeCert, err := certAuthority.Sign(nodeCertificate, m.permissioningCert, &(m.permissioningKey.PrivateKey))
	if err != nil {
		errMsg := errors.Errorf("failed to sign node certificate: %v", err)
		jww.ERROR.Printf("%v", errMsg)
		return errMsg
	}
	signedGatewayCert, err := certAuthority.Sign(gatewayCertificate, m.permissioningCert, &(m.permissioningKey.PrivateKey))
	if err != nil {
		errMsg := errors.Errorf("Failed to sign gateway certificate: %v", err)
		jww.ERROR.Printf("%v", errMsg)
		return errMsg
	}

	// Attempt to insert Node into the database
	err = database.PermissioningDb.InsertNode(ID, RegistrationCode,
		signedNodeCert, ServerAddr, GatewayAddr, signedGatewayCert)
	if err != nil {
		errMsg := errors.Errorf("unable to insert node: %+v", err)
		jww.ERROR.Printf("%+v", errMsg)
		return errMsg
	}
	jww.DEBUG.Printf("Inserted node %+v into the database with code %+v",
		ID, RegistrationCode)

	// Obtain the number of registered nodes
	_, err = database.PermissioningDb.CountRegisteredNodes()
	if err != nil {
		errMsg := errors.Errorf("Unable to count registered Nodes: %+v", err)
		jww.ERROR.Printf("%+v", errMsg)
		return errMsg
	}

	_, err = m.Comms.AddHost(string(ID), ServerAddr, []byte(ServerTlsCert), false, true)
	if err != nil {
		errMsg := errors.Errorf("Could not register host for Server %s: %+v", ServerAddr, err)
		jww.ERROR.Print(errMsg)
		return errMsg
	}

	jww.DEBUG.Printf("Total number of expected nodes for registration completion: %v", m.NumNodesInNet)
	m.nodeCompleted <- struct{}{}
	return nil
}

// Wrapper for completed node registration error handling
func nodeRegistrationCompleter(impl *RegistrationImpl) {
	// Wait for all Nodes to complete registration
	for numNodes := 0; numNodes < impl.NumNodesInNet; numNodes++ {
		jww.DEBUG.Printf("Registered %d node(s)!", numNodes)
		<-impl.nodeCompleted
	}
	// Assemble the completed topology
	gateways, nodes, err := assembleNdf(RegistrationCodes)
	if err != nil {
		jww.FATAL.Printf("unable to assemble topology: %+v", err)
	}

	//Assemble the registration server information
	registration := ndf.Registration{Address: RegParams.publicAddress, TlsCertificate: impl.certFromFile}

	//Construct an NDF
	networkDef := &ndf.NetworkDefinition{
		Registration: registration,
		Timestamp:    time.Now(),
		Nodes:        nodes,
		Gateways:     gateways,
		UDB:          udbParams,
		E2E:          RegParams.e2e,
		CMIX:         RegParams.cmix,
	}

	// Output the completed topology to a JSON file and save marshall'ed json data
	impl.ndfJson, err = outputToJSON(networkDef, impl.ndfOutputPath)
	if err != nil {
		errMsg := errors.Errorf("unable to output NDF JSON file: %+v", err)
		jww.FATAL.Printf(errMsg.Error())
	}
	//Serialize then hash the constructed ndf
	hash := sha256.New()
	ndfBytes := networkDef.Serialize()
	hash.Write(ndfBytes)
	//Lock the ndf before writing to it s.t. it's
	// threadsafe for nodes trying to register
	impl.ndfLock.Lock()
	impl.regNdfHash = hash.Sum(nil)
	impl.ndfLock.Unlock()
	// Alert that registration is complete
	impl.registrationCompleted <- struct{}{}
	jww.INFO.Printf("Node registration complete!")
}

// Assemble the completed topology from the database
func assembleNdf(codes []string) ([]ndf.Gateway, []ndf.Node, error) {
	var gateways []ndf.Gateway
	var nodes []ndf.Node
	for _, registrationCode := range codes {
		// Get node information for each registration code
		nodeInfo, err := database.PermissioningDb.GetNode(registrationCode)
		if err != nil {
			return nil, nil, errors.Errorf(
				"unable to obtain node for registration"+
					" code %+v: %+v", registrationCode, err)
		}
		var node ndf.Node
		node.ID = nodeInfo.Id
		node.TlsCertificate = nodeInfo.NodeCertificate
		node.Address = nodeInfo.ServerAddress

		var gateway ndf.Gateway
		gateway.TlsCertificate = nodeInfo.GatewayCertificate
		gateway.Address = nodeInfo.GatewayAddress
		//Since we are appending them simultaneously, indexing corresponding
		// gateway-node is just finding your index (as a gateway or a node)
		gateways = append(gateways, gateway)
		nodes = append(nodes, node)
	}
	jww.DEBUG.Printf("Assembled the network topology")
	return gateways, nodes, nil
}

// outputNodeTopologyToJSON encodes the NodeTopology structure to JSON and
// outputs it to the specified file path. An error is returned if the JSON
// marshaling fails or if the JSON file cannot be created.
func outputToJSON(ndfData *ndf.NetworkDefinition, filePath string) ([]byte, error) {
	// Generate JSON from structure
	data, err := json.Marshal(ndfData)
	if err != nil {
		return nil, err
	}
	// Write JSON to file
	err = utils.WriteFile(filePath, data, utils.FilePerms, utils.DirPerms)
	if err != nil {
		return data, err
	}

	return data, nil
}
