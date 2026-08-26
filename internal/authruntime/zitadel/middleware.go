package zitadel

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	identity, err := m.verifyToken(c.Request, token)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "zitadel_token_invalid",
			"message": err.Error(),
		})
		return
	}

	tenantID := strings.TrimSpace(identity.ResourceID)
	userID := strings.TrimSpace(identity.Subject)
	if tenantID == "" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "zitadel_tenant_missing",
			"message": "ZITADEL resource owner is required",
		})
		return
	}
	if userID == "" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "zitadel_user_missing",
			"message": "ZITADEL subject is required",
		})
		return
	}

	trustedIdentity := authidentity.AuthenticatedIdentity{
		TenantID: tenantID,
		UserID:   userID,
		Roles:    append([]string(nil), identity.Roles...),
	}
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
	c.Request.Header.Set("X-Tenant-ID", tenantID)
	c.Request.Header.Set("tenant-id", tenantID)
	c.Request.Header.Set("X-User-ID", userID)
	c.Request.Header.Set("X-User-Type", "zitadel")
	if len(trustedIdentity.Roles) > 0 {
		c.Request.Header.Set("X-User-Roles", strings.Join(trustedIdentity.Roles, ","))
	}

	if m.authzCfg.Required {
		if ok, reason := authorizeIdentity(identity, m.authzCfg); !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "zitadel_access_denied",
				"message": reason,
			})
			return
		}
	}

	c.Next()
}

func (m *middleware) verifyToken(r *http.Request, token string) (*IntrospectionResponse, error) {
	discovery, err := m.getDiscovery(r)
	if err != nil {
		return nil, err
	}
	if discovery.IntrospectionEndpoint == "" {
		return nil, errors.New("ZITADEL introspection endpoint is not available")
	}

	form := url.Values{}
	form.Set("token", token)
	form.Set("token_type_hint", "access_token")

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, discovery.IntrospectionEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if m.cfg.ClientSecret != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(m.cfg.ClientID+":"+m.cfg.ClientSecret)))
	}

	resp, err := m.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ZITADEL token introspection failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ZITADEL token introspection response is invalid: %w", err)
	}

	var payload IntrospectionResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("ZITADEL token introspection response is invalid: %w", err)
	}
	payload.Extra = data
	payload.Roles = ParseRoles(data)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ZITADEL token introspection failed: %d", resp.StatusCode)
	}
	if !payload.Active {
		return nil, errors.New("ZITADEL token introspection returned an inactive token; check whether the UI and API are using the same ZITADEL issuer/client configuration")
	}
	return &payload, nil
}

func (m *middleware) getDiscovery(r *http.Request) (discoveryDocument, error) {
	m.mu.Lock()
	cached := m.discovery
	m.mu.Unlock()
	if cached.IntrospectionEndpoint != "" {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, m.cfg.IssuerURL+"/.well-known/openid-configuration", nil)
	if err != nil {
		return discoveryDocument{}, err
	}

	resp, err := m.cfg.HTTPClient.Do(req)
	if err != nil {
		return discoveryDocument{}, fmt.Errorf("ZITADEL discovery failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return discoveryDocument{}, fmt.Errorf("ZITADEL discovery failed: %d", resp.StatusCode)
	}

	var discovery discoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return discoveryDocument{}, fmt.Errorf("ZITADEL discovery response is invalid: %w", err)
	}

	m.mu.Lock()
	m.discovery = discovery
	m.mu.Unlock()
	return discovery, nil
}

func bearerToken(authorization string) string {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func authorizeIdentity(identity *IntrospectionResponse, cfg AuthorizationConfig) (bool, string) {
	if cfg.LegacyUsernameAllowlistConfigured {
		return false, "ZITADEL username allowlists are obsolete; configure canonical allowlists"
	}
	if identity == nil {
		return false, "ZITADEL identity is missing"
	}
	if len(cfg.AllowedTenantIDs) == 0 && len(cfg.AllowedUserIDs) == 0 && len(cfg.AllowedRoles) == 0 {
		return false, "ZITADEL authorization is required but no allowlist is configured"
	}
	if valueInSet(firstNonEmptyValue(identity.ResourceID), cfg.AllowedTenantIDs) {
		return true, ""
	}
	if valueInSet(identity.Subject, cfg.AllowedUserIDs) {
		return true, ""
	}
	for _, role := range identity.Roles {
		if valueInSet(role, cfg.AllowedRoles) {
			return true, ""
		}
	}
	return false, "ZITADEL identity is not allowed to access ListingKit"
}
