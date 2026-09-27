package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *handler) DeliverZitadelSMS(c *gin.Context) {
	if h.zitadelSMSHandler == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	h.zitadelSMSHandler(c)
}
