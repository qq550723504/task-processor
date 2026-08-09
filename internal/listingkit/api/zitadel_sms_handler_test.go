package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/listingkit/zitadelsms"
)

const zitadelSMSWebhookPath = "/api/v1/listing-kits/integrations/zitadel/sms"

func TestZitadelSMSWebhookRejectsUnsignedRequestWithoutSending(t *testing.T) {
	t.Parallel()

	sender := &zitadelSMSSenderStub{}
	router := newZitadelSMSWebhookRouter(t, newZitadelSMSService(t, sender))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, zitadelSMSWebhookPath, strings.NewReader(validZitadelSMSWebhookBody)))

	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Zero(t, sender.calls)
}

func TestZitadelSMSWebhookDeliversValidSignedRequest(t *testing.T) {
	t.Parallel()

	sender := &zitadelSMSSenderStub{}
	router := newZitadelSMSWebhookRouter(t, newZitadelSMSService(t, sender))
	body := []byte(validZitadelSMSWebhookBody)
	request := httptest.NewRequest(http.MethodPost, zitadelSMSWebhookPath, strings.NewReader(string(body)))
	request.Header.Set("ZITADEL-Signature", signedZitadelSMSWebhookHeader(body, "test-signing-key", time.Now()))
	request.Header.Set("Authorization", "Bearer attacker-controlled")
	request.Header.Set("X-User-ID", "attacker-controlled")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, 1, sender.calls)
}

func TestZitadelSMSWebhookRejectsTooLargeBody(t *testing.T) {
	t.Parallel()

	sender := &zitadelSMSSenderStub{}
	router := newZitadelSMSWebhookRouter(t, newZitadelSMSService(t, sender))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, zitadelSMSWebhookPath, strings.NewReader(strings.Repeat("x", int(zitadelSMSWebhookMaxBodyBytes+1)))))

	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	require.Zero(t, sender.calls)
}

func TestZitadelSMSWebhookFailsClosedWhenUnconfigured(t *testing.T) {
	t.Parallel()

	router := newZitadelSMSWebhookRouter(t, nil)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, zitadelSMSWebhookPath, strings.NewReader(validZitadelSMSWebhookBody)))

	require.Equal(t, http.StatusServiceUnavailable, response.Code)
}

func TestZitadelSMSWebhookMapsProviderFailureToBadGateway(t *testing.T) {
	t.Parallel()

	sender := &zitadelSMSSenderStub{err: fmt.Errorf("provider unavailable")}
	router := newZitadelSMSWebhookRouter(t, newZitadelSMSService(t, sender))
	body := []byte(validZitadelSMSWebhookBody)
	request := httptest.NewRequest(http.MethodPost, zitadelSMSWebhookPath, strings.NewReader(string(body)))
	request.Header.Set("ZITADEL-Signature", signedZitadelSMSWebhookHeader(body, "test-signing-key", time.Now()))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadGateway, response.Code)
	require.Equal(t, 1, sender.calls)
}

func newZitadelSMSWebhookRouter(t *testing.T, service *zitadelsms.Service) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubHandlerCoreService{}, WithZitadelSMSService(service))
	require.NoError(t, err)
	router := gin.New()
	router.POST(zitadelSMSWebhookPath, h.DeliverZitadelSMS)
	return router
}

func newZitadelSMSService(t *testing.T, sender zitadelsms.Sender) *zitadelsms.Service {
	t.Helper()
	service, err := zitadelsms.NewService(zitadelsms.Config{
		SigningKey: "test-signing-key",
		TemplateID: "100001",
		SignName:   "ListingKit",
		AppID:      "1234567890",
	}, sender)
	require.NoError(t, err)
	return service
}

func signedZitadelSMSWebhookHeader(body []byte, signingKey string, now time.Time) string {
	timestamp := now.Unix()
	mac := hmac.New(sha256.New, []byte(signingKey))
	_, _ = mac.Write([]byte(fmt.Sprintf("%d.", timestamp)))
	_, _ = mac.Write(body)
	return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

type zitadelSMSSenderStub struct {
	calls int
	err   error
}

func (s *zitadelSMSSenderStub) Send(context.Context, zitadelsms.Message) error {
	s.calls++
	return s.err
}

const validZitadelSMSWebhookBody = `{"contextInfo":{"recipientPhoneNumber":"+8613800138000","eventType":"user.human.phone.code.added"},"templateData":{"text":"Your code"},"args":{"code":"123456"}}`
