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
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
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
	authz     zitadelAuthorizationConfig
	mu        sync.Mutex
	discovery zitadelDiscovery
}

type zitadelAuthorizationConfig struct {
	Required         bool
	AllowedTenantIDs map[string]struct{}
	AllowedUserIDs   map[string]struct{}
	AllowedUsernames map[string]struct{}
	AllowedRoles     map[string]struct{}
}

type listingKitZitadelRuntimeConfig struct {
	AuthConfig  zitadelAuthConfig
	AuthzConfig zitadelAuthorizationConfig
	Authorizer  *authz.ListingKitAuthorizer
}

var (
	listingKitZitadelRuntimeConfigMu sync.RWMutex
	listingKitZitadelRuntimeConfigV  *listingKitZitadelRuntimeConfig
)

func ConfigureListingKitZitadelAuth(cfg config.ListingKitZitadelConfig) {
	authzRequired := len(cfg.AllowedTenantIDs) > 0 ||
		len(cfg.AllowedUserIDs) > 0 ||
		len(cfg.AllowedUsernames) > 0 ||
		len(cfg.AllowedRoles) > 0
	listingKitZitadelRuntimeConfigMu.Lock()
	defer listingKitZitadelRuntimeConfigMu.Unlock()
	listingKitZitadelRuntimeConfigV = &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL:    strings.TrimRight(strings.TrimSpace(cfg.IssuerURL), "/"),
			ClientID:     strings.TrimSpace(cfg.ClientID),
			ClientSecret: strings.TrimSpace(cfg.ClientSecret),
			Required:     cfg.AuthRequired || authzRequired,
			HTTPClient:   &http.Client{Timeout: 5 * time.Second},
		},
		AuthzConfig: zitadelAuthorizationConfig{
			Required:         authzRequired,
			AllowedTenantIDs: stringSliceToSet(cfg.AllowedTenantIDs),
			AllowedUserIDs:   stringSliceToSet(cfg.AllowedUserIDs),
			AllowedUsernames: stringSliceToSet(cfg.AllowedUsernames),
			AllowedRoles:     stringSliceToSet(cfg.AllowedRoles),
		},
	}
}

func SetListingKitZitadelAuthConfigForTesting(cfg *listingKitZitadelRuntimeConfig) func() {
	listingKitZitadelRuntimeConfigMu.Lock()
	previous := listingKitZitadelRuntimeConfigV
	listingKitZitadelRuntimeConfigV = cfg
	listingKitZitadelRuntimeConfigMu.Unlock()
	return func() {
		listingKitZitadelRuntimeConfigMu.Lock()
		listingKitZitadelRuntimeConfigV = previous
		listingKitZitadelRuntimeConfigMu.Unlock()
	}
}

func currentListingKitZitadelRuntimeConfig() *listingKitZitadelRuntimeConfig {
	listingKitZitadelRuntimeConfigMu.RLock()
	defer listingKitZitadelRuntimeConfigMu.RUnlock()
	if listingKitZitadelRuntimeConfigV == nil {
		return nil
	}
	cfg := *listingKitZitadelRuntimeConfigV
	return &cfg
}

func ConfigureListingKitAuthorization(platformAdminUsers []string, platformAdminRoles []string) error {
	listingKitZitadelRuntimeConfigMu.Lock()
	defer listingKitZitadelRuntimeConfigMu.Unlock()

	current := listingKitZitadelRuntimeConfigV
	if current == nil {
		current = &listingKitZitadelRuntimeConfig{}
	}
	authorizer, err := authz.NewListingKitAuthorizer(platformAdminUsers, platformAdminRoles)
	if err != nil {
		return err
	}
	next := *current
	next.Authorizer = authorizer
	listingKitZitadelRuntimeConfigV = &next
	return nil
}

func NewZitadelAuthMiddlewareFromEnv() gin.HandlerFunc {
	runtimeCfg := currentListingKitZitadelRuntimeConfig()
	if runtimeCfg == nil {
		return nil
	}
	if !runtimeCfg.AuthConfig.Required &&
		strings.TrimSpace(runtimeCfg.AuthConfig.IssuerURL) == "" &&
		strings.TrimSpace(runtimeCfg.AuthConfig.ClientID) == "" {
		return nil
	}
	return newListingKitZitadelAuthMiddleware(runtimeCfg.AuthConfig, runtimeCfg.AuthzConfig).Handle
}

func newListingKitZitadelAuthMiddleware(cfg zitadelAuthConfig, authz zitadelAuthorizationConfig) *zitadelAuthMiddleware {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	cfg.IssuerURL = strings.TrimRight(strings.TrimSpace(cfg.IssuerURL), "/")
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	return &zitadelAuthMiddleware{cfg: cfg, authz: authz}
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

func RouteRequiresZitadelAuth(route httproute.Descriptor) bool {
	return route.Module == "listing-kit" ||
		route.Module == "listing-kit-admin" ||
		route.Module == "listing-kit-platform-admin" ||
		route.Module == "listing-kit-studio" ||
		route.Module == "shein-login" ||
		route.Module == "sds" ||
		route.Module == "sds-login"
}

func listingKitRouteRequiredPermission(route httproute.Descriptor) string {
	if value := strings.TrimSpace(route.Permission); value != "" {
		return value
	}
	switch route.Module {
	case "listing-kit-admin":
		if route.Method == http.MethodGet {
			return authz.PermissionListingKitAdminRead
		}
		return authz.PermissionListingKitAdminWrite
	case "listing-kit-platform-admin":
		return authz.PermissionListingKitPlatformAdm
	default:
		return ""
	}
}

func NewRouteRoleMiddleware(route httproute.Descriptor) gin.HandlerFunc {
	requiredPermission := listingKitRouteRequiredPermission(route)
	if requiredPermission == "" {
		return nil
	}
	runtimeCfg := currentListingKitZitadelRuntimeConfig()
	var authorizer *authz.ListingKitAuthorizer
	if runtimeCfg != nil {
		authorizer = runtimeCfg.Authorizer
	}
	if authorizer == nil {
		var err error
		authorizer, err = authz.NewListingKitAuthorizer(nil, nil)
		if err != nil {
			return func(c *gin.Context) {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":   "listingkit_authorization_unavailable",
					"message": "ListingKit authorization is not available",
				})
			}
		}
	}
	return func(c *gin.Context) {
		userRoles := roleHeaderValues(c.GetHeader("X-User-Roles"))
		if len(userRoles) == 0 {
			userRoles = roleHeaderValues(c.GetHeader("X-Zitadel-Roles"))
		}
		if authorizer.Authorize(c.GetHeader("X-User-ID"), userRoles, requiredPermission) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":               "listingkit_permission_denied",
			"message":             "ZITADEL identity is not allowed to access this ListingKit route",
			"required_permission": requiredPermission,
		})
	}
}

func roleHeaderValues(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	roles := make([]string, 0, 4)
	seen := map[string]struct{}{}
	for _, item := range strings.Split(value, ",") {
		role := strings.TrimSpace(item)
		if role != "" {
			if _, ok := seen[role]; ok {
				continue
			}
			seen[role] = struct{}{}
			roles = append(roles, role)
		}
	}
	return roles
}

func bearerToken(authorization string) string {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
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

func authorizeZitadelIdentity(identity *zitadelIntrospectionResponse, cfg zitadelAuthorizationConfig) (bool, string) {
	if identity == nil {
		return false, "ZITADEL identity is missing"
	}
	if len(cfg.AllowedTenantIDs) == 0 &&
		len(cfg.AllowedUserIDs) == 0 &&
		len(cfg.AllowedUsernames) == 0 &&
		len(cfg.AllowedRoles) == 0 {
		return false, "ZITADEL authorization is required but no allowlist is configured"
	}
	if valueInSet(firstNonEmptyZitadelValue(identity.ResourceID), cfg.AllowedTenantIDs) {
		return true, ""
	}
	if valueInSet(firstNonEmptyZitadelValue(identity.Subject, identity.UserID), cfg.AllowedUserIDs) {
		return true, ""
	}
	if valueInSet(firstNonEmptyZitadelValue(identity.Username), cfg.AllowedUsernames) {
		return true, ""
	}
	for _, role := range identity.Roles {
		if valueInSet(role, cfg.AllowedRoles) {
			return true, ""
		}
	}
	return false, "ZITADEL identity is not allowed to access ListingKit"
}

func stringSliceToSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, item := range values {
		value := strings.TrimSpace(item)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

func valueInSet(value string, set map[string]struct{}) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	_, ok := set[trimmed]
	return ok
}
