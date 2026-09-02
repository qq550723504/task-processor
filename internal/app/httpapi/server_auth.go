package httpapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
)

// configureRouteAuthorization initializes the authentication state consumed by
// the application-wide route middleware. It must not depend on construction of
// the optional ListingKit module, because other modules also publish protected
// routes.
func configureRouteAuthorization(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	listingkithttpapi.ConfigureListingKitZitadelAuth(cfg.ListingKit.Zitadel)
	if err := listingkithttpapi.ConfigureListingKitAuthorization(cfg.ListingKit.PlatformAdminUsers, cfg.ListingKit.PlatformAdminRoles); err != nil {
		return fmt.Errorf("configure route authorization: %w", err)
	}
	return nil
}

func routeAuthHandlers(route httproute.Descriptor, zitadelAuth gin.HandlerFunc) []gin.HandlerFunc {
	if route.Method == http.MethodOptions {
		return nil
	}
	if zitadelAuth == nil || !listingkithttpapi.RouteRequiresZitadelAuth(route) {
		return nil
	}
	if roleAuth := listingkithttpapi.NewRouteRoleMiddleware(route); roleAuth != nil {
		return []gin.HandlerFunc{zitadelAuth, roleAuth}
	}
	return []gin.HandlerFunc{zitadelAuth}
}

func newZitadelAuthMiddleware() gin.HandlerFunc {
	return listingkithttpapi.NewZitadelAuthMiddlewareFromEnv()
}
