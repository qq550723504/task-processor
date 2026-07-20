package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
	"task-processor/internal/listingkit"
)

func AppendRouteDescriptors(routes []httproute.Descriptor, handler RouteHandler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}

	routes = append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/generate", Module: "listing-kit", Handler: handler.GenerateListingKit},
	)
	routes = appendSettingsRouteDescriptors(routes, handler)
	routes = appendStoreRouteDescriptors(routes, handler)
	routes = appendSubscriptionRouteDescriptors(routes, handler)
	routes = appendPlatformAdminRouteDescriptors(routes, handler)
	routes = appendAdminRouteDescriptors(routes, handler)
	routes = appendStudioGenerationRouteDescriptors(routes, handler)
	routes = appendTaskRouteDescriptors(routes, handler)
	routes = appendSheinSyncRouteDescriptors(routes, handler)
	routes = appendSheinPODImageLookupRouteDescriptors(routes, handler)
	return routes
}

func AppendStudioSessionRouteDescriptors(routes []httproute.Descriptor, handler listingkit.StudioSessionHandler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/sessions/gallery", Module: "listing-kit-studio", Handler: handler.ListStudioSessionGallery},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/batches", Module: "listing-kit-studio", Handler: handler.ListStudioBatches},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/batches/:batch_id", Module: "listing-kit-studio", Handler: handler.GetStudioBatch},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches", Module: "listing-kit-studio", Handler: handler.UpsertStudioBatch},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches/:batch_id/generate", Module: "listing-kit-studio", Handler: handler.StartStudioBatchGeneration},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches/:batch_id/items/retry", Module: "listing-kit-studio", Handler: handler.RetryStudioBatchItems},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches/:batch_id/sds-child-tasks/retry", Module: "listing-kit-studio", Handler: handler.RetryStudioBatchSDSChildTasks},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches/:batch_id/design-approvals", Module: "listing-kit-studio", Handler: handler.ApproveStudioBatchDesigns},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches/:batch_id/tasks", Module: "listing-kit-studio", Handler: handler.CreateStudioBatchTasks},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/studio/batches/:batch_id", Module: "listing-kit-studio", Handler: handler.DeleteStudioBatch},
	)
}
