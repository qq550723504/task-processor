package httpapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/httproute"
)

func buildHTTPServerFromRoutes(port int, routes []httproute.Descriptor, authorization routeAuthorization) *http.Server {
	return buildHTTPServerFromRoutesAt("", port, routes, authorization)
}

func buildHTTPServerFromRoutesAt(bindAddress string, port int, routes []httproute.Descriptor, authorization routeAuthorization) *http.Server {
	return buildHTTPServerFromRoutesAtWithAuthDependencies(bindAddress, port, routes, authorization)
}

func buildHTTPServerFromRoutesAtWithAuthDependencies(bindAddress string, port int, routes []httproute.Descriptor, dependencies routeAuthDependencies) *http.Server {
	router := gin.New()
	router.Use(gin.Recovery())
	mountRoutesWithAuthDependencies(router, routes, dependencies)
	return &http.Server{
		Addr:              serverAddress(bindAddress, port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func serverAddress(bindAddress string, port int) string {
	if bindAddress == "" {
		return fmt.Sprintf(":%d", port)
	}
	return net.JoinHostPort(bindAddress, fmt.Sprint(port))
}

func mountRoutes(r *gin.Engine, routes []httproute.Descriptor, authorization routeAuthorization) {
	mountRoutesWithAuthDependencies(r, routes, authorization)
}

func mountRoutesWithAuthDependencies(r *gin.Engine, routes []httproute.Descriptor, dependencies routeAuthDependencies) {
	for _, route := range routes {
		handlers := append(routeAuthHandlersWithDependencies(route, dependencies), route.Handler)
		if route.RequestTimeout > 0 {
			handlers = append([]gin.HandlerFunc{requestContextTimeout(route.RequestTimeout)}, handlers...)
		}
		r.Handle(route.Method, route.Path, handlers...)
	}
}

func requestContextTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		// Bound identifiers before authentication can include them in an error.
		if len(c.Request.Header.Get("X-Request-ID")) > 128 {
			c.Request.Header.Del("X-Request-ID")
		}
		c.Header("Cache-Control", "private, no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	}
}
