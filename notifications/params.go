////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package notifications

import "gitlab.com/elixxir/notifications-bot/notifications/providers"

// Params struct holds info passed in for configuration
type Params struct {
	Address                string
	CertPath               string
	KeyPath                string
	NotificationsPerBatch  int
	MaxNotificationPayload int
	NotificationRate       int
	FBCreds                string
	APNS                   providers.APNSParams
	HavenFBCreds           string
	HavenAPNS              providers.APNSParams
	HttpsCertPath          string
	HttpsKeyPath           string
}
