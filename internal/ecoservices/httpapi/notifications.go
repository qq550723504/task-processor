package httpapi

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

const NotifyPath = "/api/v1/payments/ecoservices/wechat/notify"

func (h *Handler) notify(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h == nil || h.notifications == nil || h.notifications.AcceptNotification(c.Request) != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"code": "FAIL", "message": "notification was not durably accepted"})
		return
	}
	c.Status(http.StatusNoContent)
}
