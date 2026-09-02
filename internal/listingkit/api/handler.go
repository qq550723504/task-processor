package api

import (
	"context"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/memberinvite"
	"task-processor/internal/listingkit/sheinpodimage"
	"task-processor/internal/listingkit/tenantdirectory"
	"task-processor/internal/listingkit/zitadelsms"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
)

type handler struct {
	taskLifecycleService          listingkit.TaskLifecycleService
	taskRecoveryService           listingkit.TaskRecoveryService
	taskRequeueService            listingkit.TaskRequeueService
	sdsBaselineWarmService        listingkit.SDSBaselineWarmService
	sdsRetirementService          listingkit.SDSRetirementService
	childTaskRetryService         childTaskRetryService
	taskSDSRepairService          taskSDSRepairService
	uploadedImageService          uploadedImageService
	uploadedImageDeleteService    uploadedImageDeleteService
	storeAdminService             storeAdminHandlerService
	storeRepository               listingadmin.StoreRepository
	sheinSyncService              listingkit.SheinSyncService
	sheinCandidateService         listingkit.SheinCandidateService
	sheinEnrollmentService        listingkit.SheinEnrollmentService
	sheinSyncRepository           listingkit.SheinSyncRepository
	sheinPODImageLookupService    sheinpodimage.SheinPODImageLookupRepository
	operationStrategyRepository   listingadmin.OperationStrategyRepository
	scheduledTaskConfigRepository listingadmin.ScheduledTaskConfigRepository
	initErr                       error
	adminHandlers
	subscriptionDependencies
	settingsService                 settingsNamespaceService
	zitadelSMSService               *zitadelsms.Service
}

type storeAdminHandlers struct {
	storeHandler            *listingadmin.StoreHandler
	storeStatisticsHandler  *listingadmin.StoreStatisticsHandler
	dispatchEventRepository listingadmin.DispatchEventRepository
	importTaskHandler       *listingadmin.ImportTaskHandler
}

type catalogAdminHandlers struct {
	filterRuleHandler              *listingadmin.FilterRuleHandler
	profitRuleHandler              *listingadmin.ProfitRuleHandler
	pricingRuleHandler             *listingadmin.PricingRuleHandler
	operationStrategyHandler       *listingadmin.OperationStrategyHandler
	scheduledTaskConfigHandler     *listingadmin.ScheduledTaskConfigHandler
	sensitiveWordHandler           *listingadmin.SensitiveWordHandler
	generationTopicCatalogHandler  *listingadmin.GenerationTopicCatalogHandler
	generationTopicPolicyHandler   *listingadmin.GenerationTopicPolicyHandler
	generationTopicOverrideHandler *listingadmin.GenerationTopicOverrideHandler
	productImportMappingHandler    *listingadmin.ProductImportMappingHandler
	categoryHandler                *listingadmin.CategoryHandler
	productDataHandler             *listingadmin.ProductDataHandler
}

type adminHandlers struct {
	storeAdminHandlers
	catalogAdminHandlers
}

type subscriptionDependencies struct {
	subscriptionService      *listingsubscription.Service
	subscriptionHandler      *listingsubscription.Handler
	generationUsageAdmission listingkit.GenerationUsageAdmission
	platformAdminUsers       []string
	platformAdminRoles       []string
	tenantDirectory          tenantdirectory.Directory
	memberInvitationService  *memberinvite.Service
	memberInvitationAudit    memberinvite.AuditRepository
}

type handlerCoreService interface {
	listingkit.TaskLifecycleService
	uploadedImageService
}

type HandlerService interface {
	handlerCoreService
}

type storeAdminHandlerService interface {
	ListSheinStoreProfiles(ctx context.Context) ([]listingkit.ListingKitStoreProfile, error)
	UpsertSheinStoreProfile(ctx context.Context, req *listingkit.ListingKitStoreProfile) (*listingkit.ListingKitStoreProfile, error)
	DeleteSheinStoreProfile(ctx context.Context, id int64) error
	PreviewSheinPrice(ctx context.Context, taskID string, req *listingkit.SheinPricePreviewRequest) (*sheinpub.PricingReview, error)
	SearchSheinCategories(ctx context.Context, taskID string, query string) (*listingkit.SheinCategorySearchResult, error)
	UpdateSheinFinalDraft(ctx context.Context, taskID string, req *listingkit.SheinFinalDraftUpdateRequest) (*listingkit.ListingKitPreview, error)
	GetSubmissionEvents(ctx context.Context, taskID string) (*listingkit.SheinSubmissionEventPage, error)
	ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error)
}

type childTaskRetryService interface {
	ScheduleTaskChildRetry(ctx context.Context, taskID string, req *listingkit.RetryChildTaskRequest) (*listingkit.TaskChildRetryAccepted, error)
}

type taskSDSRepairService interface {
	GetTaskSDSRepair(ctx context.Context, taskID string) (*listingkit.TaskSDSRepairSession, error)
	RepairAndRetryTaskSDS(ctx context.Context, taskID string, req *listingkit.ApplyTaskSDSRepairRequest) (*listingkit.TaskResult, error)
}

type uploadedImageDeleteService interface {
	DeleteUploadedImage(ctx context.Context, key string) (*listingkit.DeletedUploadedImage, error)
}

type uploadedImageService interface {
	UploadImages(ctx context.Context, req *listingkit.UploadImagesRequest) (*listingkit.UploadImagesResponse, error)
	GetUploadedImage(ctx context.Context, key string) (*listingkit.UploadedImageFile, error)
}

type HandlerOption func(*handler)

type AdminHandlerDependencies struct {
	StoreRepository                   listingadmin.StoreRepository
	StoreStatisticsRepository         listingadmin.StoreStatisticsRepository
	DispatchEventRepository           listingadmin.DispatchEventRepository
	ImportTaskRepository              listingadmin.ImportTaskRepository
	FilterRuleRepository              listingadmin.FilterRuleRepository
	ProfitRuleRepository              listingadmin.ProfitRuleRepository
	PricingRuleRepository             listingadmin.PricingRuleRepository
	OperationStrategyRepository       listingadmin.OperationStrategyRepository
	ScheduledTaskConfigRepository     listingadmin.ScheduledTaskConfigRepository
	SensitiveWordRepository           listingadmin.SensitiveWordRepository
	GenerationTopicOverrideRepository listingadmin.GenerationTopicOverrideRepository
	GenerationTopicPolicyRepository   listingadmin.GenerationTopicPolicyRepository
	ProductImportMappingRepository    listingadmin.ProductImportMappingRepository
	CategoryRepository                listingadmin.CategoryRepository
	ProductDataRepository             listingadmin.ProductDataRepository
}

type SubscriptionDependencies struct {
	Service                         *listingsubscription.Service
	GenerationUsageAdmission        listingkit.GenerationUsageAdmission
	PlatformAdminUsers              []string
	PlatformAdminRoles              []string
	TenantDirectory                 tenantdirectory.Directory
	MemberInvitationProvider        memberinvite.Provider
	MemberInvitationAuditRepository memberinvite.AuditRepository
}

type HandlerDependencies struct {
	Admin                    AdminHandlerDependencies
	Subscription             SubscriptionDependencies
	ZitadelSMSService        *zitadelsms.Service
}

func withHandlerState(apply func(*handler)) HandlerOption {
	return func(h *handler) {
		if h == nil || apply == nil {
			return
		}
		apply(h)
	}
}

func withAdminHandlers(apply func(*adminHandlers)) HandlerOption {
	return withHandlerState(func(h *handler) {
		apply(&h.adminHandlers)
	})
}

func withAdminDependency[Repo comparable](repo Repo, apply func(Repo, *adminHandlers)) HandlerOption {
	return withAdminHandlers(func(admin *adminHandlers) {
		var zero Repo
		if repo == zero {
			return
		}
		apply(repo, admin)
	})
}

func WithDependencies(deps HandlerDependencies) HandlerOption {
	options := []HandlerOption{
		withStoreAdminDependencies(deps.Admin),
		withCatalogAdminDependencies(deps.Admin),
		withSubscriptionConfig(deps.Subscription),
		WithZitadelSMSService(deps.ZitadelSMSService),
	}
	return func(h *handler) {
		for _, option := range options {
			if option != nil {
				option(h)
			}
		}
	}
}

func WithZitadelSMSService(service *zitadelsms.Service) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.zitadelSMSService = service
	})
}

func WithTaskLifecycleService(service listingkit.TaskLifecycleService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.taskLifecycleService = service
	})
}

func WithUploadedImageService(service uploadedImageService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.uploadedImageService = service
	})
}

func WithTaskRecoveryService(service listingkit.TaskRecoveryService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.taskRecoveryService = service
	})
}

func WithTaskRequeueService(service listingkit.TaskRequeueService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.taskRequeueService = service
	})
}

func WithUploadedImageDeleteService(service uploadedImageDeleteService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.uploadedImageDeleteService = service
	})
}

func WithChildTaskRetryService(service childTaskRetryService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.childTaskRetryService = service
	})
}

func WithSDSBaselineWarmService(service listingkit.SDSBaselineWarmService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.sdsBaselineWarmService = service
	})
}

func WithSDSRetirementService(service listingkit.SDSRetirementService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.sdsRetirementService = service
	})
}

func WithSheinPODImageLookupService(service sheinpodimage.SheinPODImageLookupRepository) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.sheinPODImageLookupService = service
	})
}

func WithStoreAdminService(service storeAdminHandlerService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.storeAdminService = service
	})
}

func WithSettingsHandlerService(service settingsHandlerService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.attachSettingsService(service)
	})
}

func WithSheinSyncServices(
	syncService listingkit.SheinSyncService,
	candidateService listingkit.SheinCandidateService,
	enrollmentService listingkit.SheinEnrollmentService,
) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.sheinSyncService = syncService
		h.sheinCandidateService = candidateService
		h.sheinEnrollmentService = enrollmentService
	})
}

func WithSheinSyncRepository(repo listingkit.SheinSyncRepository) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.sheinSyncRepository = repo
	})
}
