package sourcea1688

import (
	"net/http"

	"task-processor/internal/authz"
	"task-processor/internal/httproute"
)

const ModuleName = "product-sourcing"

func AppendRouteDescriptors(routes []httproute.Descriptor, handler *Handler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	return append(routes, httproute.Descriptor{
		Method:     http.MethodPost,
		Path:       "/api/v1/product-sourcing/1688/listingkit/tasks",
		Module:     ModuleName,
		Permission: authz.PermissionProductSourcingWrite,
		Handler:    handler.CreateListingKitTask,
	})
}
