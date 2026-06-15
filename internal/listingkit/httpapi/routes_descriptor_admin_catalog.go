package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
)

func appendAdminCatalogDataRouteDescriptors(routes []httproute.Descriptor, handler AdminRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/product-import-mappings", Module: "listing-kit-admin", Handler: handler.ListAdminProductImportMappings},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/product-import-mappings/:id", Module: "listing-kit-admin", Handler: handler.GetAdminProductImportMapping},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/product-import-mappings", Module: "listing-kit-admin", Handler: handler.CreateAdminProductImportMapping},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/product-import-mappings/:id", Module: "listing-kit-admin", Handler: handler.UpdateAdminProductImportMapping},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/product-import-mappings/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminProductImportMappingStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/product-import-mappings/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminProductImportMapping},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/categories", Module: "listing-kit-admin", Handler: handler.ListAdminCategories},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/categories/:id", Module: "listing-kit-admin", Handler: handler.GetAdminCategory},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/categories", Module: "listing-kit-admin", Handler: handler.CreateAdminCategory},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/categories/:id", Module: "listing-kit-admin", Handler: handler.UpdateAdminCategory},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/categories/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminCategoryStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/categories/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminCategory},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/product-data", Module: "listing-kit-admin", Handler: handler.ListAdminProductData},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/product-data/:id", Module: "listing-kit-admin", Handler: handler.GetAdminProductData},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/product-data", Module: "listing-kit-admin", Handler: handler.CreateAdminProductData},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/product-data/:id", Module: "listing-kit-admin", Handler: handler.UpdateAdminProductData},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/product-data/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminProductDataStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/product-data/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminProductData},
	)
}
