package httpapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	corelogger "task-processor/internal/core/logger"
	"task-processor/internal/integration/zitadelsms"
)

type zitadelSMSHandler struct{ Service *zitadelsms.Service }

const zitadelSMSWebhookMaxBodyBytes int64 = 64 * 1024

// Deliver accepts callbacks from ZITADEL's HTTP SMS provider. This
// route intentionally bypasses bearer authentication because it authenticates
// the complete bounded raw payload using ZITADEL's webhook signature instead.
// Before activation, match ZITADEL's phone generator to Tencent's verification
// template: at most six ASCII digits, no letters/symbols, and matching expiry.
// Letter-bearing codes fail with TemplateParameterFormatError even if approved.
// The relay must preserve the original code; changing it breaks verification.
// Deployment checks: deployments/docker/account-compose/SMS.md.
func (h zitadelSMSHandler) Deliver(c *gin.Context) {
	body, ok := readZitadelSMSWebhookBody(c)
	if !ok {
		c.Status(http.StatusRequestEntityTooLarge)
		return
	}
	if h.Service == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}

	err := h.Service.Deliver(c.Request.Context(), body, c.GetHeader("ZITADEL-Signature"))
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, zitadelsms.ErrUnauthorizedWebhook):
		c.Status(http.StatusUnauthorized)
	case errors.Is(err, zitadelsms.ErrInvalidPayload):
		c.Status(http.StatusBadRequest)
	case errors.Is(err, zitadelsms.ErrInvalidConfiguration):
		c.Status(http.StatusServiceUnavailable)
	case errors.Is(err, zitadelsms.ErrDeliveryFailed):
		var failure *zitadelsms.DeliveryFailure
		if errors.As(err, &failure) {
			corelogger.GetGlobalLogger("zitadel-sms").WithField("provider_code", failure.Code).Warn("Tencent SMS delivery failed")
		}
		c.Status(http.StatusBadGateway)
	default:
		c.Status(http.StatusBadGateway)
	}
}

func readZitadelSMSWebhookBody(c *gin.Context) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, zitadelSMSWebhookMaxBodyBytes))
	if err != nil {
		return nil, false
	}
	return body, true
}
