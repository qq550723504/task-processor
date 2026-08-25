package zitadelsms

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
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
		Params:     []string{payload.Args.OTP},
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
		// ZITADEL's webhook serializer lowercases only the first character of
		// the "OTP" argument, so the v4.17.1 wire key is "oTP".
		OTP  string `json:"oTP"`
		Code string `json:"code"` // legacy/provider compatibility
	} `json:"args"`
}

func parseZitadelSMSPayload(body []byte) (zitadelSMSPayload, bool) {
	var payload zitadelSMSPayload
	if json.Unmarshal(body, &payload) != nil ||
		!isE164(payload.ContextInfo.RecipientPhoneNumber) ||
		!approvedEventType(payload.ContextInfo.EventType) ||
		!validOTPArgument(&payload) {
		return zitadelSMSPayload{}, false
	}
	if strings.TrimSpace(payload.Args.OTP) == "" {
		payload.Args.OTP = payload.Args.Code
	}
	return payload, true
}

func validOTPArgument(payload *zitadelSMSPayload) bool {
	if payload == nil {
		return false
	}
	value := payload.Args.OTP
	if strings.TrimSpace(value) == "" {
		value = payload.Args.Code
	}
	return strings.TrimSpace(value) != "" && len(value) <= 256
}

// Provenance is pinned to ZITADEL Core v4.17.1: the human MFA event type is
// HumanOTPSMSCodeAddedType in internal/repository/user/human_mfa_otp.go, and
// the session event is emitted by SessionCommands.OTPSMSChallenged in
// internal/command/session.go. These Core contracts are consumed here only for
// the Login V2 v4.17.1 compatibility boundary; enabling this relay requires
// verifying both deployed Core and Login V2 versions, not just matching names.
// References:
// https://github.com/zitadel/zitadel/blob/v4.17.1/internal/repository/user/human_mfa_otp.go
// https://github.com/zitadel/zitadel/blob/v4.17.1/internal/command/session.go#L188-L190
var approvedZitadelSMSEvents = map[string]struct{}{
	"user.human.phone.code.added":          {},
	"user.human.initialization.code.added": {},
	"user.human.mfa.otp.sms.code.added":    {},
	"session.otp.sms.challenged":           {},
}

func approvedEventType(eventType string) bool {
	_, ok := approvedZitadelSMSEvents[eventType]
	return ok
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
	clientFactory := func(ctx context.Context) (*sms.Client, error) {
		client, err := sms.NewClient(common.NewCredential(secretID, secretKey), tencentSMSRegion, profile.NewClientProfile())
		if err != nil {
			return nil, err
		}
		client.WithHttpTransport(contextRoundTripper{ctx: ctx, base: http.DefaultTransport})
		return client, nil
	}
	if _, err := clientFactory(context.Background()); err != nil {
		return nil, ErrInvalidConfiguration
	}
	return &tencentSender{clientFactory: clientFactory}, nil
}

type tencentSender struct {
	clientFactory func(context.Context) (*sms.Client, error)
}

func (s *tencentSender) Send(ctx context.Context, message Message) error {
	if s == nil || s.clientFactory == nil {
		return ErrDeliveryFailed
	}
	client, err := s.clientFactory(ctx)
	if err != nil {
		return ErrDeliveryFailed
	}
	request := sms.NewSendSmsRequest()
	request.PhoneNumberSet = []*string{common.StringPtr(message.Phone)}
	request.TemplateID = common.StringPtr(message.TemplateID)
	request.SmsSdkAppid = common.StringPtr(message.AppID)
	request.Sign = common.StringPtr(message.SignName)
	request.TemplateParamSet = common.StringPtrs(message.Params)

	response, err := client.SendSms(request)
	if err != nil || response == nil || response.Response == nil || len(response.Response.SendStatusSet) != 1 ||
		response.Response.SendStatusSet[0] == nil || response.Response.SendStatusSet[0].Code == nil || *response.Response.SendStatusSet[0].Code != "Ok" {
		return ErrDeliveryFailed
	}
	return nil
}

type contextRoundTripper struct {
	ctx  context.Context
	base http.RoundTripper
}

func (t contextRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if t.ctx != nil {
		request = request.WithContext(t.ctx)
	}
	if t.base == nil {
		t.base = http.DefaultTransport
	}
	return t.base.RoundTrip(request)
}
