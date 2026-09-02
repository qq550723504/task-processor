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
	router := gin.New()
	router.Use(gin.Recovery())
	mountRoutes(router, routes, authorization)
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
	for _, route := range routes {
		handlers := append(routeAuthHandlers(route, authorization), route.Handler)
		r.Handle(route.Method, route.Path, handlers...)
	}
}
