package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
)

func AppendRouteDescriptors(routes []httproute.Descriptor, handler RouteHandler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}

	routes = append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/generate", Module: "listing-kit", Handler: handler.GenerateListingKit},
	)
	routes = appendAuthContextRouteDescriptor(routes, handler)
	routes = appendSettingsRouteDescriptors(routes, handler)
	routes = appendStoreRouteDescriptors(routes, handler)
	routes = appendSubscriptionRouteDescriptors(routes, handler)
	routes = appendPlatformAdminRouteDescriptors(routes, handler)
	routes = appendZitadelSMSRouteDescriptors(routes, handler)
	routes = appendAdminRouteDescriptors(routes, handler)
	routes = appendTaskRouteDescriptors(routes, handler)
	routes = appendSheinSyncRouteDescriptors(routes, handler)
	routes = appendSheinPODImageLookupRouteDescriptors(routes, handler)
	return routes
}
