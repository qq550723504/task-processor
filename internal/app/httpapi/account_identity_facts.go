package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"time"
)

type accountIdentityFactsReader interface {
	ReadUserFacts(context.Context, string, string) (zitadel.SelfUserFacts, error)
}

func (m accountIdentityModule) readFacts(c *gin.Context) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) {
		writeAccountIdentityError(c, 401, "AUTHENTICATION_REQUIRED")
		return
	}
	if !accountProfileReadRequest(c.Request) {
		writeAccountIdentityError(c, 400, "INVALID_REQUEST")
		return
	}
	reader, ok := m.client.(accountIdentityFactsReader)
	if !ok {
		writeAccountIdentityError(c, 503, "DEPENDENCY_UNAVAILABLE")
		return
	}
	token := zitadel.BearerTokenFromContext(c.Request.Context())
	if token == "" {
		writeAccountIdentityError(c, 401, "AUTHENTICATION_REQUIRED")
		return
	}
	facts, err := reader.ReadUserFacts(c.Request.Context(), token, identity.UserID)
	if err != nil {
		writeAccountIdentityUpstreamError(c, err, false)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(http.StatusOK, gin.H{"schemaVersion": "account-identity-facts-v1", "userId": identity.UserID, "registeredAt": facts.RegisteredAt, "lastLogin": facts.LastLogin, "passwordChangedAt": facts.PasswordChangedAt, "source": "zitadel_auth_v1", "readAt": time.Now().UTC()})
}
