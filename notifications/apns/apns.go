////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package apns

import "github.com/sideshow/apns2"

type ApnsComm struct {
	*apns2.Client
	topic string
}

func NewApnsComm(cl *apns2.Client, topic string) *ApnsComm {
	return &ApnsComm{
		Client: cl,
		topic:  topic,
	}
}

func (c *ApnsComm) GetTopic() string {
	return c.topic
}
