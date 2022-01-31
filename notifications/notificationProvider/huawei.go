package notificationProvider

import (
	"gitlab.com/elixxir/notifications-bot/storage"
)

type Huawei struct {
}

func (h *Huawei) Notify(payload string, target storage.GTNResult) (bool, error) {
	return false, nil
}

func NewHuawei(clientID, clientSecret string) (*Huawei, error) {
	//conf := oauth2.Config{
	//	ClientID:     clientID,
	//	ClientSecret: clientSecret,
	//	Endpoint: oauth2.Endpoint{
	//		AuthURL:   "https://oauth-login.cloud.huawei.com/oauth2/v3/authorize",
	//		TokenURL:  "https://oauth-login.cloud.huawei.com/oauth2/v3/token",
	//		AuthStyle: oauth2.AuthStyleAutoDetect,
	//	},
	//	RedirectURL: "", // TODO: not totally sure how this works
	//	Scopes:      nil,
	//}
	return nil, nil
}
