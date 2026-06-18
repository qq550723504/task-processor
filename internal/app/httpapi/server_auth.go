package httpapi

import (
	"github.com/gin-gonic/gin"

	listingkithttpapi "task-processor/internal/listingkit/httpapi"
)

func routeAuthHandlers(route routeDescriptor, zitadelAuth gin.HandlerFunc) []gin.HandlerFunc {
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
