////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Package providers contains logic for interacting with external notifications providers such as APNS

package providers

import "gitlab.com/elixxir/notifications-bot/storage"

// Provider interface represents an external notification provider, implementing
// an easy-to-use Notify function for the rest of the repo to call.
type Provider interface {
	// Notify sends a notification and returns the token status and an error
	Notify(csv string, target storage.GTNResult) (bool, error)
}
