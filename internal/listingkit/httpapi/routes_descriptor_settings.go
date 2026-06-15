package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
)

func appendSettingsRouteDescriptors(routes []httproute.Descriptor, handler SettingsRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/settings", Module: "listing-kit", Handler: handler.ListSettingsNamespaces},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/settings/:namespace/schema", Module: "listing-kit", Handler: handler.GetSettingsNamespaceSchema},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/settings/:namespace", Module: "listing-kit", Handler: handler.GetSettingsNamespace},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/settings/:namespace", Module: "listing-kit", Handler: handler.UpdateSettingsNamespace},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/store-profiles", Module: "listing-kit", Handler: handler.ListSheinStoreProfiles},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/store-profiles", Module: "listing-kit", Handler: handler.UpsertSheinStoreProfile},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/store-profiles/:id", Module: "listing-kit", Handler: handler.DeleteSheinStoreProfile},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein/settings", Module: "listing-kit", Handler: handler.GetSheinSettings},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/shein/settings", Module: "listing-kit", Handler: handler.UpdateSheinSettings},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/ai-clients/settings", Module: "listing-kit", Handler: handler.GetAIClientSettings},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/ai-clients/settings", Module: "listing-kit", Handler: handler.UpdateAIClientSettings},
	)
}
