////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package firebase

import (
	"context"
	"firebase.google.com/go/messaging"
	"testing"
)

type MockSender struct{}

const token = "foIh7-NdlksspjDwT8O5kT:APA91bEQUCFeAadkIE-T3fHqAIIYwZm8lks0wQRIp5oh0qtMtjHcPjQhVZ3IDntZlv7PYAcHvDeu_7ncI8GcAlKama7YjzSLO9MgtAjxZMFivVfzQb-BD-6u0-MrJNR6XoOB9YX059ZB"

func (MockSender) Send(ctx context.Context, app *messaging.Message) (string, error) {
	return "test", nil
}

// This tests the function which sends a notification to firebase.
// Note: this requires you to have a valid token & service credentials
func TestSendNotification(t *testing.T) {
	app := MockSender{}

	_, err := sendNotification(app, token, "data")
	if err != nil {
		t.Error(err.Error())
	}
}

// Unit test the NewFirebaseComm method
func TestNewFirebaseComm(t *testing.T) {
	comm := NewFirebaseComm(nil)
	if comm.SendNotification == nil {
		t.Error("Failed to set functions in comm")
	}
}

/*
 * This function can't be unit tested without mocking firebase's infrastructure to a degree that is counterproductive
func TestSetupMessagingApp(t *testing.T) {
	dir, _ := os.Getwd()
	_, _, err := setupMessagingApp(dir+"/../creds/serviceAccountKey.json")
	if err != nil {
		t.Errorf("Failed to setup messaging app: %+v", err)
	}
}
*/
