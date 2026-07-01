package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
)

func appendAdminRuleRouteDescriptors(routes []httproute.Descriptor, handler AdminRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/filter-rules", Module: "listing-kit-admin", Handler: handler.ListAdminFilterRules},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/filter-rules/:id", Module: "listing-kit-admin", Handler: handler.GetAdminFilterRule},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/filter-rules", Module: "listing-kit-admin", Handler: handler.CreateAdminFilterRule},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/filter-rules/:id", Module: "listing-kit-admin", Handler: handler.UpdateAdminFilterRule},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/filter-rules/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminFilterRuleStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/filter-rules/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminFilterRule},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/profit-rules", Module: "listing-kit-admin", Handler: handler.ListAdminProfitRules},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/profit-rules/:id", Module: "listing-kit-admin", Handler: handler.GetAdminProfitRule},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/profit-rules", Module: "listing-kit-admin", Handler: handler.CreateAdminProfitRule},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/profit-rules/:id", Module: "listing-kit-admin", Handler: handler.UpdateAdminProfitRule},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/profit-rules/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminProfitRuleStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/profit-rules/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminProfitRule},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/pricing-rules", Module: "listing-kit-admin", Handler: handler.ListAdminPricingRules},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/pricing-rules/:id", Module: "listing-kit-admin", Handler: handler.GetAdminPricingRule},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/pricing-rules", Module: "listing-kit-admin", Handler: handler.CreateAdminPricingRule},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/pricing-rules/:id", Module: "listing-kit-admin", Handler: handler.UpdateAdminPricingRule},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/pricing-rules/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminPricingRuleStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/pricing-rules/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminPricingRule},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/operation-strategies", Module: "listing-kit-admin", Handler: handler.ListAdminOperationStrategies},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/operation-strategies/:id", Module: "listing-kit-admin", Handler: handler.GetAdminOperationStrategy},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/operation-strategies", Module: "listing-kit-admin", Handler: handler.CreateAdminOperationStrategy},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/operation-strategies/:id", Module: "listing-kit-admin", Handler: handler.UpdateAdminOperationStrategy},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/operation-strategies/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminOperationStrategyStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/operation-strategies/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminOperationStrategy},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/scheduled-task-configs", Module: "listing-kit-admin", Handler: handler.ListAdminScheduledTaskConfigs},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/scheduled-task-configs/:id", Module: "listing-kit-admin", Handler: handler.GetAdminScheduledTaskConfig},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/scheduled-task-configs", Module: "listing-kit-admin", Handler: handler.UpsertAdminScheduledTaskConfig},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/scheduled-task-configs/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminScheduledTaskConfigStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/scheduled-task-configs/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminScheduledTaskConfig},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/sensitive-words", Module: "listing-kit-admin", Handler: handler.ListAdminSensitiveWords},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/sensitive-words/:id", Module: "listing-kit-admin", Handler: handler.GetAdminSensitiveWord},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/sensitive-words", Module: "listing-kit-admin", Handler: handler.CreateAdminSensitiveWord},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/sensitive-words/:id", Module: "listing-kit-admin", Handler: handler.UpdateAdminSensitiveWord},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/sensitive-words/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminSensitiveWordStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/sensitive-words/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminSensitiveWord},
	)
}
