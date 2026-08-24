package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/httproute"
)

// AuthContextRouteHandler is optional so existing ListingKit route handlers do
// not need to implement the device-authorization support endpoint.
type AuthContextRouteHandler interface {
	GetAuthContext(*gin.Context)
}

func appendAuthContextRouteDescriptor(routes []httproute.Descriptor, handler RouteHandler) []httproute.Descriptor {
	authContextHandler, ok := handler.(AuthContextRouteHandler)
	if !ok || authContextHandler == nil {
		return routes
	}
	return append(routes, httproute.Descriptor{
		Method:  http.MethodGet,
		Path:    "/api/v1/listing-kits/auth-context",
		Module:  "listing-kit",
		Handler: authContextHandler.GetAuthContext,
	})
}
