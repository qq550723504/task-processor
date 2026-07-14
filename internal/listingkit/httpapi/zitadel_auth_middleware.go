package httpapi

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

	"task-processor/internal/listingkit"
)

func (m *zitadelAuthMiddleware) Handle(c *gin.Context) {
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
	trustedIdentity := listingkit.AuthenticatedIdentity{
		TenantID: identity.ResourceID,
		UserID:   firstNonEmptyZitadelValue(identity.UserID, identity.Subject, identity.Username),
		Roles:    identity.Roles,
	}
	if strings.TrimSpace(trustedIdentity.TenantID) == "" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "zitadel_tenant_missing",
			"message": "ZITADEL identity has no tenant",
		})
		return
	}
	c.Request = c.Request.WithContext(listingkit.WithAuthenticatedIdentity(c.Request.Context(), trustedIdentity))

	if identity.ResourceID != "" {
		c.Request.Header.Set("X-Tenant-ID", identity.ResourceID)
		c.Request.Header.Set("tenant-id", identity.ResourceID)
	}
	if userID := firstNonEmptyZitadelValue(identity.UserID, identity.Subject, identity.Username); userID != "" {
		c.Request.Header.Set("X-User-ID", userID)
		c.Request.Header.Set("X-User-Type", "zitadel")
	}
	if len(identity.Roles) > 0 {
		c.Request.Header.Set("X-User-Roles", strings.Join(identity.Roles, ","))
	}
	if m.authz.Required {
		if ok, reason := authorizeZitadelIdentity(identity, m.authz); !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "zitadel_access_denied",
				"message": reason,
			})
			return
		}
	}
	c.Next()
}

func (m *zitadelAuthMiddleware) verifyToken(r *http.Request, token string) (*zitadelIntrospectionResponse, error) {
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
	var payload zitadelIntrospectionResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("ZITADEL token introspection response is invalid: %w", err)
	}
	payload.Extra = data
	payload.Roles = parseZitadelRoles(data)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ZITADEL token introspection failed: %d", resp.StatusCode)
	}
	if !payload.Active {
		return nil, errors.New(
			"ZITADEL token introspection returned an inactive token; check whether the UI and API are using the same ZITADEL issuer/client configuration",
		)
	}
	return &payload, nil
}

func (m *zitadelAuthMiddleware) getDiscovery(r *http.Request) (zitadelDiscovery, error) {
	m.mu.Lock()
	cached := m.discovery
	m.mu.Unlock()
	if cached.IntrospectionEndpoint != "" {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, m.cfg.IssuerURL+"/.well-known/openid-configuration", nil)
	if err != nil {
		return zitadelDiscovery{}, err
	}
	resp, err := m.cfg.HTTPClient.Do(req)
	if err != nil {
		return zitadelDiscovery{}, fmt.Errorf("ZITADEL discovery failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zitadelDiscovery{}, fmt.Errorf("ZITADEL discovery failed: %d", resp.StatusCode)
	}
	var discovery zitadelDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return zitadelDiscovery{}, fmt.Errorf("ZITADEL discovery response is invalid: %w", err)
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
