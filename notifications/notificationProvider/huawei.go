package notificationProvider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/notifications-bot/constants"
	"gitlab.com/elixxir/notifications-bot/storage"
	"io/ioutil"
	"net/http"
	"time"
)

const HuaweiPostURL = "https://push-api.cloud.huawei.com/v1/%s/messages:send"

const (
	// Success code from push server
	Success = "80000000"
	// ParameterError invalid code from push server
	ParameterError = "80100001"
	// TokenFailedErr invalid code from push server
	TokenFailedErr = "80200001"
	// TokenTimeoutErr timeout code from push server
	TokenTimeoutErr = "80200003"
)

// Huawei provider implementation
type Huawei struct {
	client    *http.Client
	appID     string
	appSecret string
	authURL   string
	pushURL   string
	token     string
	expires   time.Time
}

// HuaweiConfig details the config to create a huawei provider
type HuaweiConfig struct {
	AppId     string
	AppSecret string
	AuthUrl   string
	PushUrl   string
}

/* HUAWEI HTTP CALL STRUCTURES */
// SEE DEMO CODE AT https://github.com/HMS-Core/hms-push-serverdemo-go/tree/master/src/push FOR EXAMPLES OF USE

type TokenMsg struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type MessageRequest struct {
	ValidateOnly bool     `json:"validate_only"`
	Message      *Message `json:"message"`
}

type MessageResponse struct {
	Code      string `json:"code"`
	Msg       string `json:"msg"`
	RequestId string `json:"requestId"`
}

type Message struct {
	Data         string        `json:"data,omitempty"`
	Notification *Notification `json:"notification,omitempty"`
	// Android      *AndroidConfig `json:"android,omitempty"`
	// Apns *Apns `json:"apns,omitempty"`
	// WebPush      *WebPushConfig `json:"webpush,omitempty"`
	Token     []string `json:"token,omitempty"`
	Topic     string   `json:"topic,omitempty"`
	Condition string   `json:"condition,omitempty"`
}

type Notification struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	Image string `json:"image,omitempty"`
}

type Apns struct {
	Headers    *ApnsHeaders           `json:"headers,omitempty"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
	HmsOptions *ApnsHmsOptions        `json:"hms_options,omitempty"`
}

type ApnsHmsOptions struct {
	TargetUserType int `json:"target_user_type,omitempty"`
}

type ApnsHeaders struct {
	Authorization  string `json:"authorization,omitempty"`
	ApnsId         string `json:"apns-id,omitempty"`
	ApnsExpiration int64  `json:"apns-expiration,omitempty"`
	ApnsPriority   string `json:"apns-priority,omitempty"`
	ApnsTopic      string `json:"apns-topic,omitempty"`
	ApnsCollapseId string `json:"apns-collapse-id,omitempty"`
}

type Aps struct {
	Alert            interface{} `json:"alert,omitempty"` // dictionary or string
	Badge            int         `json:"badge,omitempty"`
	Sound            string      `json:"sound,omitempty"`
	ContentAvailable int         `json:"content-available,omitempty"`
	Category         string      `json:"category,omitempty"`
	ThreadId         string      `json:"thread-id,omitempty"`
}

type AlertDictionary struct {
	Title        string   `json:"title,omitempty"`
	Body         string   `json:"body,omitempty"`
	TitleLocKey  string   `json:"title-loc-key,omitempty"`
	TitleLocArgs []string `json:"title-loc-args,omitempty"`
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
}

func (h *Huawei) Notify(payload string, target storage.GTNResult) (bool, error) {
	ctx := context.Background()
	if h.token == "" || time.Now().After(h.expires) {
		err := h.getAuthToken(&ctx)
		if err != nil {
			return true, errors.WithMessage(err, "Could not get a current auth token with huawei")
		}
	}

	n := Message{
		Data: payload,
		Notification: &Notification{
			Title: constants.NotificationTitle,
			Body:  constants.NotificationBody,
		},
		Token: []string{target.Token},
	}
	body, err := json.Marshal(n)
	if err != nil {
		return true, errors.WithMessage(err, "Could not marshal json payload")
	}
	buf := bytes.NewBuffer(body)
	req, err := http.NewRequest(http.MethodPost, h.pushURL, buf)
	if err != nil {
		return true, errors.WithMessage(err, "Failed to create HTTP request to send notification")
	}
	h.resetRequestAuthHeader(req)

	messageResponse, err := h.doNotifyRequest(req)
	if err != nil {
		return true, err
	}

	// Check oauth token responses
	if messageResponse.Code == TokenTimeoutErr || messageResponse.Code == TokenFailedErr {
		jww.DEBUG.Print("Token authentication failed, retrieving new token & retrying...")
		err = h.getAuthToken(&ctx)
		if err != nil {
			return true, errors.WithMessage(err, "Failed to refresh auth token")
		}
		h.resetRequestAuthHeader(req)

		messageResponse, err = h.doNotifyRequest(req)
		if err != nil {
			return true, errors.WithMessage(err, "Message send failed a second time, will not retry again")
		}
	}

	// TODO: check if user token is OK?

	return true, nil
}

func NewHuawei(config *HuaweiConfig) (*Huawei, error) {
	transport := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: true}, // Do we want this off?
	}

	return &Huawei{
		client: &http.Client{
			Transport: transport,
		},
		appID:     config.AppId,
		appSecret: config.AppSecret,
		authURL:   config.AuthUrl,
		pushURL:   config.PushUrl,
	}, nil
}

func (h *Huawei) getAuthToken(ctx *context.Context) error {
	// Create authentication request
	body := fmt.Sprintf("grant_type=client_credentials&client_secret=%s&client_id=%s", h.appSecret, h.appID)
	req, err := http.NewRequest(http.MethodPost, h.authURL, bytes.NewBuffer([]byte(body)))
	if err != nil {
		return errors.WithMessage(err, "Failed to create http request")
	}

	// Execute authentication request
	resp, err := h.client.Do(req)
	if err != nil {
		return errors.WithMessage(err, "Failed to execute http request")
	} else if resp.StatusCode != 200 {
		// TODO: flesh out this error path
		return errors.New("Bad status code")
	}

	// Parse response from server
	tokenMsg := &TokenMsg{}
	tokenResponse, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.WithMessage(err, "Failed to read response message")
	}
	err = json.Unmarshal(tokenResponse, tokenMsg)
	if err != nil {
		return errors.WithMessage(err, "Failed to parse response message")
	}

	// Set token & expiry
	h.token = tokenMsg.AccessToken
	h.expires = time.Now().Add(time.Second * time.Duration(tokenMsg.ExpiresIn))

	return nil
}

func (h *Huawei) resetRequestAuthHeader(req *http.Request) {
	req.Header = http.Header{}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", h.token))
}

func (h *Huawei) doNotifyRequest(req *http.Request) (*MessageResponse, error) {
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to execute HTTP request to send notification")
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to read body of response to notification")
	}

	messageResponse := &MessageResponse{}
	err = json.Unmarshal(respBody, messageResponse)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to unmarshal message response")
	}
	return messageResponse, nil
}
