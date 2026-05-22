package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/httproute"
	"task-processor/internal/listingkit"
)

type PromptTemplateRouteHandler interface {
	ListPromptTemplateCatalog(c *gin.Context)
	GetPromptTemplateSchema(c *gin.Context)
	ListPromptTemplates(c *gin.Context)
	UpsertPromptTemplate(c *gin.Context)
	SetPromptTemplateStatus(c *gin.Context)
}

type TaskRouteHandler interface {
	listingkit.TaskHandler
	listingkit.CustomerStoreHandler
}

type SettingsRouteHandler interface {
	listingkit.SettingsHandler
	listingkit.CustomerStoreHandler
}

type StoreRouteHandler interface {
	listingkit.CustomerStoreHandler
}

type SubscriptionRouteHandler interface {
	listingkit.SubscriptionHandler
}

type PlatformAdminRouteHandler interface {
	listingkit.SubscriptionHandler
}

type AdminRouteHandler interface {
	listingkit.PlatformAdminHandler
}

type StudioGenerationRouteHandler interface {
	listingkit.TaskHandler
}

type RouteHandler interface {
	TaskRouteHandler
	SettingsRouteHandler
	StoreRouteHandler
	SubscriptionRouteHandler
	PlatformAdminRouteHandler
	AdminRouteHandler
	StudioGenerationRouteHandler
}

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
	return routes
}

func AppendPromptTemplateRouteDescriptors(routes []httproute.Descriptor, handler PromptTemplateRouteHandler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/prompts/catalog", Module: "listing-kit-prompts", Handler: handler.ListPromptTemplateCatalog},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/prompts/schema/:key", Module: "listing-kit-prompts", Handler: handler.GetPromptTemplateSchema},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/prompts", Module: "listing-kit-prompts", Handler: handler.ListPromptTemplates},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/prompts", Module: "listing-kit-prompts", Handler: handler.UpsertPromptTemplate},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/prompts/:key/status", Module: "listing-kit-prompts", Handler: handler.SetPromptTemplateStatus},
	)
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
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/studio/batches/:batch_id", Module: "listing-kit-studio", Handler: handler.DeleteStudioBatch},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/sessions", Module: "listing-kit-studio", Handler: handler.EnsureStudioSession},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/sessions/:session_id", Module: "listing-kit-studio", Handler: handler.GetStudioSession},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/studio/sessions/:session_id", Module: "listing-kit-studio", Handler: handler.UpdateStudioSession},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/sessions/:session_id/designs", Module: "listing-kit-studio", Handler: handler.ReplaceStudioSessionDesigns},
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
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/store-routing", Module: "listing-kit", Handler: handler.GetSheinStoreRoutingSettings},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/store-routing", Module: "listing-kit", Handler: handler.UpdateSheinStoreRoutingSettings},
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
	return append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/designs", Module: "listing-kit", Handler: handler.GenerateStudioDesigns},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/product-images", Module: "listing-kit", Handler: handler.GenerateStudioProductImages},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/async-jobs", Module: "listing-kit", Handler: handler.StartStudioAsyncJob},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/async-jobs/:job_id", Module: "listing-kit", Handler: handler.GetStudioAsyncJob},
	)
}

func appendTaskRouteDescriptors(routes []httproute.Descriptor, handler TaskRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/shein-images/regenerate", Module: "listing-kit", Handler: handler.RegenerateSheinDataImage},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/uploads/images", Module: "listing-kit", Handler: handler.UploadListingKitImages},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/uploads/files/*key", Module: "listing-kit", Handler: handler.GetUploadedListingKitImage},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/uploads/files/*key", Module: "listing-kit", Handler: handler.DeleteUploadedListingKitImage},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks", Module: "listing-kit", Handler: handler.ListTasks},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id", Module: "listing-kit", Handler: handler.GetTaskResult},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/preview", Module: "listing-kit", Handler: handler.GetTaskPreview},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-tasks", Module: "listing-kit", Handler: handler.GetTaskGenerationTasks},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-queue", Module: "listing-kit", Handler: handler.GetTaskGenerationQueue},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-review-session", Module: "listing-kit", Handler: handler.GetTaskGenerationReviewSession},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-review-preview", Module: "listing-kit", Handler: handler.GetTaskGenerationReviewPreview},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/generation-navigation/dispatch", Module: "listing-kit", Handler: handler.DispatchTaskGenerationNavigation},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/generation-tasks/retry", Module: "listing-kit", Handler: handler.RetryTaskGenerationTasks},
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
