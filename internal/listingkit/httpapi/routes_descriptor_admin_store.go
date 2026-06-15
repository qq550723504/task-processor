package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
)

func appendAdminStoreRouteDescriptors(routes []httproute.Descriptor, handler AdminRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/stores", Module: "listing-kit-admin", Handler: handler.ListAdminStores},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/stores/simple", Module: "listing-kit-admin", Handler: handler.ListSimpleAdminStores},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/stores/deleted", Module: "listing-kit-admin", Handler: handler.ListDeletedAdminStores},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/stores/:id", Module: "listing-kit-admin", Handler: handler.GetAdminStore},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/stores", Module: "listing-kit-admin", Handler: handler.CreateAdminStore},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/stores/:id", Module: "listing-kit-admin", Handler: handler.UpdateAdminStore},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/stores/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminStoreStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/stores/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminStore},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/stores/:id/restore", Module: "listing-kit-admin", Handler: handler.RestoreAdminStore},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/stores/:id/permanent", Module: "listing-kit-admin", Handler: handler.PermanentlyDeleteAdminStore},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/stores/:id/extend-validity", Module: "listing-kit-admin", Handler: handler.ExtendAdminStoreValidity},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/store-statistics", Module: "listing-kit-admin", Handler: handler.ListAdminStoreStatistics},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/import-tasks", Module: "listing-kit-admin", Handler: handler.ListAdminImportTasks},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/import-tasks/batch", Module: "listing-kit-admin", Handler: handler.BatchCreateAdminImportTasks},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/import-tasks/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminImportTask},
	)
}
