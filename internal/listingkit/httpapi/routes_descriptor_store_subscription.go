package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
)

func appendStoreRouteDescriptors(routes []httproute.Descriptor, handler StoreRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/stores", Module: "listing-kit", Handler: handler.ListTenantStores},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/stores/simple", Module: "listing-kit", Handler: handler.ListSimpleTenantStores},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/stores", Module: "listing-kit", Handler: handler.CreateTenantStore},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/stores/:id", Module: "listing-kit", Handler: handler.UpdateTenantStore},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/stores/:id", Module: "listing-kit", Handler: handler.DeleteTenantStore},
	)
}

func appendSubscriptionRouteDescriptors(routes []httproute.Descriptor, handler SubscriptionRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/subscription/me", Module: "listing-kit", Handler: handler.GetCurrentSubscription},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/subscription/modules", Module: "listing-kit-admin", Handler: handler.ListSubscriptionModules},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/subscription/entitlements", Module: "listing-kit-admin", Handler: handler.ListSubscriptionEntitlements},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/subscription/entitlements/:module_code", Module: "listing-kit-admin", Handler: handler.UpsertSubscriptionEntitlement},
	)
}

func appendPlatformAdminRouteDescriptors(routes []httproute.Descriptor, handler PlatformAdminRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/platform/subscriptions", Module: "listing-kit-platform-admin", Handler: handler.ListPlatformTenantSubscriptions},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/platform/subscription-plans", Module: "listing-kit-platform-admin", Handler: handler.ListPlatformSubscriptionPlans},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/platform/subscription-plans", Module: "listing-kit-platform-admin", Handler: handler.UpsertPlatformSubscriptionPlan},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/platform/subscription-plans/:plan_code", Module: "listing-kit-platform-admin", Handler: handler.UpsertPlatformSubscriptionPlan},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/platform/subscription-plans/:plan_code/modules/:module_code", Module: "listing-kit-platform-admin", Handler: handler.UpsertPlatformSubscriptionPlanModule},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/platform/subscription-plans/:plan_code/modules/:module_code", Module: "listing-kit-platform-admin", Handler: handler.DeletePlatformSubscriptionPlanModule},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/platform/subscription-plans/:plan_code/status", Module: "listing-kit-platform-admin", Handler: handler.SetPlatformSubscriptionPlanStatus},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/platform/subscription-plans/:plan_code/tenants", Module: "listing-kit-platform-admin", Handler: handler.ListPlatformSubscriptionPlanTenants},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/platform/subscription-plans/:plan_code/audit-logs", Module: "listing-kit-platform-admin", Handler: handler.ListPlatformSubscriptionPlanAuditLogs},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/platform/subscriptions/:tenant_id", Module: "listing-kit-platform-admin", Handler: handler.GetPlatformTenantSubscription},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/platform/subscriptions/:tenant_id/audit-logs", Module: "listing-kit-platform-admin", Handler: handler.ListPlatformTenantSubscriptionAuditLogs},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/platform/subscriptions/:tenant_id/plan", Module: "listing-kit-platform-admin", Handler: handler.ApplyPlatformTenantSubscriptionPlan},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/platform/subscriptions/:tenant_id/entitlements/:module_code", Module: "listing-kit-platform-admin", Handler: handler.UpsertPlatformTenantSubscriptionEntitlement},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/platform/subscriptions/:tenant_id/usage/:module_code/:period_key/:metric", Module: "listing-kit-platform-admin", Handler: handler.SetPlatformTenantSubscriptionUsage},
	)
}
