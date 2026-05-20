package httpapi

import (
	"net/http"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/httproute"
)

func AppendRouteDescriptors(routes []httproute.Descriptor, handler amazonlisting.Handler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/amazon/listings/generate", Module: "amazon-listing", Handler: handler.GenerateListing},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/amazon/listings/tasks", Module: "amazon-listing", Handler: handler.ListTaskQueue},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/amazon/listings/tasks/:task_id", Module: "amazon-listing", Handler: handler.GetTaskResult},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/amazon/listings/tasks/:task_id/workbench", Module: "amazon-listing", Handler: handler.GetTaskWorkbench},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/amazon/listings/tasks/:task_id/review", Module: "amazon-listing", Handler: handler.ReviewTask},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/amazon/listings/tasks/:task_id/submit", Module: "amazon-listing", Handler: handler.SubmitTask},
	)
}
