package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authz"
	"task-processor/internal/core/config"
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
	if !runtimeCfg.AuthConfig.Required && !runtimeCfg.AuthzConfig.Required {
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
