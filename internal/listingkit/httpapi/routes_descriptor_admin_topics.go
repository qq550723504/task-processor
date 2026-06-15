package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
)

func appendAdminTopicRouteDescriptors(routes []httproute.Descriptor, handler AdminRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/generation-topic-catalog", Module: "listing-kit-admin", Handler: handler.ListAdminGenerationTopicCatalog},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/generation-topic-overrides", Module: "listing-kit-admin", Handler: handler.ListAdminGenerationTopicOverrides},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/generation-topic-overrides/:id", Module: "listing-kit-admin", Handler: handler.GetAdminGenerationTopicOverride},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/generation-topic-overrides", Module: "listing-kit-admin", Handler: handler.CreateAdminGenerationTopicOverride},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/generation-topic-overrides/:id", Module: "listing-kit-admin", Handler: handler.UpdateAdminGenerationTopicOverride},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/generation-topic-overrides/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminGenerationTopicOverrideStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/generation-topic-overrides/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminGenerationTopicOverride},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/generation-topic-policies", Module: "listing-kit-admin", Handler: handler.ListAdminGenerationTopicPolicies},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/generation-topic-policies/:id", Module: "listing-kit-admin", Handler: handler.GetAdminGenerationTopicPolicy},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/generation-topic-policies", Module: "listing-kit-admin", Handler: handler.CreateAdminGenerationTopicPolicy},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/generation-topic-policies/:id", Module: "listing-kit-admin", Handler: handler.UpdateAdminGenerationTopicPolicy},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/generation-topic-policies/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminGenerationTopicPolicyStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/generation-topic-policies/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminGenerationTopicPolicy},
	)
}
