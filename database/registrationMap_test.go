////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package database

import "testing"

// Happy path
func TestMapImpl_InsertClientRegCode(t *testing.T) {
	m := &MapImpl{
		client: make(map[string]*RegistrationCode),
	}

	// Attempt to load in a valid code
	code := "TEST"
	uses := 100
	err := m.InsertClientRegCode(code, uses)

	// Verify the insert was successful
	if err != nil || m.client[code] == nil || m.client[code].
		RemainingUses != uses {
		t.Errorf("Expected to successfully insert client registration code")
	}
}

// Error Path: Duplicate client registration code
func TestMapImpl_InsertClientRegCode_Duplicate(t *testing.T) {
	m := &MapImpl{
		client: make(map[string]*RegistrationCode),
	}

	// Load in a registration code
	code := "TEST"
	uses := 100
	m.client[code] = &RegistrationCode{Code: code}

	// Attempt to load in a duplicate code
	err := m.InsertClientRegCode(code, uses)

	// Verify the insert failed
	if err == nil {
		t.Errorf("Expected to fail inserting duplicate client registration" +
			" code")
	}
}

// Happy path
func TestMapImpl_UseCode(t *testing.T) {
	m := &MapImpl{
		client: make(map[string]*RegistrationCode),
	}

	// Load in a registration code
	code := "TEST"
	uses := 100
	m.client[code] = &RegistrationCode{Code: code, RemainingUses: uses}

	// Verify the code was used successfully
	err := m.UseCode(code)
	if err != nil || m.client[code].RemainingUses != uses-1 {
		t.Errorf("Expected using client registration code to succeed")
	}
}

// Error Path: No remaining uses of client registration code
func TestMapImpl_UseCode_NoRemainingUses(t *testing.T) {
	m := &MapImpl{
		client: make(map[string]*RegistrationCode),
	}

	// Load in a registration code
	code := "TEST"
	uses := 0
	m.client[code] = &RegistrationCode{Code: code, RemainingUses: uses}

	// Verify the code was used successfully
	err := m.UseCode(code)
	if err == nil {
		t.Errorf("Expected using client registration code with no remaining" +
			" uses to fail")
	}
}

// Error Path: Invalid client registration code
func TestMapImpl_UseCode_Invalid(t *testing.T) {
	m := &MapImpl{
		client: make(map[string]*RegistrationCode),
	}

	// Verify the code was used successfully
	err := m.UseCode("TEST")
	if err == nil {
		t.Errorf("Expected using invalid client registration code with no to" +
			" fail")
	}
}
