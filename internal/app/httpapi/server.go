package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	listingkithttpapi "task-processor/internal/listingkit/httpapi"
)

func buildHTTPServerFromRoutes(port int, routes []routeDescriptor) *http.Server {
	router := gin.New()
	router.Use(gin.Recovery())
	mountRoutes(router, routes)
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func mountRoutes(r *gin.Engine, routes []routeDescriptor) {
	zitadelAuth := listingkithttpapi.NewZitadelAuthMiddlewareFromEnv()
	for _, route := range routes {
		if zitadelAuth != nil && listingkithttpapi.RouteRequiresZitadelAuth(route) {
			if roleAuth := listingkithttpapi.NewRouteRoleMiddleware(route); roleAuth != nil {
				r.Handle(route.Method, route.Path, zitadelAuth, roleAuth, route.Handler)
				continue
			}
			r.Handle(route.Method, route.Path, zitadelAuth, route.Handler)
			continue
		}
		r.Handle(route.Method, route.Path, route.Handler)
	}
}
