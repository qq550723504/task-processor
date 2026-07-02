package httpapi

import "github.com/gin-gonic/gin"

type settingsOnlyRouteHandler struct{}

func (settingsOnlyRouteHandler) ListSettingsNamespaces(c *gin.Context)     {}
func (settingsOnlyRouteHandler) GetSettingsHealth(c *gin.Context)          {}
func (settingsOnlyRouteHandler) GetSettingsNamespaceSchema(c *gin.Context) {}
func (settingsOnlyRouteHandler) GetSettingsNamespace(c *gin.Context)       {}
func (settingsOnlyRouteHandler) UpdateSettingsNamespace(c *gin.Context)    {}
func (settingsOnlyRouteHandler) ListSheinStoreProfiles(c *gin.Context)     {}
func (settingsOnlyRouteHandler) UpsertSheinStoreProfile(c *gin.Context)    {}
func (settingsOnlyRouteHandler) DeleteSheinStoreProfile(c *gin.Context)    {}
func (settingsOnlyRouteHandler) GetSheinSettings(c *gin.Context)           {}
func (settingsOnlyRouteHandler) UpdateSheinSettings(c *gin.Context)        {}
func (settingsOnlyRouteHandler) GetAIClientSettings(c *gin.Context)        {}
func (settingsOnlyRouteHandler) UpdateAIClientSettings(c *gin.Context)     {}

type taskOnlyRouteHandler struct{}

func (taskOnlyRouteHandler) GenerateListingKit(c *gin.Context)               {}
func (taskOnlyRouteHandler) ListTasks(c *gin.Context)                        {}
func (taskOnlyRouteHandler) GetSDSBaselineReadiness(c *gin.Context)          {}
func (taskOnlyRouteHandler) WarmSDSBaseline(c *gin.Context)                  {}
func (taskOnlyRouteHandler) CreateSDSRetirementRun(c *gin.Context)           {}
func (taskOnlyRouteHandler) GetSDSRetirementRun(c *gin.Context)              {}
func (taskOnlyRouteHandler) UpdateSDSRetirementSelection(c *gin.Context)     {}
func (taskOnlyRouteHandler) ConfirmSDSRetirementRun(c *gin.Context)          {}
func (taskOnlyRouteHandler) RetrySDSRetirementRun(c *gin.Context)            {}
func (taskOnlyRouteHandler) UploadListingKitImages(c *gin.Context)           {}
func (taskOnlyRouteHandler) GetUploadedListingKitImage(c *gin.Context)       {}
func (taskOnlyRouteHandler) DeleteUploadedListingKitImage(c *gin.Context)    {}
func (taskOnlyRouteHandler) AnalyzeStudioReferenceStyle(c *gin.Context)      {}
func (taskOnlyRouteHandler) GenerateStudioDesigns(c *gin.Context)            {}
func (taskOnlyRouteHandler) GenerateStudioProductImages(c *gin.Context)      {}
func (taskOnlyRouteHandler) StartStudioAsyncJob(c *gin.Context)              {}
func (taskOnlyRouteHandler) GetStudioAsyncJob(c *gin.Context)                {}
func (taskOnlyRouteHandler) RegenerateSheinDataImage(c *gin.Context)         {}
func (taskOnlyRouteHandler) GetTaskResult(c *gin.Context)                    {}
func (taskOnlyRouteHandler) RequeuePendingTasks(c *gin.Context)              {}
func (taskOnlyRouteHandler) RecoverTaskNow(c *gin.Context)                   {}
func (taskOnlyRouteHandler) BulkRecoverTasks(c *gin.Context)                 {}
func (taskOnlyRouteHandler) GetTaskPreview(c *gin.Context)                   {}
func (taskOnlyRouteHandler) GetTaskGenerationTasks(c *gin.Context)           {}
func (taskOnlyRouteHandler) GetTaskGenerationQueue(c *gin.Context)           {}
func (taskOnlyRouteHandler) GetTaskGenerationReviewSession(c *gin.Context)   {}
func (taskOnlyRouteHandler) GetTaskGenerationReviewPreview(c *gin.Context)   {}
func (taskOnlyRouteHandler) DispatchTaskGenerationNavigation(c *gin.Context) {}
func (taskOnlyRouteHandler) RetryTaskGenerationTasks(c *gin.Context)         {}
func (taskOnlyRouteHandler) RetryTaskChildTask(c *gin.Context)               {}
func (taskOnlyRouteHandler) ExecuteTaskGenerationAction(c *gin.Context)      {}
func (taskOnlyRouteHandler) GetTaskRevisionHistory(c *gin.Context)           {}
func (taskOnlyRouteHandler) GetTaskRevisionHistoryDetail(c *gin.Context)     {}
func (taskOnlyRouteHandler) GetTaskExport(c *gin.Context)                    {}
func (taskOnlyRouteHandler) ApplyTaskRevision(c *gin.Context)                {}
func (taskOnlyRouteHandler) ValidateTaskRevision(c *gin.Context)             {}
func (taskOnlyRouteHandler) SubmitTask(c *gin.Context)                       {}
func (taskOnlyRouteHandler) RefreshSubmissionStatus(c *gin.Context)          {}
func (taskOnlyRouteHandler) PreviewSheinPrice(c *gin.Context)                {}
func (taskOnlyRouteHandler) SearchSheinCategories(c *gin.Context)            {}
func (taskOnlyRouteHandler) UpdateSheinFinalDraft(c *gin.Context)            {}
func (taskOnlyRouteHandler) GetSubmissionEvents(c *gin.Context)              {}
func (taskOnlyRouteHandler) ClearSheinResolutionCache(c *gin.Context)        {}

type subscriptionOnlyRouteHandler struct{}

func (subscriptionOnlyRouteHandler) GetCurrentSubscription(c *gin.Context)  {}
func (subscriptionOnlyRouteHandler) ListSubscriptionModules(c *gin.Context) {}
func (subscriptionOnlyRouteHandler) ListSubscriptionEntitlements(c *gin.Context) {
}
func (subscriptionOnlyRouteHandler) UpsertSubscriptionEntitlement(c *gin.Context) {}

type platformAdminSubscriptionRouteHandler struct{}

func (platformAdminSubscriptionRouteHandler) ListPlatformTenantSubscriptions(c *gin.Context) {
}
func (platformAdminSubscriptionRouteHandler) ListPlatformSubscriptionPlans(c *gin.Context) {}
func (platformAdminSubscriptionRouteHandler) UpsertPlatformSubscriptionPlan(c *gin.Context) {
}
func (platformAdminSubscriptionRouteHandler) UpsertPlatformSubscriptionPlanModule(c *gin.Context) {
}
func (platformAdminSubscriptionRouteHandler) DeletePlatformSubscriptionPlanModule(c *gin.Context) {
}
func (platformAdminSubscriptionRouteHandler) SetPlatformSubscriptionPlanStatus(c *gin.Context) {
}
func (platformAdminSubscriptionRouteHandler) ListPlatformSubscriptionPlanTenants(c *gin.Context) {
}
func (platformAdminSubscriptionRouteHandler) ListPlatformSubscriptionPlanAuditLogs(c *gin.Context) {
}
func (platformAdminSubscriptionRouteHandler) GetPlatformTenantSubscription(c *gin.Context) {}
func (platformAdminSubscriptionRouteHandler) ApplyPlatformTenantSubscriptionPlan(c *gin.Context) {
}
func (platformAdminSubscriptionRouteHandler) UpsertPlatformTenantSubscriptionEntitlement(c *gin.Context) {
}
func (platformAdminSubscriptionRouteHandler) SetPlatformTenantSubscriptionUsage(c *gin.Context) {
}
func (platformAdminSubscriptionRouteHandler) ListPlatformTenantSubscriptionAuditLogs(c *gin.Context) {
}

type storeOnlyRouteHandler struct{}

func (storeOnlyRouteHandler) ListTenantStores(c *gin.Context)       {}
func (storeOnlyRouteHandler) CreateTenantStore(c *gin.Context)      {}
func (storeOnlyRouteHandler) UpdateTenantStore(c *gin.Context)      {}
func (storeOnlyRouteHandler) DeleteTenantStore(c *gin.Context)      {}
func (storeOnlyRouteHandler) ListSimpleTenantStores(c *gin.Context) {}

type studioGenerationOnlyRouteHandler struct{}

func (studioGenerationOnlyRouteHandler) AnalyzeStudioReferenceStyle(c *gin.Context) {}
func (studioGenerationOnlyRouteHandler) GenerateStudioDesigns(c *gin.Context)       {}
func (studioGenerationOnlyRouteHandler) GenerateStudioProductImages(c *gin.Context) {}
func (studioGenerationOnlyRouteHandler) StartStudioAsyncJob(c *gin.Context)         {}
func (studioGenerationOnlyRouteHandler) GetStudioAsyncJob(c *gin.Context)           {}

var _ SettingsRouteHandler = settingsOnlyRouteHandler{}
var _ TaskRouteHandler = taskOnlyRouteHandler{}
var _ SubscriptionRouteHandler = subscriptionOnlyRouteHandler{}
var _ PlatformAdminRouteHandler = platformAdminSubscriptionRouteHandler{}
var _ StoreRouteHandler = storeOnlyRouteHandler{}
var _ StudioGenerationRouteHandler = studioGenerationOnlyRouteHandler{}
