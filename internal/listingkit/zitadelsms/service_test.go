package zitadelsms

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
