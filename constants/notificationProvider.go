package constants

type NotificationProvider uint8

const (
	APNS = iota
	FCM
	HUAWEI
)

func (n NotificationProvider) String() string {
	return []string{"APNS", "FCM", "HUAWEI"}[n]
}
