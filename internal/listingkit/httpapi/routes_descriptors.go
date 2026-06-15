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
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches/:batch_id/design-approvals", Module: "listing-kit-studio", Handler: handler.ApproveStudioBatchDesigns},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches/:batch_id/tasks", Module: "listing-kit-studio", Handler: handler.CreateStudioBatchTasks},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/studio/batches/:batch_id", Module: "listing-kit-studio", Handler: handler.DeleteStudioBatch},
	)
}

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

func appendAdminRouteDescriptors(routes []httproute.Descriptor, handler AdminRouteHandler) []httproute.Descriptor {
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
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/sensitive-words", Module: "listing-kit-admin", Handler: handler.ListAdminSensitiveWords},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/sensitive-words/:id", Module: "listing-kit-admin", Handler: handler.GetAdminSensitiveWord},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/admin/sensitive-words", Module: "listing-kit-admin", Handler: handler.CreateAdminSensitiveWord},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/admin/sensitive-words/:id", Module: "listing-kit-admin", Handler: handler.UpdateAdminSensitiveWord},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/admin/sensitive-words/:id/status", Module: "listing-kit-admin", Handler: handler.UpdateAdminSensitiveWordStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/admin/sensitive-words/:id", Module: "listing-kit-admin", Handler: handler.DeleteAdminSensitiveWord},
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

func appendStudioGenerationRouteDescriptors(routes []httproute.Descriptor, handler StudioGenerationRouteHandler) []httproute.Descriptor {
	routes = append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/designs", Module: "listing-kit", Handler: handler.GenerateStudioDesigns},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/product-images", Module: "listing-kit", Handler: handler.GenerateStudioProductImages},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/async-jobs", Module: "listing-kit", Handler: handler.StartStudioAsyncJob},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/async-jobs/:job_id", Module: "listing-kit", Handler: handler.GetStudioAsyncJob},
	)
	batchRuns, ok := handler.(studioBatchRunRouteHandler)
	if !ok {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batch-runs", Module: "listing-kit", Handler: batchRuns.CreateStudioBatchRun},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/batch-runs/:run_id", Module: "listing-kit", Handler: batchRuns.GetStudioBatchRun},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/batch-runs/:run_id/items", Module: "listing-kit", Handler: batchRuns.ListStudioBatchRunItems},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batch-runs/:run_id/cancel", Module: "listing-kit", Handler: batchRuns.CancelStudioBatchRun},
	)
}

func appendTaskRouteDescriptors(routes []httproute.Descriptor, handler TaskRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/shein-images/regenerate", Module: "listing-kit", Handler: handler.RegenerateSheinDataImage},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/uploads/images", Module: "listing-kit", Handler: handler.UploadListingKitImages},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/uploads/files/*key", Module: "listing-kit", Handler: handler.GetUploadedListingKitImage},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/uploads/files/*key", Module: "listing-kit", Handler: handler.DeleteUploadedListingKitImage},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks", Module: "listing-kit", Handler: handler.ListTasks},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/sds/baselines/readiness", Module: "listing-kit", Handler: handler.GetSDSBaselineReadiness},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/sds/baselines/warm", Module: "listing-kit", Handler: handler.WarmSDSBaseline},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id", Module: "listing-kit", Handler: handler.GetTaskResult},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/requeue", Module: "listing-kit", Handler: handler.RequeuePendingTasks},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/recover", Module: "listing-kit", Handler: handler.RecoverTaskNow},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/recovery/recover", Module: "listing-kit", Handler: handler.BulkRecoverTasks},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/preview", Module: "listing-kit", Handler: handler.GetTaskPreview},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-tasks", Module: "listing-kit", Handler: handler.GetTaskGenerationTasks},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-queue", Module: "listing-kit", Handler: handler.GetTaskGenerationQueue},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-review-session", Module: "listing-kit", Handler: handler.GetTaskGenerationReviewSession},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-review-preview", Module: "listing-kit", Handler: handler.GetTaskGenerationReviewPreview},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/generation-navigation/dispatch", Module: "listing-kit", Handler: handler.DispatchTaskGenerationNavigation},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/generation-tasks/retry", Module: "listing-kit", Handler: handler.RetryTaskGenerationTasks},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/child-tasks/retry", Module: "listing-kit", Handler: handler.RetryTaskChildTask},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/generation-actions/execute", Module: "listing-kit", Handler: handler.ExecuteTaskGenerationAction},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/revision-history", Module: "listing-kit", Handler: handler.GetTaskRevisionHistory},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/revision-history/:revision_id", Module: "listing-kit", Handler: handler.GetTaskRevisionHistoryDetail},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/export", Module: "listing-kit", Handler: handler.GetTaskExport},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/revision", Module: "listing-kit", Handler: handler.ApplyTaskRevision},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/revision/validate", Module: "listing-kit", Handler: handler.ValidateTaskRevision},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/shein/price-preview", Module: "listing-kit", Handler: handler.PreviewSheinPrice},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/shein/categories", Module: "listing-kit", Handler: handler.SearchSheinCategories},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/tasks/:task_id/shein/final-draft", Module: "listing-kit", Handler: handler.UpdateSheinFinalDraft},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/submission-events", Module: "listing-kit", Handler: handler.GetSubmissionEvents},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/submit", Module: "listing-kit", Handler: handler.SubmitTask},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/submission-status/refresh", Module: "listing-kit", Handler: handler.RefreshSubmissionStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/tasks/:task_id/shein-resolution-cache", Module: "listing-kit", Handler: handler.ClearSheinResolutionCache},
	)
}

func appendSheinSyncRouteDescriptors(routes []httproute.Descriptor, handler sheinSyncRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/dashboard", Module: "listing-kit", Handler: handler.ListSheinEnrollmentDashboard},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/sync", Module: "listing-kit", Handler: handler.TriggerSheinStoreSync},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/summary", Module: "listing-kit", Handler: handler.GetSheinEnrollmentStoreSummary},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/products", Module: "listing-kit", Handler: handler.ListSheinSyncedProducts},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/shein-sync/products/:id/cost", Module: "listing-kit", Handler: handler.UpdateSheinSyncedProductCost},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/candidates/refresh", Module: "listing-kit", Handler: handler.RefreshSheinActivityCandidates},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/candidates", Module: "listing-kit", Handler: handler.ListSheinActivityCandidates},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/shein-sync/candidates/:id/review", Module: "listing-kit", Handler: handler.ReviewSheinActivityCandidate},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/enrollments", Module: "listing-kit", Handler: handler.ExecuteSheinActivityEnrollment},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/enrollment-runs", Module: "listing-kit", Handler: handler.ListSheinActivityEnrollmentRuns},
	)
}
