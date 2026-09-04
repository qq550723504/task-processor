package httpapi

import (
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
		r.Handle(route.Method, route.Path, handlers...)
	}
}
