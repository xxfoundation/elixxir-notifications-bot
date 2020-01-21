////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////
package cmd

import (
	"bytes"
	"crypto/x509"
	"fmt"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/connect"
	"gitlab.com/elixxir/comms/node"
	"gitlab.com/elixxir/crypto/signature/rsa"
	"gitlab.com/elixxir/crypto/tls"
	"gitlab.com/elixxir/primitives/ndf"
	"gitlab.com/elixxir/primitives/utils"
	"gitlab.com/elixxir/registration/database"
	"gitlab.com/elixxir/registration/testkeys"
	"os"
	"testing"
	"time"
)

var nodeAddr = "0.0.0.0:6900"
var nodeCert []byte
var nodeKey []byte
var permAddr = "0.0.0.0:5900"
var testParams Params
var gatewayKey []byte
var gatewayCert []byte
var ndfFile []byte

/*
var testPermissioningKey *rsa.PrivateKey
var testpermissioningCert *x509.Certificate*/
var nodeComm *node.Comms

func TestMain(m *testing.M) {
	var err error
	nodeCert, err = utils.ReadFile(testkeys.GetNodeCertPath())
	if err != nil {
		fmt.Printf("Could not get node cert: %+v\n", err)
	}

	nodeKey, err = utils.ReadFile(testkeys.GetNodeKeyPath())
	if err != nil {
		fmt.Printf("Could not get node key: %+v\n", err)
	}

	gatewayKey, err = utils.ReadFile(testkeys.GetCAKeyPath())
	if err != nil {
		fmt.Printf("Could not get gateway key: %+v\n", err)
	}

	gatewayCert, err = utils.ReadFile(testkeys.GetCACertPath())
	if err != nil {
		fmt.Printf("Could not get gateway cert: %+v\n", err)
	}

	ndfFile, err = utils.ReadFile(testkeys.GetClientNdf())
	if err != nil {
		fmt.Printf("Could not get ndf: %+v\n", err)
	}

	testParams = Params{
		Address:       permAddr,
		CertPath:      testkeys.GetCACertPath(),
		KeyPath:       testkeys.GetCAKeyPath(),
		NdfOutputPath: testkeys.GetNDFPath(),
		publicAddress: permAddr,
	}
	nodeComm = node.StartNode(nodeAddr, node.NewImplementation(), nodeCert, nodeKey)

	runFunc := func() int {
		code := m.Run()
		nodeComm.Shutdown()
		return code
	}

	os.Exit(runFunc())
}

//Helper function that initailizes the permisssioning server's globals
//Todo: throw in the permDB??
func initPermissioningServerKeys() (*rsa.PrivateKey, *x509.Certificate) {
	permKeyBytes, _ := utils.ReadFile(testkeys.GetCAKeyPath())
	permCertBytes, _ := utils.ReadFile(testkeys.GetCACertPath())

	testPermissioningKey, _ := rsa.LoadPrivateKeyFromPem(permKeyBytes)
	testpermissioningCert, _ := tls.LoadCertificate(string(permCertBytes))
	return testPermissioningKey, testpermissioningCert

}

//Error path: Test an insertion on an empty database
func TestEmptyDataBase(t *testing.T) {
	//Start the registration server
	testParams := Params{
		CertPath: testkeys.GetCACertPath(),
		KeyPath:  testkeys.GetCAKeyPath(),
	}
	impl := StartRegistration(testParams)
	//Set the permissioning keys for testing
	//initPermissioningServerKeys()

	database.PermissioningDb = database.NewDatabase("test", "password", "regCodes", "0.0.0.0:6969")

	//using node cert as gateway cert
	err := impl.RegisterNode([]byte("test"), nodeAddr, string(nodeCert),
		nodeAddr, string(nodeCert), "AAA")
	if err == nil {
		expectedErr := "Unable to insert node: unable to register node AAA"
		t.Errorf("Database was empty but allowed a reg code to go through. "+
			"Expected %s, Recieved: %+v", expectedErr, err)
		return
	}
	impl.Comms.Shutdown()

}

//Happy path: looking for a code that is in the database
func TestRegCodeExists_InsertRegCode(t *testing.T) {

	impl := StartRegistration(testParams)
	impl.nodeCompleted = make(chan struct{}, 1)
	database.PermissioningDb = database.NewDatabase("test", "password", "regCodes", "0.0.0.0:6969")
	//Insert a sample regCode
	err := database.PermissioningDb.InsertNodeRegCode("AAAA")
	if err != nil {
		t.Errorf("Failed to insert client reg code %+v", err)
	}
	//Register a node with that regCode
	err = impl.RegisterNode([]byte("test"), nodeAddr, string(nodeCert),
		nodeAddr, string(nodeCert), "AAAA")
	if err != nil {
		t.Errorf("Registered a node with a known reg code, but recieved the following error: %+v", err)
	}

	//Kill the connections for the next test
	impl.Comms.Shutdown()
}

//Happy Path:  Insert a reg code along with a node
func TestRegCodeExists_RegUser(t *testing.T) {
	//Initialize an implementation and the permissioning server
	impl := &RegistrationImpl{}
	impl.nodeCompleted = make(chan struct{}, 1)

	jww.SetStdoutThreshold(jww.LevelInfo)

	impl.permissioningKey, impl.permissioningCert = initPermissioningServerKeys()

	//Inialiaze the database
	database.PermissioningDb = database.NewDatabase("test", "password", "regCodes", "0.0.0.0:6900")
	//Insert regcodes into it
	err := database.PermissioningDb.InsertClientRegCode("AAAA", 100)
	if err != nil {
		t.Errorf("Failed to insert client reg code %+v", err)
	}

	//Attempt to register a user
	sig, err := impl.RegisterUser("AAAA", string(nodeKey))

	if err != nil {
		t.Errorf("Failed to register a node when it should have worked: %+v", err)
	}

	if sig == nil {
		t.Errorf("Failed to sign public key, recieved %+v as a signature", sig)
	}

}

//Attempt to register a node after the
func TestCompleteRegistration_HappyPath(t *testing.T) {
	//Crate database
	database.PermissioningDb = database.NewDatabase("test", "password", "regCodes", "0.0.0.0:6969")
	//Insert a sample regCode
	strings := make([]string, 0)
	strings = append(strings, "BBBB")
	database.PopulateNodeRegistrationCodes(strings)
	RegistrationCodes = strings

	//Start the registration server
	impl := StartRegistration(testParams)
	RegParams = testParams
	go nodeRegistrationCompleter(impl)

	err := impl.RegisterNode([]byte("test"), "0.0.0.0:6900", string(nodeCert),
		"0.0.0.0:6900", string(nodeCert), "BBBB")
	//So the impl is not destroyed
	time.Sleep(5 * time.Second)

	if err != nil {
		t.Errorf("Expected happy path, recieved error: %+v", err)
	}
	//Kill the connections for the next test
	nodeComm.DisconnectAll()
	impl.Comms.Shutdown()

}

//Error path: test that trying to register with the same reg code fails
func TestDoubleRegistration(t *testing.T) {
	//Create database
	database.PermissioningDb = database.NewDatabase("test", "password", "regCodes", "0.0.0.0:6969")

	//Create reg codes and populate the database
	strings := make([]string, 0)
	strings = append(strings, "AAAA", "BBBB", "CCCC")
	database.PopulateNodeRegistrationCodes(strings)
	RegistrationCodes = strings
	RegParams = testParams

	//Start registration server
	impl := StartRegistration(testParams)
	go nodeRegistrationCompleter(impl)

	//Create a second node to register
	nodeComm2 := node.StartNode("0.0.0.0:6901", node.NewImplementation(), nodeCert, nodeKey)

	//Register 1st node
	err := impl.RegisterNode([]byte("test"), nodeAddr, string(nodeCert),
		nodeAddr, string(nodeCert), "BBBB")
	if err != nil {
		t.Errorf("Expected happy path, recieved error: %+v", err)
	}

	//Register 2nd node
	err = impl.RegisterNode([]byte("B"), "0.0.0.0:6901", string(nodeCert),
		"0.0.0.0:6901", string(nodeCert), "BBBB")
	//Kill the connections for the next test
	nodeComm.DisconnectAll()
	nodeComm2.DisconnectAll()
	nodeComm2.Shutdown()
	impl.Comms.Shutdown()
	time.Sleep(5 * time.Second)
	if err != nil {
		return
	}

	t.Errorf("Expected happy path, recieved error: %+v", err)
}

//Happy path: attempt to register 2 nodes
func TestTopology_MultiNodes(t *testing.T) {
	//Create database
	database.PermissioningDb = database.NewDatabase("test", "password", "regCodes", "0.0.0.0:6969")

	//Create reg codes and populate the database
	strings := make([]string, 0)
	strings = append(strings, "BBBB", "CCCC")
	database.PopulateNodeRegistrationCodes(strings)
	RegistrationCodes = strings
	RegParams = testParams

	//Start registration server
	impl := StartRegistration(testParams)
	go nodeRegistrationCompleter(impl)

	//Create a second node to register
	nodeComm2 := node.StartNode("0.0.0.0:6901", node.NewImplementation(), nodeCert, nodeKey)

	//Register 1st node
	err := impl.RegisterNode([]byte("A"), nodeAddr, string(nodeCert),
		nodeAddr, string(nodeCert), "BBBB")
	if err != nil {
		t.Errorf("Expected happy path, recieved error: %+v", err)
	}

	//Register 2nd node
	err = impl.RegisterNode([]byte("B"), "0.0.0.0:6901", string(gatewayCert),
		"0.0.0.0:6901", string(gatewayCert), "CCCC")
	if err != nil {
		t.Errorf("Expected happy path, recieved error: %+v", err)
	}
	//Sleep so that the permissioning has time to connect to the nodes (
	// ie impl isn't destroyed)
	time.Sleep(5 * time.Second)

	//Kill the connections for the next test
	nodeComm.DisconnectAll()
	nodeComm2.DisconnectAll()
	nodeComm2.Shutdown()
	impl.Comms.Shutdown()
	time.Sleep(5 * time.Second)
}

//Happy path
func TestRegistrationImpl_Polldf(t *testing.T) {
	//Create database
	database.PermissioningDb = database.NewDatabase("test", "password", "regCodes", "0.0.0.0:6969")

	//Create reg codes and populate the database
	strings := make([]string, 0)
	strings = append(strings, "BBBB", "CCCC", "DDDD")
	database.PopulateNodeRegistrationCodes(strings)
	RegistrationCodes = strings
	RegParams = testParams

	//Start registration server
	impl := StartRegistration(testParams)
	go nodeRegistrationCompleter(impl)

	//Start the other nodes
	nodeComm2 := node.StartNode("0.0.0.0:6901", node.NewImplementation(), nodeCert, nodeKey)
	nodeComm3 := node.StartNode("0.0.0.0:6902", node.NewImplementation(), nodeCert, nodeKey)
	udbId := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4}

	udbParams.ID = udbId

	//Register 1st node
	err := impl.RegisterNode([]byte("B"), nodeAddr, string(nodeCert),
		"0.0.0.0:7900", string(gatewayCert), "BBBB")
	if err != nil {
		t.Errorf("Expected happy path, recieved error: %+v", err)
	}

	//Register 2nd node
	err = impl.RegisterNode([]byte("C"), "0.0.0.0:6901", string(nodeCert),
		"0.0.0.0:7901", string(gatewayCert), "CCCC")
	if err != nil {
		t.Errorf("Expected happy path, recieved error: %+v", err)
	}

	//Register 3rd node
	err = impl.RegisterNode([]byte("D"), "0.0.0.0:6902", string(nodeCert),
		"0.0.0.0:7902", string(gatewayCert), "DDDD")
	if err != nil {
		t.Errorf("Expected happy path, recieved error: %+v", err)
	}
	//Wait for registration to complete
	time.Sleep(5 * time.Second)
	observedNDFBytes, err := impl.PollNdf(nil, &connect.Auth{})
	if err != nil {
		t.Errorf("failed to update ndf: %v", err)
	}

	observedNDF, _, err := ndf.DecodeNDF(string(observedNDFBytes))
	if err != nil {
		t.Errorf("Could not decode ndf: %v", err)
	}
	if bytes.Compare(observedNDF.UDB.ID, udbId) != 0 {
		t.Errorf("Failed to set udbID. Expected: %v, \nRecieved: %v", udbId, observedNDF.UDB.ID)
	}

	if observedNDF.Registration.Address != permAddr {
		t.Errorf("Failed to set registration address. Expected: %v \n Recieved: %v",
			permAddr, observedNDF.Registration.Address)
	}
	expectedNodeIDs := make([][]byte, 0)
	expectedNodeIDs = append(expectedNodeIDs, []byte("B"), []byte("C"), []byte("D"))
	for i := range observedNDF.Nodes {
		if bytes.Compare(expectedNodeIDs[i], observedNDF.Nodes[i].ID) != 0 {
			t.Errorf("Could not build node %d's, id: Expected: %v \n Recieved: %v", i,
				expectedNodeIDs, observedNDF.Nodes[i].ID)
		}
	}

	//Disconnect nodeComms
	nodeComm.DisconnectAll()
	nodeComm2.DisconnectAll()
	nodeComm3.DisconnectAll()
	//Shutdown node comms
	nodeComm2.Shutdown()
	nodeComm3.Shutdown()

	//Shutdown registration
	impl.Comms.Shutdown()
}

//Error  path
func TestRegistrationImpl_PollNdf_NoNDF(t *testing.T) {
	//Create database
	database.PermissioningDb = database.NewDatabase("test", "password", "regCodes", "0.0.0.0:6969")

	//Create reg codes and populate the database
	strings := make([]string, 0)
	strings = append(strings, "BBBB", "CCCC")
	database.PopulateNodeRegistrationCodes(strings)
	RegistrationCodes = strings
	RegParams = testParams

	//Start registration server
	impl := StartRegistration(testParams)
	go nodeRegistrationCompleter(impl)

	//Setup udb configurations
	udbId := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4}
	udbParams.ID = udbId

	//Register 1st node
	err := impl.RegisterNode([]byte("B"), nodeAddr, string(nodeCert),
		"0.0.0.0:7900", string(gatewayCert), "BBBB")
	if err != nil {
		t.Errorf("Expected happy path, recieved error: %+v", err)
	}

	time.Sleep(5 * time.Second)

	//Make a client ndf hash that is not up to date
	clientNdfHash := []byte("test")

	_, err = impl.PollNdf(clientNdfHash, &connect.Auth{})
	if err != nil {
		//Disconnect nodeComms
		nodeComm.DisconnectAll()

		//Shutdown registration
		impl.Comms.Shutdown()
		return
	}

	t.Error("Expected error path, should not have an ndf ready")
	//Disconnect nodeComms
	nodeComm.DisconnectAll()

	//Shutdown registration
	impl.Comms.Shutdown()
}

func TestRegistrationImpl_GetCurrentClientVersion(t *testing.T) {
	impl := StartRegistration(testParams)
	testVersion := "0.0.0a"
	setClientVersion(testVersion)
	version, err := impl.GetCurrentClientVersion()
	if err != nil {
		t.Error(err)
	}
	if version != testVersion {
		t.Errorf("Version was %+v, expected %+v", version, testVersion)
	}
}

// Test a case that should pass validation
func TestValidateClientVersion_Success(t *testing.T) {
	err := validateVersion("0.0.0a")
	if err != nil {
		t.Errorf("Unexpected error from validateVersion: %+v", err.Error())
	}
}

// Test some cases that shouldn't pass validation
func TestValidateClientVersion_Failure(t *testing.T) {
	err := validateVersion("")
	if err == nil {
		t.Error("Expected error for empty version string")
	}
	err = validateVersion("0")
	if err == nil {
		t.Error("Expected error for version string with one number")
	}
	err = validateVersion("0.0")
	if err == nil {
		t.Error("Expected error for version string with two numbers")
	}
	err = validateVersion("a.4.0")
	if err == nil {
		t.Error("Expected error for version string with non-numeric major version")
	}
	err = validateVersion("4.a.0")
	if err == nil {
		t.Error("Expected error for version string with non-numeric minor version")
	}
}
