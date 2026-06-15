package httpapi

import (
	"encoding/json"
	"net/http"
	"sync"

	"task-processor/internal/authz"
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
