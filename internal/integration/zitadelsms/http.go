package zitadelsms

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	corelogger "task-processor/internal/core/logger"
)

type Handler struct{ Service *Service }

const MaxBodyBytes int64 = 64 * 1024

// Deliver accepts callbacks from ZITADEL's HTTP SMS provider. This
// route intentionally bypasses bearer authentication because it authenticates
// the complete bounded raw payload using ZITADEL's webhook signature instead.
func (h Handler) Deliver(c *gin.Context) {
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
	case errors.Is(err, ErrUnauthorizedWebhook):
		c.Status(http.StatusUnauthorized)
	case errors.Is(err, ErrInvalidPayload):
		c.Status(http.StatusBadRequest)
	case errors.Is(err, ErrInvalidConfiguration):
		c.Status(http.StatusServiceUnavailable)
	case errors.Is(err, ErrDeliveryFailed):
		var failure *DeliveryFailure
		if errors.As(err, &failure) {
			corelogger.GetGlobalLogger("zitadel-sms").WithField("provider_code", failure.Code).Warn("Tencent SMS delivery failed")
		}
		c.Status(http.StatusBadGateway)
	default:
		c.Status(http.StatusBadGateway)
	}
}

func readZitadelSMSWebhookBody(c *gin.Context) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes))
	if err != nil {
		return nil, false
	}
	return body, true
}
