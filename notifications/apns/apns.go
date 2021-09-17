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
