package httpapi

import "github.com/gin-gonic/gin"

type StoreRouteHandler interface {
	ListTenantStores(c *gin.Context)
	ListSimpleTenantStores(c *gin.Context)
	CreateTenantStore(c *gin.Context)
	UpdateTenantStore(c *gin.Context)
	DeleteTenantStore(c *gin.Context)
}

type SubscriptionRouteHandler interface {
	GetCurrentSubscription(c *gin.Context)
	ListSubscriptionModules(c *gin.Context)
	ListSubscriptionEntitlements(c *gin.Context)
	UpsertSubscriptionEntitlement(c *gin.Context)
}

type PlatformAdminRouteHandler interface {
	ListPlatformTenantSubscriptions(c *gin.Context)
	ListPlatformSubscriptionPlans(c *gin.Context)
	UpsertPlatformSubscriptionPlan(c *gin.Context)
	UpsertPlatformSubscriptionPlanModule(c *gin.Context)
	DeletePlatformSubscriptionPlanModule(c *gin.Context)
	SetPlatformSubscriptionPlanStatus(c *gin.Context)
	ListPlatformSubscriptionPlanTenants(c *gin.Context)
	ListPlatformSubscriptionPlanAuditLogs(c *gin.Context)
	GetPlatformTenantSubscription(c *gin.Context)
	ApplyPlatformTenantSubscriptionPlan(c *gin.Context)
	UpsertPlatformTenantSubscriptionEntitlement(c *gin.Context)
	SetPlatformTenantSubscriptionUsage(c *gin.Context)
	ListPlatformTenantSubscriptionAuditLogs(c *gin.Context)
}
