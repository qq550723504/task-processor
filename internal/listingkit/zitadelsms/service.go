package zitadelsms

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20190711"
)

var (
	ErrUnauthorizedWebhook  = errors.New("unauthorized ZITADEL SMS webhook")
	ErrInvalidPayload       = errors.New("invalid ZITADEL SMS payload")
	ErrDeliveryFailed       = errors.New("Tencent SMS delivery failed")
	ErrInvalidConfiguration = errors.New("invalid ZITADEL SMS configuration")
)

const (
	zitadelSignatureTolerance = 5 * time.Minute
	tencentSMSRegion          = "ap-guangzhou"
)

type Message struct {
	Phone      string
	TemplateID string
	SignName   string
	AppID      string
	Params     []string
}

type Sender interface {
	Send(context.Context, Message) error
}

type Config struct {
	SigningKey string
	TemplateID string
	SignName   string
	AppID      string
}

type Service struct {
	config Config
	sender Sender
}

func NewService(config Config, sender Sender) (*Service, error) {
	if strings.TrimSpace(config.SigningKey) == "" ||
		strings.TrimSpace(config.TemplateID) == "" ||
		strings.TrimSpace(config.SignName) == "" ||
		strings.TrimSpace(config.AppID) == "" ||
		sender == nil {
		return nil, ErrInvalidConfiguration
	}
	return &Service{config: config, sender: sender}, nil
}

func (s *Service) Deliver(ctx context.Context, body []byte, signature string) error {
	if s == nil || !validateZitadelSignature(body, signature, s.config.SigningKey, time.Now()) {
		return ErrUnauthorizedWebhook
	}

	payload, ok := parseZitadelSMSPayload(body)
	if !ok {
		return ErrInvalidPayload
	}

	if err := s.sender.Send(ctx, Message{
		Phone:      payload.ContextInfo.RecipientPhoneNumber,
		TemplateID: s.config.TemplateID,
		SignName:   s.config.SignName,
		AppID:      s.config.AppID,
		Params:     []string{payload.Args.Code},
	}); err != nil {
		return ErrDeliveryFailed
	}
	return nil
}

type zitadelSMSPayload struct {
	ContextInfo struct {
		RecipientPhoneNumber string `json:"recipientPhoneNumber"`
		EventType            string `json:"eventType"`
	} `json:"contextInfo"`
	TemplateData struct {
		Text string `json:"text"`
	} `json:"templateData"`
	Args struct {
		Code string `json:"code"`
	} `json:"args"`
}

func parseZitadelSMSPayload(body []byte) (zitadelSMSPayload, bool) {
	var payload zitadelSMSPayload
	if json.Unmarshal(body, &payload) != nil ||
		!isE164(payload.ContextInfo.RecipientPhoneNumber) ||
		!approvedEventType(payload.ContextInfo.EventType) ||
		strings.TrimSpace(payload.TemplateData.Text) == "" ||
		strings.TrimSpace(payload.Args.Code) == "" || len(payload.Args.Code) > 256 {
		return zitadelSMSPayload{}, false
	}
	return payload, true
}

func approvedEventType(eventType string) bool {
	return eventType == "user.human.phone.code.added" ||
		eventType == "user.human.initialization.code.added"
}

func isE164(phone string) bool {
	if len(phone) < 3 || len(phone) > 16 || phone[0] != '+' || phone[1] < '1' || phone[1] > '9' {
		return false
	}
	for _, digit := range phone[2:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

// validateZitadelSignature is a narrow, source-compatible adaptation of
// github.com/zitadel/zitadel/pkg/actions/signing.go at commit
// 83854e80e0b3f90244f6bd837a5eb40d4849d4de (Apache-2.0). ZITADEL does not
// publish that package in a stable module; embedding this small protocol parser
// avoids importing its unreleased monorepo dependency graph.
func validateZitadelSignature(payload []byte, header, signingKey string, now time.Time) bool {
	if strings.TrimSpace(signingKey) == "" {
		return false
	}
	timestamp, signatures, ok := parseZitadelSignatureHeader(header)
	skew := now.Sub(timestamp)
	if !ok || skew > zitadelSignatureTolerance || skew < -zitadelSignatureTolerance {
		return false
	}

	mac := hmac.New(sha256.New, []byte(signingKey))
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp.Unix(), 10)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)
	for _, signature := range signatures {
		if hmac.Equal(expected, signature) {
			return true
		}
	}
	return false
}

func parseZitadelSignatureHeader(header string) (time.Time, [][]byte, bool) {
	if header == "" {
		return time.Time{}, nil, false
	}
	var timestamp time.Time
	var signatures [][]byte
	for _, pair := range strings.Split(header, ",") {
		parts := strings.Split(pair, "=")
		if len(parts) != 2 {
			return time.Time{}, nil, false
		}
		switch parts[0] {
		case "t":
			unix, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return time.Time{}, nil, false
			}
			timestamp = time.Unix(unix, 0)
		case "v1":
			signature, err := hex.DecodeString(parts[1])
			if err == nil {
				signatures = append(signatures, signature)
			}
		}
	}
	return timestamp, signatures, len(signatures) > 0
}

// NewTencentSender creates the production Sender using Tencent Cloud's
// official Go SDK. The credentials remain in memory only and are never logged.
func NewTencentSender(secretID, secretKey string) (Sender, error) {
	if strings.TrimSpace(secretID) == "" || strings.TrimSpace(secretKey) == "" {
		return nil, ErrInvalidConfiguration
	}
	client, err := sms.NewClient(common.NewCredential(secretID, secretKey), tencentSMSRegion, profile.NewClientProfile())
	if err != nil {
		return nil, ErrInvalidConfiguration
	}
	return &tencentSender{client: client}, nil
}

type tencentSender struct {
	client *sms.Client
}

func (s *tencentSender) Send(_ context.Context, message Message) error {
	request := sms.NewSendSmsRequest()
	request.PhoneNumberSet = []*string{common.StringPtr(message.Phone)}
	request.TemplateID = common.StringPtr(message.TemplateID)
	request.SmsSdkAppid = common.StringPtr(message.AppID)
	request.Sign = common.StringPtr(message.SignName)
	request.TemplateParamSet = common.StringPtrs(message.Params)

	response, err := s.client.SendSms(request)
	if err != nil || response == nil || response.Response == nil || len(response.Response.SendStatusSet) != 1 ||
		response.Response.SendStatusSet[0] == nil || response.Response.SendStatusSet[0].Code == nil || *response.Response.SendStatusSet[0].Code != "Ok" {
		return ErrDeliveryFailed
	}
	return nil
}
