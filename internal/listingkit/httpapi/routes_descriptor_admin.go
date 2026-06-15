package httpapi

import "task-processor/internal/httproute"

func appendAdminRouteDescriptors(routes []httproute.Descriptor, handler AdminRouteHandler) []httproute.Descriptor {
	routes = appendAdminStoreRouteDescriptors(routes, handler)
	routes = appendAdminRuleRouteDescriptors(routes, handler)
	routes = appendAdminTopicRouteDescriptors(routes, handler)
	return appendAdminCatalogDataRouteDescriptors(routes, handler)
}
