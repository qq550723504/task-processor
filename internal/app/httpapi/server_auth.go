package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
)

type routeAuthorization struct {
	identityMiddleware gin.HandlerFunc
	authorizer         *authz.ListingKitAuthorizer
}

// buildRouteAuthorization creates the immutable authorization dependency owned
// by one HTTP server. It does not publish configuration through package globals.
func buildRouteAuthorization(cfg *config.Config) (routeAuthorization, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	zitadel := cfg.ListingKit.Zitadel
	legacyUsernameAllowlistConfigured := zitadel.LegacyUsernameAllowlistConfigured || len(zitadel.AllowedUsernames) > 0
	authorizer, err := authz.NewListingKitAuthorizer(cfg.ListingKit.PlatformAdminUsers, cfg.ListingKit.PlatformAdminRoles)
	if err != nil {
		return routeAuthorization{}, fmt.Errorf("build route authorizer: %w", err)
	}
	return routeAuthorization{
		identityMiddleware: zitadelruntime.NewMiddleware(zitadelruntime.Config{
			IssuerURL:    strings.TrimRight(strings.TrimSpace(zitadel.IssuerURL), "/"),
			ClientID:     strings.TrimSpace(zitadel.ClientID),
			ClientSecret: strings.TrimSpace(zitadel.ClientSecret),
			ProjectID:    strings.TrimSpace(zitadel.ProjectID),
		}, zitadelruntime.AuthorizationConfig{
			Required:                          zitadel.AuthorizationRequired || legacyUsernameAllowlistConfigured,
			LegacyUsernameAllowlistConfigured: legacyUsernameAllowlistConfigured,
			AllowedTenantIDs:                  zitadelruntime.StringSliceToSet(zitadel.AllowedTenantIDs),
			AllowedUserIDs:                    zitadelruntime.StringSliceToSet(zitadel.AllowedUserIDs),
			AllowedRoles:                      zitadelruntime.StringSliceToSet(zitadel.AllowedRoles),
		}),
		authorizer: authorizer,
	}, nil
}

func routeAuthHandlers(route httproute.Descriptor, authorization routeAuthorization) []gin.HandlerFunc {
	if route.Method == http.MethodOptions {
		return nil
	}
	if authorization.identityMiddleware == nil || !listingkithttpapi.RouteRequiresZitadelAuth(route) {
		return nil
	}
	if roleAuth := listingkithttpapi.NewRouteRoleMiddlewareWithAuthorizer(route, authorization.authorizer); roleAuth != nil {
		return []gin.HandlerFunc{authorization.identityMiddleware, roleAuth}
	}
	return []gin.HandlerFunc{authorization.identityMiddleware}
}
