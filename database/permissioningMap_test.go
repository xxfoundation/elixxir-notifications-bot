////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package database

import "testing"

// Happy path
func TestMapImpl_InsertNodeRegCode(t *testing.T) {
	m := &MapImpl{
		node: make(map[string]*NodeInformation),
	}

	// Attempt to load in a valid code
	code := "TEST"
	err := m.InsertNodeRegCode(code)

	// Verify the insert was successful
	if err != nil || m.node[code] == nil {
		t.Errorf("Expected to successfully insert node registration code")
	}
}

// Error Path: Duplicate node registration code
func TestMapImpl_InsertNodeRegCode_Duplicate(t *testing.T) {
	m := &MapImpl{
		node: make(map[string]*NodeInformation),
	}

	// Load in a registration code
	code := "TEST"
	m.node[code] = &NodeInformation{Code: code}

	// Attempt to load in a duplicate code
	err := m.InsertNodeRegCode(code)

	// Verify the insert failed
	if err == nil {
		t.Errorf("Expected to fail inserting duplicate node registration code")
	}
}

// Happy path
func TestMapImpl_InsertNode(t *testing.T) {
	m := &MapImpl{
		node: make(map[string]*NodeInformation),
	}

	// Load in a registration code
	code := "TEST"
	cert := "cert"
	gwCert := "gwcert"
	addr := "addr"
	gwAddr := "gwaddr"
	m.node[code] = &NodeInformation{Code: code}

	// Attempt to insert a node
	err := m.InsertNode(make([]byte, 0), code, cert, addr, gwAddr, gwCert)

	// Verify the insert was successful
	if info := m.node[code]; err != nil || info.NodeCertificate != cert ||
		info.GatewayCertificate != gwCert || info.ServerAddress != addr ||
		info.GatewayAddress != gwAddr {
		t.Errorf("Expected to successfully insert node information: %+v", info)
	}
}

// Error path: Invalid registration code
func TestMapImpl_InsertNode_Invalid(t *testing.T) {
	m := &MapImpl{
		node: make(map[string]*NodeInformation),
	}

	// Do NOT load in a registration code
	code := "TEST"

	// Attempt to insert a node without an associated registration code
	err := m.InsertNode(make([]byte, 0), code, code, code, code, code)

	// Verify the insert failed
	if err == nil {
		t.Errorf("Expected to fail inserting node information without the" +
			" correct registration code")
	}
}

// Full happy path
func TestMapImpl_CountRegisteredNodes(t *testing.T) {
	m := &MapImpl{
		node: make(map[string]*NodeInformation),
	}

	// Check that there are zero registered nodes
	count, err := m.CountRegisteredNodes()
	if err != nil || count != 0 {
		t.Errorf("Expected no registered nodes")
	}

	// Load in a registration code
	code := "TEST"
	m.node[code] = &NodeInformation{Code: code}

	// Check that adding an unregistered node still returns zero
	count, err = m.CountRegisteredNodes()
	if err != nil || count != 0 {
		t.Errorf("Still expected no registered nodes")
	}

	// Load in a node
	m.node[code].Id = make([]byte, 0)

	// Check that adding a registered node increases the count
	count, err = m.CountRegisteredNodes()
	if err != nil || count != 1 {
		t.Errorf("Expected a registered node")
	}
}

// Happy path
func TestMapImpl_GetNode(t *testing.T) {
	m := &MapImpl{
		node: make(map[string]*NodeInformation),
	}

	// Load in a registration code
	code := "TEST"
	m.node[code] = &NodeInformation{Code: code}

	// Check that the correct node is obtained
	info, err := m.GetNode(code)
	if err != nil || info.Code != code {
		t.Errorf("Expected to be able to obtain correct node")
	}
}

// Error path: Nonexistent registration code
func TestMapImpl_GetNode_Invalid(t *testing.T) {
	m := &MapImpl{
		node: make(map[string]*NodeInformation),
	}

	// Check that no node is obtained from empty map
	info, err := m.GetNode("TEST")
	if err == nil || info != nil {
		t.Errorf("Expected to not find the node")
	}
}

// Happy path
func TestMapImpl_InsertUser(t *testing.T) {
	m := &MapImpl{
		user: make(map[string]bool),
	}

	testKey := "TEST"
	_ = m.InsertUser(testKey)
	if !m.user[testKey] {
		t.Errorf("Insert failed to add the user!")
	}
}

// Happy path
func TestMapImpl_GetUser(t *testing.T) {
	m := &MapImpl{
		user: make(map[string]bool),
	}

	testKey := "TEST"
	m.user[testKey] = true

	user, err := m.GetUser(testKey)
	if err != nil || user.PublicKey != testKey {
		t.Errorf("Get failed to get user!")
	}
}

// Get user that does not exist
func TestMapImpl_GetUserNotExists(t *testing.T) {
	m := &MapImpl{
		user: make(map[string]bool),
	}

	testKey := "TEST"

	_, err := m.GetUser(testKey)
	if err == nil {
		t.Errorf("Get expected to not find user!")
	}
}
