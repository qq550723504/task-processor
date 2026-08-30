package zitadel

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authidentity"
)

func (m *middleware) Handle(c *gin.Context) {
	if m.cfg.IssuerURL == "" || m.cfg.ClientID == "" {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error":   "zitadel_auth_not_configured",
			"message": "ZITADEL authentication is not configured",
		})
		return
	}

	token := bearerToken(c.GetHeader("Authorization"))
	if token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "zitadel_token_missing",
			"message": "Missing ZITADEL bearer token",
		})
		return
	}

	identity, err := m.verifier.Verify(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, errResourceOwnerMissing) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "zitadel_tenant_missing",
				"message": err.Error(),
			})
			return
		}
		if errors.Is(err, errSubjectMissing) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "zitadel_user_missing",
				"message": err.Error(),
			})
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "zitadel_token_invalid",
			"message": err.Error(),
		})
		return
	}

	trustedIdentity := identity
	for _, header := range []string{
		"X-User-ID",
		"X-User-Type",
		"X-User-Roles",
		"X-Zitadel-Roles",
		"X-User",
		"X-Tenant-ID",
		"tenant-id",
		"X-Tenant",
	} {
		c.Request.Header.Del(header)
	}

	c.Request = c.Request.WithContext(authidentity.WithAuthenticatedIdentity(c.Request.Context(), trustedIdentity))
	c.Request.Header.Set("X-Tenant-ID", identity.TenantID)
	c.Request.Header.Set("tenant-id", identity.TenantID)
	c.Request.Header.Set("X-User-ID", identity.UserID)
	c.Request.Header.Set("X-User-Type", "zitadel")
	if len(trustedIdentity.Roles) > 0 {
		c.Request.Header.Set("X-User-Roles", strings.Join(trustedIdentity.Roles, ","))
	}

	if m.authzCfg.Required {
		if ok, reason := authorizeIdentity(&identity, m.authzCfg); !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "zitadel_access_denied",
				"message": reason,
			})
			return
		}
	}

	c.Next()
}

func bearerToken(authorization string) string {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func authorizeIdentity(identity *authidentity.AuthenticatedIdentity, cfg AuthorizationConfig) (bool, string) {
	if cfg.LegacyUsernameAllowlistConfigured {
		return false, "ZITADEL username allowlists are obsolete; configure canonical allowlists"
	}
	if identity == nil {
		return false, "ZITADEL identity is missing"
	}
	if len(cfg.AllowedTenantIDs) == 0 && len(cfg.AllowedUserIDs) == 0 && len(cfg.AllowedRoles) == 0 {
		return false, "ZITADEL authorization is required but no allowlist is configured"
	}
	if valueInSet(firstNonEmptyValue(identity.TenantID), cfg.AllowedTenantIDs) {
		return true, ""
	}
	if valueInSet(identity.UserID, cfg.AllowedUserIDs) {
		return true, ""
	}
	for _, role := range identity.Roles {
		if valueInSet(role, cfg.AllowedRoles) {
			return true, ""
		}
	}
	return false, "ZITADEL identity is not allowed to access ListingKit"
}
