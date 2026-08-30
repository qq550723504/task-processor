package zitadel

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Config struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	ProjectID    string
	HTTPClient   *http.Client
}

type AuthorizationConfig struct {
	Required                          bool
	LegacyUsernameAllowlistConfigured bool
	AllowedTenantIDs                  map[string]struct{}
	AllowedUserIDs                    map[string]struct{}
	AllowedRoles                      map[string]struct{}
}

type IntrospectionResponse struct {
	Active     bool            `json:"active"`
	Subject    string          `json:"sub"`
	Username   string          `json:"username"`
	UserID     string          `json:"user_id"`
	ResourceID string          `json:"urn:zitadel:iam:user:resourceowner:id"`
	Roles      []string        `json:"-"`
	Extra      json.RawMessage `json:"-"`
}

type discoveryDocument struct {
	IntrospectionEndpoint string `json:"introspection_endpoint"`
}

type middleware struct {
	cfg      Config
	authzCfg AuthorizationConfig
	verifier Verifier
}

func NewMiddleware(cfg Config, authzCfg AuthorizationConfig) gin.HandlerFunc {
	return newMiddleware(cfg, authzCfg).Handle
}

func newMiddleware(cfg Config, authzCfg AuthorizationConfig) *middleware {
	cfg = normalizeConfig(cfg)
	return &middleware{cfg: cfg, authzCfg: authzCfg, verifier: newVerifier(cfg)}
}

func normalizeConfig(cfg Config) Config {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	cfg.IssuerURL = strings.TrimRight(strings.TrimSpace(cfg.IssuerURL), "/")
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.ClientSecret = strings.TrimSpace(cfg.ClientSecret)
	cfg.ProjectID = strings.TrimSpace(cfg.ProjectID)
	return cfg
}
