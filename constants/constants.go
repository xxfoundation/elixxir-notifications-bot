////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package constants

const NotificationsTag = "notificationData"
const NotificationTitle = "Privacy: protected!"
const NotificationBody = "Some notifications are not for you to ensure privacy; we hope to remove this notification soon"

type App uint8

const (
	MessengerIOS App = iota
	MessengerAndroid
	HavenIOS
	HavenAndroid
)

func (a App) String() string {
	switch a {
	case MessengerIOS:
		return "messengerIOS"
	case MessengerAndroid:
		return "messengerAndroid"
	case HavenIOS:
		return "havenIOS"
	case HavenAndroid:
		return "havenAndroid"
	default:
		return "unknown"
	}
}
