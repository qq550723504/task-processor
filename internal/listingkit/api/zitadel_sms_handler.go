package api

import (
	"github.com/gin-gonic/gin"
	"task-processor/internal/integration/zitadelsms"
)

func (h *handler) DeliverZitadelSMS(c *gin.Context) {
	(zitadelsms.Handler{Service: h.zitadelSMSService}).Deliver(c)
}
