package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetAuthContext returns the identity verified by the ZITADEL middleware.
// It intentionally omits billing configuration and all token material.
func (h *handler) GetAuthContext(c *gin.Context) {
	identity, ok := authenticatedActor(c)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tenant_id": identity.TenantID,
		"user_id":   identity.UserID,
		"roles":     identity.Roles,
	})
}
