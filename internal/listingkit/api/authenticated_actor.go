package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authidentity"
)

func authenticatedActor(c *gin.Context) (authidentity.AuthenticatedIdentity, bool) {
	if c == nil || c.Request == nil {
		return authidentity.AuthenticatedIdentity{}, false
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "zitadel_user_missing",
			"message": "authenticated ZITADEL subject is required",
		})
		return authidentity.AuthenticatedIdentity{}, false
	}
	return identity, true
}
