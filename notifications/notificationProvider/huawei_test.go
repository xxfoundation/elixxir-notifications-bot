package notificationProvider

//func TestHuawei_Notify(t *testing.T) {
//	provider, err := NewHuawei(HuaweiParams{
//		AppId:     "105516491",
//		AppSecret: "",
//		AuthUrl:   "https://oauth-login.cloud.huawei.com/oauth2/v3/token",
//		PushUrl:   "https://push-api.cloud.huawei.com/v1/105516491/messages:send",
//	})
//
//	if err != nil {
//		t.Errorf("Failed to create provider: %+v", err)
//	}
//	fmt.Println("0")
//	_, err = provider.Notify("test", storage.GTNResult{
//		EphemeralId:          0,
//		Token:                "i'm-a-token",
//		TransmissionRSAHash:  nil,
//		NotificationProvider: uint32(notifications.HUAWEI),
//	})
//	if err != nil {
//		t.Errorf("Failed to notify: %+v", err)
//	}
//}
