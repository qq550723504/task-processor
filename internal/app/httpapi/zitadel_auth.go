package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type zitadelAuthConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	Required     bool
	HTTPClient   *http.Client
}

type zitadelDiscovery struct {
	IntrospectionEndpoint string `json:"introspection_endpoint"`
}

type zitadelIntrospectionResponse struct {
	Active     bool            `json:"active"`
	Subject    string          `json:"sub"`
	Username   string          `json:"username"`
	UserID     string          `json:"user_id"`
	ResourceID string          `json:"urn:zitadel:iam:user:resourceowner:id"`
	Roles      []string        `json:"-"`
	Extra      json.RawMessage `json:"-"`
}

type zitadelAuthMiddleware struct {
	cfg       zitadelAuthConfig
	mu        sync.Mutex
	discovery zitadelDiscovery
}

func newListingKitZitadelAuthMiddlewareFromEnv() gin.HandlerFunc {
	cfg := zitadelAuthConfig{
		IssuerURL:    strings.TrimRight(strings.TrimSpace(os.Getenv("ZITADEL_ISSUER_URL")), "/"),
		ClientID:     strings.TrimSpace(os.Getenv("ZITADEL_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("ZITADEL_CLIENT_SECRET")),
		Required:     envBool("LISTINGKIT_ZITADEL_AUTH_REQUIRED") || envBool("TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTH_REQUIRED"),
		HTTPClient:   &http.Client{Timeout: 5 * time.Second},
	}
	if cfg.IssuerURL == "" && cfg.ClientID == "" && !cfg.Required {
		return nil
	}
	return newListingKitZitadelAuthMiddleware(cfg).Handle
}

func newListingKitZitadelAuthMiddleware(cfg zitadelAuthConfig) *zitadelAuthMiddleware {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	cfg.IssuerURL = strings.TrimRight(strings.TrimSpace(cfg.IssuerURL), "/")
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	return &zitadelAuthMiddleware{cfg: cfg}
}

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

	if identity.ResourceID != "" {
		c.Request.Header.Set("X-Tenant-ID", identity.ResourceID)
		c.Request.Header.Set("tenant-id", identity.ResourceID)
	}
	if userID := firstNonEmptyZitadelValue(identity.Subject, identity.UserID, identity.Username); userID != "" {
		c.Request.Header.Set("X-User-ID", userID)
		c.Request.Header.Set("X-User-Type", "zitadel")
	}
	if len(identity.Roles) > 0 {
		c.Request.Header.Set("X-User-Roles", strings.Join(identity.Roles, ","))
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !payload.Active {
		return nil, fmt.Errorf("ZITADEL token introspection failed: %d", resp.StatusCode)
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

func listingKitRouteRequiresZitadelAuth(route routeDescriptor) bool {
	return route.Module == "listing-kit" || route.Module == "listing-kit-admin" || route.Module == "listing-kit-platform-admin" || route.Module == "listing-kit-studio" || route.Module == "shein-login"
}

func bearerToken(authorization string) string {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func firstNonEmptyZitadelValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseZitadelRoles(data []byte) []string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	roles := []string{}
	add := func(value string) {
		role := strings.TrimSpace(value)
		if role == "" {
			return
		}
		if _, ok := seen[role]; ok {
			return
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}
	for _, key := range []string{"urn:zitadel:iam:org:project:roles", "roles", "role"} {
		value, ok := raw[key]
		if !ok {
			continue
		}
		var list []string
		if err := json.Unmarshal(value, &list); err == nil {
			for _, role := range list {
				add(role)
			}
			continue
		}
		var single string
		if err := json.Unmarshal(value, &single); err == nil {
			for _, role := range strings.Split(single, ",") {
				add(role)
			}
			continue
		}
		var roleMap map[string]any
		if err := json.Unmarshal(value, &roleMap); err == nil {
			for role := range roleMap {
				add(role)
			}
		}
	}
	return roles
}
