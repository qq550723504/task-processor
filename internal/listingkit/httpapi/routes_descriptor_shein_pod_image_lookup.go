package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
)

func appendSheinPODImageLookupRouteDescriptors(routes []httproute.Descriptor, handler sheinPODImageLookupRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-pod-image-lookup/stores/:store_id", Module: "listing-kit", Handler: handler.LookupSheinPODImages},
	)
}
