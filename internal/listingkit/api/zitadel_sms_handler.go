package api

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit/zitadelsms"
)

const zitadelSMSWebhookMaxBodyBytes int64 = 64 * 1024

// DeliverZitadelSMS accepts callbacks from ZITADEL's HTTP SMS provider. This
// route intentionally bypasses bearer authentication because it authenticates
// the complete bounded raw payload using ZITADEL's webhook signature instead.
func (h *handler) DeliverZitadelSMS(c *gin.Context) {
	body, ok := readZitadelSMSWebhookBody(c)
	if !ok {
		c.Status(http.StatusRequestEntityTooLarge)
		return
	}
	if h.zitadelSMSService == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}

	err := h.zitadelSMSService.Deliver(c.Request.Context(), body, c.GetHeader("ZITADEL-Signature"))
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, zitadelsms.ErrUnauthorizedWebhook), errors.Is(err, zitadelsms.ErrInvalidPayload):
		c.Status(http.StatusUnauthorized)
	case errors.Is(err, zitadelsms.ErrInvalidConfiguration):
		c.Status(http.StatusServiceUnavailable)
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
