package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
	"task-processor/internal/productenrich"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

func AppendProductRouteDescriptors(routes []httproute.Descriptor, productHandler productenrich.ProductHandler, imageHandler productimagehttpapi.RouteHandler) []httproute.Descriptor {
	routes = appendProductHandlerRoutes(routes, productHandler)
	routes = appendImageHandlerRoutes(routes, imageHandler)
	return routes
}

func appendProductHandlerRoutes(routes []httproute.Descriptor, handler productenrich.ProductHandler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/products/generate", Module: "products", Handler: handler.GenerateProduct},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/products/tasks/:task_id", Module: "products", Handler: handler.GetTaskResult},
	)
}

func appendImageHandlerRoutes(routes []httproute.Descriptor, handler productimagehttpapi.RouteHandler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/images/process", Module: "images", Handler: handler.ProcessImages},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/images/tasks/:task_id", Module: "images", Handler: handler.GetTaskResult},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/images/tasks/:task_id/review", Module: "images", Handler: handler.ReviewTask},
	)
}
