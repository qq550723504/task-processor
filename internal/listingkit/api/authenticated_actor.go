package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func authenticatedActor(c *gin.Context) (listingkit.AuthenticatedIdentity, bool) {
	if c == nil || c.Request == nil {
		return listingkit.AuthenticatedIdentity{}, false
	}
	identity, ok := listingkit.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "zitadel_user_missing",
			"message": "authenticated ZITADEL subject is required",
		})
		return listingkit.AuthenticatedIdentity{}, false
	}
	return identity, true
}
