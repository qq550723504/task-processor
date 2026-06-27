package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
)

func appendSheinSyncRouteDescriptors(routes []httproute.Descriptor, handler sheinSyncRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/dashboard", Module: "listing-kit", Handler: handler.ListSheinEnrollmentDashboard},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/sync", Module: "listing-kit", Handler: handler.TriggerSheinStoreSync},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/source-sds-products/:source_code/sync", Module: "listing-kit", Handler: handler.SyncSheinSourceSDSProduct},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/summary", Module: "listing-kit", Handler: handler.GetSheinEnrollmentStoreSummary},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/products", Module: "listing-kit", Handler: handler.ListSheinSyncedProducts},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/shein-sync/products/:id/cost", Module: "listing-kit", Handler: handler.UpdateSheinSyncedProductCost},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/sds-cost-groups", Module: "listing-kit", Handler: handler.ListSheinSDSCostGroups},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/source-sds-cost-groups", Module: "listing-kit", Handler: handler.ListSheinSourceSDSCostGroups},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/source-sds-metadata", Module: "listing-kit", Handler: handler.ListSheinSourceSDSMetadata},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/sds-cost-groups/:group_key/cost", Module: "listing-kit", Handler: handler.UpdateSheinSDSCostGroup},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/candidates/refresh", Module: "listing-kit", Handler: handler.RefreshSheinActivityCandidates},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/candidates", Module: "listing-kit", Handler: handler.ListSheinActivityCandidates},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/shein-sync/candidates/:id/review", Module: "listing-kit", Handler: handler.ReviewSheinActivityCandidate},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/enrollments", Module: "listing-kit", Handler: handler.ExecuteSheinActivityEnrollment},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/enrollment-runs", Module: "listing-kit", Handler: handler.ListSheinActivityEnrollmentRuns},
	)
}
