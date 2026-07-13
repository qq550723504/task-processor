package httpapi

import (
	"context"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/httproute"
	"task-processor/internal/infra/worker"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestNewHTTPModuleRegistersListingRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()
	module := NewHTTPModule(stubRouteHandler{})

	require.Equal(t, "listing-kit", module.Name())
	require.True(t, module.Enabled(nil))
	require.NoError(t, module.Register(reg))

	keys := routeKeys(reg.Routes())
	require.Contains(t, keys, "POST /api/v1/listing-kits/generate")
	require.Contains(t, keys, "GET /api/v1/listing-kits/settings-health")
	require.Contains(t, keys, "POST /api/v1/listing-kits/studio/reference-style/analyze")
	require.Contains(t, keys, "POST /api/v1/listing-kits/tasks/requeue")
	require.Contains(t, keys, "POST /api/v1/listing-kits/sds/retirements")
	require.Contains(t, keys, "GET /api/v1/listing-kits/sds/retirements/:run_id")
	require.Contains(t, keys, "PATCH /api/v1/listing-kits/sds/retirements/:run_id/items")
	require.Contains(t, keys, "POST /api/v1/listing-kits/sds/retirements/:run_id/confirm")
	require.Contains(t, keys, "POST /api/v1/listing-kits/sds/retirements/:run_id/retry")
	require.NotContains(t, keys, "GET /api/v1/listing-kits/studio/sessions/gallery")
}

func TestNewStudioHTTPModuleRegistersStudioRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()
	module := NewStudioHTTPModule(stubStudioSessionRouteHandler{})

	require.Equal(t, "listing-kit-studio", module.Name())
	require.True(t, module.Enabled(nil))
	require.NoError(t, module.Register(reg))

	keys := routeKeys(reg.Routes())
	require.NotContains(t, keys, "POST /api/v1/listing-kits/generate")
	require.Contains(t, keys, "GET /api/v1/listing-kits/studio/sessions/gallery")
	require.NotContains(t, keys, "POST /api/v1/listing-kits/studio/sessions")
	require.NotContains(t, keys, "GET /api/v1/listing-kits/studio/sessions/:session_id")
	require.NotContains(t, keys, "PATCH /api/v1/listing-kits/studio/sessions/:session_id")
	require.NotContains(t, keys, "POST /api/v1/listing-kits/studio/sessions/:session_id/designs")
	require.NotContains(t, keys, "POST /api/v1/listing-kits/studio/sessions/:session_id/designs/append")
}

func TestNewRuntimeModuleRegistersRoutesAndWorkerPool(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()
	module := NewRuntimeModule(&Module{
		Handler:              stubRouteHandler{},
		StudioSessionHandler: stubStudioSessionRouteHandler{},
		Pool:                 stubWorkerPool{},
	})

	require.Equal(t, "listing-kit", module.Name())
	require.True(t, module.Enabled(nil))
	require.NoError(t, module.Register(reg))

	keys := routeKeys(reg.Routes())
	require.Contains(t, keys, "POST /api/v1/listing-kits/generate")
	require.Contains(t, keys, "GET /api/v1/listing-kits/studio/sessions/gallery")
	require.NotContains(t, keys, "POST /api/v1/listing-kits/studio/sessions")
	require.NotContains(t, keys, "GET /api/v1/listing-kits/studio/sessions/:session_id")
	require.NotContains(t, keys, "PATCH /api/v1/listing-kits/studio/sessions/:session_id")

	pools := reg.WorkerPools()
	require.Len(t, pools, 1)
	require.Equal(t, "listing_kit", pools[0].Name)
}

func TestAppendRouteDescriptorsIncludesSheinSyncRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()
	module := NewHTTPModule(stubRouteHandler{})

	require.NoError(t, module.Register(reg))

	keys := routeKeys(reg.Routes())
	require.Contains(t, keys, "GET /api/v1/listing-kits/shein-sync/dashboard")
	require.Contains(t, keys, "POST /api/v1/listing-kits/shein-sync/stores/:store_id/sync")
	require.Contains(t, keys, "POST /api/v1/listing-kits/shein-sync/stores/:store_id/source-sds-products/:source_code/sync")
	require.Contains(t, keys, "GET /api/v1/listing-kits/shein-sync/stores/:store_id/summary")
	require.Contains(t, keys, "GET /api/v1/listing-kits/shein-sync/stores/:store_id/products")
	require.Contains(t, keys, "GET /api/v1/listing-kits/shein-sync/stores/:store_id/source-sds-cost-groups")
	require.Contains(t, keys, "GET /api/v1/listing-kits/shein-sync/stores/:store_id/source-sds-metadata")
	require.Contains(t, keys, "GET /api/v1/listing-kits/shein-sync/stores/:store_id/activity-strategy")
	require.Contains(t, keys, "PATCH /api/v1/listing-kits/shein-sync/stores/:store_id/activity-strategy")
	require.Contains(t, keys, "POST /api/v1/listing-kits/shein-sync/stores/:store_id/enrollments")
	require.Contains(t, keys, "GET /api/v1/listing-kits/shein-sync/stores/:store_id/enrollment-runs")
	require.Contains(t, keys, "GET /api/v1/listing-kits/shein-pod-image-lookup/stores/:store_id")
}

type stubStudioSessionRouteHandler struct{}

func (stubStudioSessionRouteHandler) ListStudioSessionGallery(*gin.Context)   {}
func (stubStudioSessionRouteHandler) ListStudioBatches(*gin.Context)          {}
func (stubStudioSessionRouteHandler) GetStudioBatch(*gin.Context)             {}
func (stubStudioSessionRouteHandler) StartStudioBatchGeneration(*gin.Context) {}
func (stubStudioSessionRouteHandler) RetryStudioBatchItems(*gin.Context)      {}
func (stubStudioSessionRouteHandler) ApproveStudioBatchDesigns(*gin.Context)  {}
func (stubStudioSessionRouteHandler) CreateStudioBatchTasks(*gin.Context)     {}
func (stubStudioSessionRouteHandler) UpsertStudioBatch(*gin.Context)          {}
func (stubStudioSessionRouteHandler) DeleteStudioBatch(*gin.Context)          {}

type stubRouteHandler struct{}

func (stubRouteHandler) GenerateListingKit(*gin.Context)                          {}
func (stubRouteHandler) GetTaskSDSRepair(*gin.Context)                            {}
func (stubRouteHandler) RepairAndRetryTaskSDS(*gin.Context)                       {}
func (stubRouteHandler) ListTasks(*gin.Context)                                   {}
func (stubRouteHandler) GetSDSBaselineReadiness(*gin.Context)                     {}
func (stubRouteHandler) WarmSDSBaseline(*gin.Context)                             {}
func (stubRouteHandler) CreateSDSRetirementRun(*gin.Context)                      {}
func (stubRouteHandler) GetSDSRetirementRun(*gin.Context)                         {}
func (stubRouteHandler) UpdateSDSRetirementSelection(*gin.Context)                {}
func (stubRouteHandler) ConfirmSDSRetirementRun(*gin.Context)                     {}
func (stubRouteHandler) RetrySDSRetirementRun(*gin.Context)                       {}
func (stubRouteHandler) UploadListingKitImages(*gin.Context)                      {}
func (stubRouteHandler) GetUploadedListingKitImage(*gin.Context)                  {}
func (stubRouteHandler) DeleteUploadedListingKitImage(*gin.Context)               {}
func (stubRouteHandler) AnalyzeStudioReferenceStyle(*gin.Context)                 {}
func (stubRouteHandler) GenerateStudioDesigns(*gin.Context)                       {}
func (stubRouteHandler) GenerateStudioProductImages(*gin.Context)                 {}
func (stubRouteHandler) StartStudioAsyncJob(*gin.Context)                         {}
func (stubRouteHandler) GetStudioAsyncJob(*gin.Context)                           {}
func (stubRouteHandler) RegenerateSheinDataImage(*gin.Context)                    {}
func (stubRouteHandler) GetTaskResult(*gin.Context)                               {}
func (stubRouteHandler) RequeuePendingTasks(*gin.Context)                         {}
func (stubRouteHandler) RecoverTaskNow(*gin.Context)                              {}
func (stubRouteHandler) BulkRecoverTasks(*gin.Context)                            {}
func (stubRouteHandler) GetTaskPreview(*gin.Context)                              {}
func (stubRouteHandler) GetTaskGenerationTasks(*gin.Context)                      {}
func (stubRouteHandler) GetTaskGenerationQueue(*gin.Context)                      {}
func (stubRouteHandler) GetTaskGenerationReviewSession(*gin.Context)              {}
func (stubRouteHandler) GetTaskGenerationReviewPreview(*gin.Context)              {}
func (stubRouteHandler) DispatchTaskGenerationNavigation(*gin.Context)            {}
func (stubRouteHandler) RetryTaskGenerationTasks(*gin.Context)                    {}
func (stubRouteHandler) RetryTaskChildTask(*gin.Context)                          {}
func (stubRouteHandler) ExecuteTaskGenerationAction(*gin.Context)                 {}
func (stubRouteHandler) GetTaskRevisionHistory(*gin.Context)                      {}
func (stubRouteHandler) GetTaskRevisionHistoryDetail(*gin.Context)                {}
func (stubRouteHandler) GetTaskExport(*gin.Context)                               {}
func (stubRouteHandler) ApplyTaskRevision(*gin.Context)                           {}
func (stubRouteHandler) ValidateTaskRevision(*gin.Context)                        {}
func (stubRouteHandler) SubmitTask(*gin.Context)                                  {}
func (stubRouteHandler) RefreshSubmissionStatus(*gin.Context)                     {}
func (stubRouteHandler) PreviewSheinPrice(*gin.Context)                           {}
func (stubRouteHandler) SearchSheinCategories(*gin.Context)                       {}
func (stubRouteHandler) UpdateSheinFinalDraft(*gin.Context)                       {}
func (stubRouteHandler) GetSubmissionEvents(*gin.Context)                         {}
func (stubRouteHandler) ClearSheinResolutionCache(*gin.Context)                   {}
func (stubRouteHandler) ListSheinEnrollmentDashboard(*gin.Context)                {}
func (stubRouteHandler) TriggerSheinStoreSync(*gin.Context)                       {}
func (stubRouteHandler) SyncSheinSourceSDSProduct(*gin.Context)                   {}
func (stubRouteHandler) GetSheinEnrollmentStoreSummary(*gin.Context)              {}
func (stubRouteHandler) ListSheinSyncedProducts(*gin.Context)                     {}
func (stubRouteHandler) UpdateSheinSyncedProductCost(*gin.Context)                {}
func (stubRouteHandler) ListSheinSDSCostGroups(*gin.Context)                      {}
func (stubRouteHandler) ListSheinSourceSDSCostGroups(*gin.Context)                {}
func (stubRouteHandler) ListSheinSourceSDSMetadata(*gin.Context)                  {}
func (stubRouteHandler) UpdateSheinSDSCostGroup(*gin.Context)                     {}
func (stubRouteHandler) GetSheinActivityStrategy(*gin.Context)                    {}
func (stubRouteHandler) UpdateSheinActivityStrategy(*gin.Context)                 {}
func (stubRouteHandler) RefreshSheinActivityCandidates(*gin.Context)              {}
func (stubRouteHandler) ListSheinActivityCandidates(*gin.Context)                 {}
func (stubRouteHandler) ResetSheinActivityCandidates(*gin.Context)                {}
func (stubRouteHandler) ReviewSheinActivityCandidate(*gin.Context)                {}
func (stubRouteHandler) ExecuteSheinActivityEnrollment(*gin.Context)              {}
func (stubRouteHandler) ListSheinActivityEnrollmentRuns(*gin.Context)             {}
func (stubRouteHandler) ListSheinActivityEnrollmentRunItems(*gin.Context)         {}
func (stubRouteHandler) LookupSheinPODImages(*gin.Context)                        {}
func (stubRouteHandler) ListSettingsNamespaces(*gin.Context)                      {}
func (stubRouteHandler) GetSettingsHealth(*gin.Context)                           {}
func (stubRouteHandler) GetSettingsNamespaceSchema(*gin.Context)                  {}
func (stubRouteHandler) GetSettingsNamespace(*gin.Context)                        {}
func (stubRouteHandler) UpdateSettingsNamespace(*gin.Context)                     {}
func (stubRouteHandler) ListSheinStoreProfiles(*gin.Context)                      {}
func (stubRouteHandler) UpsertSheinStoreProfile(*gin.Context)                     {}
func (stubRouteHandler) DeleteSheinStoreProfile(*gin.Context)                     {}
func (stubRouteHandler) GetSheinSettings(*gin.Context)                            {}
func (stubRouteHandler) UpdateSheinSettings(*gin.Context)                         {}
func (stubRouteHandler) GetAIClientSettings(*gin.Context)                         {}
func (stubRouteHandler) UpdateAIClientSettings(*gin.Context)                      {}
func (stubRouteHandler) ListTenantStores(*gin.Context)                            {}
func (stubRouteHandler) ListSimpleTenantStores(*gin.Context)                      {}
func (stubRouteHandler) CreateTenantStore(*gin.Context)                           {}
func (stubRouteHandler) UpdateTenantStore(*gin.Context)                           {}
func (stubRouteHandler) DeleteTenantStore(*gin.Context)                           {}
func (stubRouteHandler) GetCurrentSubscription(*gin.Context)                      {}
func (stubRouteHandler) ListSubscriptionModules(*gin.Context)                     {}
func (stubRouteHandler) ListSubscriptionEntitlements(*gin.Context)                {}
func (stubRouteHandler) UpsertSubscriptionEntitlement(*gin.Context)               {}
func (stubRouteHandler) ListPlatformTenantSubscriptions(*gin.Context)             {}
func (stubRouteHandler) ListPlatformSubscriptionPlans(*gin.Context)               {}
func (stubRouteHandler) UpsertPlatformSubscriptionPlan(*gin.Context)              {}
func (stubRouteHandler) UpsertPlatformSubscriptionPlanModule(*gin.Context)        {}
func (stubRouteHandler) DeletePlatformSubscriptionPlanModule(*gin.Context)        {}
func (stubRouteHandler) SetPlatformSubscriptionPlanStatus(*gin.Context)           {}
func (stubRouteHandler) ListPlatformSubscriptionPlanTenants(*gin.Context)         {}
func (stubRouteHandler) ListPlatformSubscriptionPlanAuditLogs(*gin.Context)       {}
func (stubRouteHandler) GetPlatformTenantSubscription(*gin.Context)               {}
func (stubRouteHandler) ApplyPlatformTenantSubscriptionPlan(*gin.Context)         {}
func (stubRouteHandler) UpsertPlatformTenantSubscriptionEntitlement(*gin.Context) {}
func (stubRouteHandler) SetPlatformTenantSubscriptionUsage(*gin.Context)          {}
func (stubRouteHandler) ListPlatformTenantSubscriptionAuditLogs(*gin.Context)     {}
func (stubRouteHandler) ListAdminStores(*gin.Context)                             {}
func (stubRouteHandler) GetAdminStore(*gin.Context)                               {}
func (stubRouteHandler) CreateAdminStore(*gin.Context)                            {}
func (stubRouteHandler) UpdateAdminStore(*gin.Context)                            {}
func (stubRouteHandler) UpdateAdminStoreStatus(*gin.Context)                      {}
func (stubRouteHandler) DeleteAdminStore(*gin.Context)                            {}
func (stubRouteHandler) ListSimpleAdminStores(*gin.Context)                       {}
func (stubRouteHandler) ListAdminStoreStatistics(*gin.Context)                    {}
func (stubRouteHandler) GetAdminDispatchEventSummary(*gin.Context)                {}
func (stubRouteHandler) ListAdminDispatchEvents(*gin.Context)                     {}
func (stubRouteHandler) ListDeletedAdminStores(*gin.Context)                      {}
func (stubRouteHandler) RestoreAdminStore(*gin.Context)                           {}
func (stubRouteHandler) PermanentlyDeleteAdminStore(*gin.Context)                 {}
func (stubRouteHandler) ExtendAdminStoreValidity(*gin.Context)                    {}
func (stubRouteHandler) ListAdminImportTasks(*gin.Context)                        {}
func (stubRouteHandler) BatchCreateAdminImportTasks(*gin.Context)                 {}
func (stubRouteHandler) DeleteAdminImportTask(*gin.Context)                       {}
func (stubRouteHandler) ListAdminFilterRules(*gin.Context)                        {}
func (stubRouteHandler) GetAdminFilterRule(*gin.Context)                          {}
func (stubRouteHandler) CreateAdminFilterRule(*gin.Context)                       {}
func (stubRouteHandler) UpdateAdminFilterRule(*gin.Context)                       {}
func (stubRouteHandler) UpdateAdminFilterRuleStatus(*gin.Context)                 {}
func (stubRouteHandler) DeleteAdminFilterRule(*gin.Context)                       {}
func (stubRouteHandler) ListAdminProfitRules(*gin.Context)                        {}
func (stubRouteHandler) GetAdminProfitRule(*gin.Context)                          {}
func (stubRouteHandler) CreateAdminProfitRule(*gin.Context)                       {}
func (stubRouteHandler) UpdateAdminProfitRule(*gin.Context)                       {}
func (stubRouteHandler) UpdateAdminProfitRuleStatus(*gin.Context)                 {}
func (stubRouteHandler) DeleteAdminProfitRule(*gin.Context)                       {}
func (stubRouteHandler) ListAdminPricingRules(*gin.Context)                       {}
func (stubRouteHandler) GetAdminPricingRule(*gin.Context)                         {}
func (stubRouteHandler) CreateAdminPricingRule(*gin.Context)                      {}
func (stubRouteHandler) UpdateAdminPricingRule(*gin.Context)                      {}
func (stubRouteHandler) UpdateAdminPricingRuleStatus(*gin.Context)                {}
func (stubRouteHandler) DeleteAdminPricingRule(*gin.Context)                      {}
func (stubRouteHandler) ListAdminOperationStrategies(*gin.Context)                {}
func (stubRouteHandler) GetAdminOperationStrategy(*gin.Context)                   {}
func (stubRouteHandler) CreateAdminOperationStrategy(*gin.Context)                {}
func (stubRouteHandler) UpdateAdminOperationStrategy(*gin.Context)                {}
func (stubRouteHandler) UpdateAdminOperationStrategyStatus(*gin.Context)          {}
func (stubRouteHandler) DeleteAdminOperationStrategy(*gin.Context)                {}
func (stubRouteHandler) ListAdminScheduledTaskConfigs(*gin.Context)               {}
func (stubRouteHandler) GetAdminScheduledTaskConfig(*gin.Context)                 {}
func (stubRouteHandler) UpsertAdminScheduledTaskConfig(*gin.Context)              {}
func (stubRouteHandler) UpdateAdminScheduledTaskConfigStatus(*gin.Context)        {}
func (stubRouteHandler) DeleteAdminScheduledTaskConfig(*gin.Context)              {}
func (stubRouteHandler) ListAdminSensitiveWords(*gin.Context)                     {}
func (stubRouteHandler) GetAdminSensitiveWord(*gin.Context)                       {}
func (stubRouteHandler) CreateAdminSensitiveWord(*gin.Context)                    {}
func (stubRouteHandler) UpdateAdminSensitiveWord(*gin.Context)                    {}
func (stubRouteHandler) UpdateAdminSensitiveWordStatus(*gin.Context)              {}
func (stubRouteHandler) DeleteAdminSensitiveWord(*gin.Context)                    {}
func (stubRouteHandler) ListAdminGenerationTopicCatalog(*gin.Context)             {}
func (stubRouteHandler) ListAdminGenerationTopicOverrides(*gin.Context)           {}
func (stubRouteHandler) GetAdminGenerationTopicOverride(*gin.Context)             {}
func (stubRouteHandler) CreateAdminGenerationTopicOverride(*gin.Context)          {}
func (stubRouteHandler) UpdateAdminGenerationTopicOverride(*gin.Context)          {}
func (stubRouteHandler) UpdateAdminGenerationTopicOverrideStatus(*gin.Context)    {}
func (stubRouteHandler) DeleteAdminGenerationTopicOverride(*gin.Context)          {}
func (stubRouteHandler) ListAdminGenerationTopicPolicies(*gin.Context)            {}
func (stubRouteHandler) GetAdminGenerationTopicPolicy(*gin.Context)               {}
func (stubRouteHandler) CreateAdminGenerationTopicPolicy(*gin.Context)            {}
func (stubRouteHandler) UpdateAdminGenerationTopicPolicy(*gin.Context)            {}
func (stubRouteHandler) UpdateAdminGenerationTopicPolicyStatus(*gin.Context)      {}
func (stubRouteHandler) DeleteAdminGenerationTopicPolicy(*gin.Context)            {}
func (stubRouteHandler) ListAdminProductImportMappings(*gin.Context)              {}
func (stubRouteHandler) GetAdminProductImportMapping(*gin.Context)                {}
func (stubRouteHandler) CreateAdminProductImportMapping(*gin.Context)             {}
func (stubRouteHandler) UpdateAdminProductImportMapping(*gin.Context)             {}
func (stubRouteHandler) UpdateAdminProductImportMappingStatus(*gin.Context)       {}
func (stubRouteHandler) DeleteAdminProductImportMapping(*gin.Context)             {}
func (stubRouteHandler) ListAdminCategories(*gin.Context)                         {}
func (stubRouteHandler) GetAdminCategory(*gin.Context)                            {}
func (stubRouteHandler) CreateAdminCategory(*gin.Context)                         {}
func (stubRouteHandler) UpdateAdminCategory(*gin.Context)                         {}
func (stubRouteHandler) UpdateAdminCategoryStatus(*gin.Context)                   {}
func (stubRouteHandler) DeleteAdminCategory(*gin.Context)                         {}
func (stubRouteHandler) ListAdminProductData(*gin.Context)                        {}
func (stubRouteHandler) GetAdminProductData(*gin.Context)                         {}
func (stubRouteHandler) CreateAdminProductData(*gin.Context)                      {}
func (stubRouteHandler) UpdateAdminProductData(*gin.Context)                      {}
func (stubRouteHandler) UpdateAdminProductDataStatus(*gin.Context)                {}
func (stubRouteHandler) DeleteAdminProductData(*gin.Context)                      {}

func routeKeys(routes []httproute.Descriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	return keys
}

type stubWorkerPool struct{}

func (stubWorkerPool) Start(context.Context)            {}
func (stubWorkerPool) Stop(context.Context)             {}
func (stubWorkerPool) Submit(worker.WorkerJob) error    { return nil }
func (stubWorkerPool) AvailableSlots() int              { return 0 }
func (stubWorkerPool) GetQueueStats() worker.QueueStats { return worker.QueueStats{} }
func (stubWorkerPool) SetJobHandler(worker.JobHandler)  {}
func (stubWorkerPool) GetMetrics() *worker.Metrics      { return nil }
