package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
	zitadelAuth := newZitadelAuthMiddleware()
	for _, route := range routes {
		handlers := append(routeAuthHandlers(route, zitadelAuth), route.Handler)
		r.Handle(route.Method, route.Path, handlers...)
	}
}
