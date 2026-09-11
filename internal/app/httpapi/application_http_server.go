package httpapi

import (
	"context"
	"net/http"
	"time"

	"task-processor/internal/httproute"
)

// buildIsolatedApplicationHTTPServer applies the shared transport boundary for
// explicitly assembled applications. The caller still owns the returned
// server, its listener and every dependency used by the route handlers.
func buildIsolatedApplicationHTTPServer(routes []httproute.Descriptor, dependencies routeAuthDependencies, requestTimeout time.Duration) *http.Server {
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, routes, dependencies)
	server.ReadTimeout = requestTimeout
	server.WriteTimeout = requestTimeout + 2*time.Second
	inner := server.Handler
	server.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		ctx, cancel := context.WithTimeout(request.Context(), requestTimeout)
		defer cancel()
		inner.ServeHTTP(writer, request.WithContext(ctx))
	})
	return server
}
