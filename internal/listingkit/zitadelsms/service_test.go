package zitadelsms

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20190711"
)

type senderStub struct {
	messages []Message
	err      error
}

func (s *senderStub) Send(_ context.Context, message Message) error {
	s.messages = append(s.messages, message)
	return s.err
}

func TestDeliverRejectsInvalidOrStaleSignatureWithoutCallingTencent(t *testing.T) {
	for _, signature := range []string{
		"t=1,v1=bad",
		signedHeader(t, []byte("{}"), time.Now().Add(-6*time.Minute)),
	} {
		sender := &senderStub{}
		service := newTestSMSService(t, sender)

		err := service.Deliver(context.Background(), []byte("{}"), signature)

		require.ErrorIs(t, err, ErrUnauthorizedWebhook)
		require.Empty(t, sender.messages)
	}
}

func TestDeliverRejectsFutureSignatureWithoutCallingTencent(t *testing.T) {
	sender := &senderStub{}
	service := newTestSMSService(t, sender)
	body := validZitadelSMSPayload(t, "+8613800138000", "123456", "user.human.phone.code.added")

	err := service.Deliver(context.Background(), body, signedHeader(t, body, time.Now().Add(6*time.Minute)))

	require.ErrorIs(t, err, ErrUnauthorizedWebhook)
	require.Empty(t, sender.messages)
}

func TestDeliverMapsEveryApprovedEventToTencent(t *testing.T) {
	// The two OTP additions are Core v4.17.1 event contracts (human_mfa_otp.go
	// and session.go). Login V2 compatibility remains bounded to v4.17.1 and is
	// a deployment verification prerequisite, not an inference from this test.
	for _, eventType := range []string{
		"user.human.phone.code.added",
		"user.human.initialization.code.added",
		"user.human.mfa.otp.sms.code.added",
		"session.otp.sms.challenged",
	} {
		t.Run(eventType, func(t *testing.T) {
			sender := &senderStub{}
			service := newTestSMSService(t, sender)
			body := validZitadelSMSPayload(t, "+8613800138000", "123456", eventType)

			err := service.Deliver(context.Background(), body, signedHeader(t, body, time.Now()))

			require.NoError(t, err)
			require.Len(t, sender.messages, 1)
		})
	}
}

func TestDeliverAcceptsSMSChallengeWithoutLocalizedTemplateText(t *testing.T) {
	sender := &senderStub{}
	service := newTestSMSService(t, sender)
	body := []byte(`{"contextInfo":{"recipientPhoneNumber":"+8613800138000","eventType":"session.otp.sms.challenged"},"templateData":{},"args":{"code":"123456"}}`)

	err := service.Deliver(context.Background(), body, signedHeader(t, body, time.Now()))

	require.NoError(t, err)
	require.Equal(t, []string{"123456"}, sender.messages[0].Params)
}

func TestDeliverRejectsNearMatchOTPSMSEventsWithoutSending(t *testing.T) {
	for _, eventType := range []string{
		"user.human.mfa.otp.sms.code.sent",
		"session.otp.sms.checked",
		"user.human.mfa.otp.sms.code.added.extra",
	} {
		t.Run(eventType, func(t *testing.T) {
			sender := &senderStub{}
			service := newTestSMSService(t, sender)
			body := validZitadelSMSPayload(t, "+8613800138000", "123456", eventType)

			err := service.Deliver(context.Background(), body, signedHeader(t, body, time.Now()))

			require.ErrorIs(t, err, ErrInvalidPayload)
			require.Empty(t, sender.messages)
		})
	}
}

func TestDeliverMapsVerifiedPayloadToConfiguredTencentTemplate(t *testing.T) {
	sender := &senderStub{}
	service := newTestSMSService(t, sender)
	body := validZitadelSMSPayload(t, "+8613800138000", "123456", "user.human.phone.code.added")

	err := service.Deliver(context.Background(), body, signedHeader(t, body, time.Now()))

	require.NoError(t, err)
	require.Equal(t, []Message{{
		Phone:      "+8613800138000",
		TemplateID: "template-id",
		SignName:   "sign-name",
		AppID:      "app-id",
		Params:     []string{"123456"},
	}}, sender.messages)
}

func TestDeliverRejectsMalformedOrUnapprovedVerifiedPayloadWithoutSending(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{name: "malformed JSON", body: []byte("{")},
		{name: "non E164 recipient", body: validZitadelSMSPayload(t, "13800138000", "123456", "user.human.phone.code.added")},
		{name: "unapproved event", body: validZitadelSMSPayload(t, "+8613800138000", "123456", "user.human.password.code.added")},
		{name: "missing code", body: validZitadelSMSPayload(t, "+8613800138000", "", "user.human.phone.code.added")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sender := &senderStub{}
			service := newTestSMSService(t, sender)

			err := service.Deliver(context.Background(), test.body, signedHeader(t, test.body, time.Now()))

			require.ErrorIs(t, err, ErrInvalidPayload)
			require.Empty(t, sender.messages)
			require.NotContains(t, err.Error(), "+8613800138000")
			require.NotContains(t, err.Error(), "123456")
		})
	}
}

func TestDeliverRedactsTencentFailure(t *testing.T) {
	sender := &senderStub{err: errors.New("Tencent failed for +8613800138000 with code 123456")}
	service := newTestSMSService(t, sender)
	body := validZitadelSMSPayload(t, "+8613800138000", "123456", "user.human.initialization.code.added")

	err := service.Deliver(context.Background(), body, signedHeader(t, body, time.Now()))

	require.ErrorIs(t, err, ErrDeliveryFailed)
	require.NotContains(t, err.Error(), "+8613800138000")
	require.NotContains(t, err.Error(), "123456")
}

func TestTencentSenderPropagatesContextCancellationToSDKRequest(t *testing.T) {
	clientProfile := profile.NewClientProfile()
	clientProfile.HttpProfile.Endpoint = "sms.test"
	clientProfile.HttpProfile.Scheme = "http"
	client, err := sms.NewClient(common.NewCredential("secret-id", "secret-key"), tencentSMSRegion, clientProfile)
	require.NoError(t, err)

	started := make(chan struct{})
	var observedContext context.Context
	transport := smsRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		observedContext = request.Context()
		close(started)
		select {
		case <-request.Context().Done():
			return nil, request.Context().Err()
		case <-time.After(250 * time.Millisecond):
			return nil, errors.New("request context was not propagated")
		}
	})

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), smsContextProbeKey{}, "propagated"))
	defer cancel()
	sender := &tencentSender{clientFactory: func(ctx context.Context) (*sms.Client, error) {
		client.WithHttpTransport(contextRoundTripper{ctx: ctx, base: transport})
		return client, nil
	}}
	done := make(chan error, 1)
	go func() {
		done <- sender.Send(ctx, Message{
			Phone:      "+8613800138000",
			TemplateID: "template-id",
			SignName:   "sign-name",
			AppID:      "app-id",
			Params:     []string{"123456"},
		})
	}()

	select {
	case <-started:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("Tencent SDK request did not start")
	}

	require.ErrorIs(t, <-done, ErrDeliveryFailed)
	require.Equal(t, "propagated", observedContext.Value(smsContextProbeKey{}))
	require.ErrorIs(t, observedContext.Err(), context.Canceled)
}

func TestNewServiceRejectsIncompleteConfiguration(t *testing.T) {
	_, err := NewService(Config{}, &senderStub{})
	require.ErrorIs(t, err, ErrInvalidConfiguration)
}

func TestValidateZitadelSignatureMatchesUpstreamProtocolVector(t *testing.T) {
	payload := []byte(`{"z":"payload"}`)
	header := "t=1700000000,v1=5b0fb27248424ddaef7bc73ac35a48bf57693dba48d5da3e69b52d4efda65642"

	valid := validateZitadelSignature(payload, header, "test-signing-key", time.Unix(1700000000, 0))

	require.True(t, valid)
}

func newTestSMSService(t *testing.T, sender Sender) *Service {
	t.Helper()
	service, err := NewService(Config{
		SigningKey: "test-signing-key",
		TemplateID: "template-id",
		SignName:   "sign-name",
		AppID:      "app-id",
	}, sender)
	require.NoError(t, err)
	return service
}

type smsRoundTripperFunc func(*http.Request) (*http.Response, error)

type smsContextProbeKey struct{}

func (f smsRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func validZitadelSMSPayload(t *testing.T, phone, code, eventType string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"contextInfo": map[string]any{
			"recipientPhoneNumber": phone,
			"eventType":            eventType,
		},
		"templateData": map[string]any{
			"text": "verification notification",
		},
		"args": map[string]any{
			"code": code,
		},
	})
	require.NoError(t, err)
	return body
}

func signedHeader(t *testing.T, body []byte, timestamp time.Time) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte("test-signing-key"))
	_, err := fmt.Fprintf(mac, "%d.", timestamp.Unix())
	require.NoError(t, err)
	_, err = mac.Write(body)
	require.NoError(t, err)
	return strings.Join([]string{
		fmt.Sprintf("t=%d", timestamp.Unix()),
		"v1=" + hex.EncodeToString(mac.Sum(nil)),
	}, ",")
}
