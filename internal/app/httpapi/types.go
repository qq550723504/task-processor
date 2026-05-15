package httpapi

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"task-processor/internal/amazonlisting"
	appbootstrap "task-processor/internal/app/bootstrap"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingkit"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/taskrpcapi"
)

type Options struct {
	ConfigPath     string
	Port           int
	ShutdownSignal chan os.Signal
}

type runtimeDeps struct {
	cfg                   *config.Config
	closers               []func() error
	openaiMgr             *openaiclient.Manager
	aiCredentialStore     *openaiclient.GormCredentialResolver
	llmMgr                productenrich.LLMManager
	inputParser           productenrich.InputParser
	understanding         productenrich.ProductUnderstanding
	imageWorkDir          string
	shared                *appbootstrap.SharedResources
	managementClient      *management.ClientManager
	productService        productenrich.ProductService
	imageService          productimage.Service
	sdsSyncService        sdsusecase.Service
	imageSubjectExtractor productimage.SubjectExtractor
	imageWhiteBgRenderer  productimage.WhiteBackgroundRenderer
	imageSceneRenderer    productimage.SceneRenderer
}

type appBootstrap struct {
	productHandler       productenrich.ProductHandler
	imageHandler         productimage.Handler
	amazonListingHandler amazonlisting.Handler
	listingKitHandler    listingkit.Handler
	studioSessionHandler listingkit.StudioSessionHandler
	sdsCatalogHandler    sdsCatalogRouteHandler
	sheinLoginHandler    sheinLoginRouteHandler
	sdsLoginHandler      sdsLoginRouteHandler
	taskRPCHandler       taskrpcapi.Handler
	server               *http.Server
	routes               []routeDescriptor
	pools                []worker.WorkerPool
	closers              []func() error
}

type productModule struct {
	handler productenrich.ProductHandler
	pool    worker.WorkerPool
}

type imageModule struct {
	handler productimage.Handler
	pool    worker.WorkerPool
}

type amazonListingModule struct {
	handler amazonlisting.Handler
	pool    worker.WorkerPool
}

type listingKitModule struct {
	handler              listingkit.Handler
	studioSessionHandler listingkit.StudioSessionHandler
	pool                 worker.WorkerPool
}

type productRouteHandler interface {
	GenerateProduct(c *gin.Context)
	GetTaskResult(c *gin.Context)
}

type imageRouteHandler interface {
	ProcessImages(c *gin.Context)
	GetTaskResult(c *gin.Context)
	ReviewTask(c *gin.Context)
}

type amazonListingRouteHandler interface {
	GenerateListing(c *gin.Context)
	ListTaskQueue(c *gin.Context)
	GetTaskResult(c *gin.Context)
	GetTaskWorkbench(c *gin.Context)
	ReviewTask(c *gin.Context)
	SubmitTask(c *gin.Context)
}

type listingKitRouteHandler interface {
	GenerateListingKit(c *gin.Context)
	GenerateStudioDesigns(c *gin.Context)
	GenerateStudioProductImages(c *gin.Context)
	StartStudioAsyncJob(c *gin.Context)
	GetStudioAsyncJob(c *gin.Context)
	RegenerateSheinDataImage(c *gin.Context)
	UploadListingKitImages(c *gin.Context)
	GetUploadedListingKitImage(c *gin.Context)
	ListTasks(c *gin.Context)
	GetTaskResult(c *gin.Context)
	GetTaskPreview(c *gin.Context)
	GetTaskGenerationTasks(c *gin.Context)
	GetTaskGenerationQueue(c *gin.Context)
	GetTaskGenerationReviewSession(c *gin.Context)
	GetTaskGenerationReviewPreview(c *gin.Context)
	DispatchTaskGenerationNavigation(c *gin.Context)
	RetryTaskGenerationTasks(c *gin.Context)
	ExecuteTaskGenerationAction(c *gin.Context)
	GetTaskRevisionHistory(c *gin.Context)
	GetTaskRevisionHistoryDetail(c *gin.Context)
	GetTaskExport(c *gin.Context)
	ApplyTaskRevision(c *gin.Context)
	ValidateTaskRevision(c *gin.Context)
	SubmitTask(c *gin.Context)
	RefreshSubmissionStatus(c *gin.Context)
	GetSheinSettings(c *gin.Context)
	UpdateSheinSettings(c *gin.Context)
	GetAIClientSettings(c *gin.Context)
	UpdateAIClientSettings(c *gin.Context)
	ListAdminStores(c *gin.Context)
	GetAdminStore(c *gin.Context)
	CreateAdminStore(c *gin.Context)
	UpdateAdminStore(c *gin.Context)
	UpdateAdminStoreStatus(c *gin.Context)
	DeleteAdminStore(c *gin.Context)
	ListSimpleAdminStores(c *gin.Context)
	ListAdminStoreStatistics(c *gin.Context)
	ListDeletedAdminStores(c *gin.Context)
	RestoreAdminStore(c *gin.Context)
	PermanentlyDeleteAdminStore(c *gin.Context)
	ExtendAdminStoreValidity(c *gin.Context)
	ListAdminImportTasks(c *gin.Context)
	BatchCreateAdminImportTasks(c *gin.Context)
	DeleteAdminImportTask(c *gin.Context)
	ListAdminFilterRules(c *gin.Context)
	GetAdminFilterRule(c *gin.Context)
	CreateAdminFilterRule(c *gin.Context)
	UpdateAdminFilterRule(c *gin.Context)
	UpdateAdminFilterRuleStatus(c *gin.Context)
	DeleteAdminFilterRule(c *gin.Context)
	ListAdminProfitRules(c *gin.Context)
	GetAdminProfitRule(c *gin.Context)
	CreateAdminProfitRule(c *gin.Context)
	UpdateAdminProfitRule(c *gin.Context)
	UpdateAdminProfitRuleStatus(c *gin.Context)
	DeleteAdminProfitRule(c *gin.Context)
	ListAdminPricingRules(c *gin.Context)
	GetAdminPricingRule(c *gin.Context)
	CreateAdminPricingRule(c *gin.Context)
	UpdateAdminPricingRule(c *gin.Context)
	UpdateAdminPricingRuleStatus(c *gin.Context)
	DeleteAdminPricingRule(c *gin.Context)
	ListAdminOperationStrategies(c *gin.Context)
	GetAdminOperationStrategy(c *gin.Context)
	CreateAdminOperationStrategy(c *gin.Context)
	UpdateAdminOperationStrategy(c *gin.Context)
	UpdateAdminOperationStrategyStatus(c *gin.Context)
	DeleteAdminOperationStrategy(c *gin.Context)
	ListAdminSensitiveWords(c *gin.Context)
	GetAdminSensitiveWord(c *gin.Context)
	CreateAdminSensitiveWord(c *gin.Context)
	UpdateAdminSensitiveWord(c *gin.Context)
	UpdateAdminSensitiveWordStatus(c *gin.Context)
	DeleteAdminSensitiveWord(c *gin.Context)
	ListAdminProductImportMappings(c *gin.Context)
	GetAdminProductImportMapping(c *gin.Context)
	CreateAdminProductImportMapping(c *gin.Context)
	UpdateAdminProductImportMapping(c *gin.Context)
	UpdateAdminProductImportMappingStatus(c *gin.Context)
	DeleteAdminProductImportMapping(c *gin.Context)
	ListAdminCategories(c *gin.Context)
	GetAdminCategory(c *gin.Context)
	CreateAdminCategory(c *gin.Context)
	UpdateAdminCategory(c *gin.Context)
	UpdateAdminCategoryStatus(c *gin.Context)
	DeleteAdminCategory(c *gin.Context)
	ListAdminProductData(c *gin.Context)
	GetAdminProductData(c *gin.Context)
	CreateAdminProductData(c *gin.Context)
	UpdateAdminProductData(c *gin.Context)
	UpdateAdminProductDataStatus(c *gin.Context)
	DeleteAdminProductData(c *gin.Context)
	PreviewSheinPrice(c *gin.Context)
	SearchSheinCategories(c *gin.Context)
	UpdateSheinFinalDraft(c *gin.Context)
	GetSubmissionEvents(c *gin.Context)
	ClearSheinResolutionCache(c *gin.Context)
}

type studioSessionRouteHandler interface {
	EnsureStudioSession(c *gin.Context)
	GetStudioSession(c *gin.Context)
	UpdateStudioSession(c *gin.Context)
	ReplaceStudioSessionDesigns(c *gin.Context)
	ListStudioSessionGallery(c *gin.Context)
}

type sdsCatalogRouteHandler interface {
	ListSDSProducts(c *gin.Context)
	GetSDSProduct(c *gin.Context)
	ListSDSCategories(c *gin.Context)
	ListSDSShipmentAreas(c *gin.Context)
}

type taskRPCRouteHandler interface {
	GetTaskStatus(c *gin.Context)
	RetryTask(c *gin.Context)
	CancelTask(c *gin.Context)
	GetQueueStats(c *gin.Context)
	GetHealth(c *gin.Context)
}

type sheinLoginRouteHandler interface {
	Health(c *gin.Context)
	ListAccounts(c *gin.Context)
	Login(c *gin.Context)
	Status(c *gin.Context)
	SubmitVerifyCode(c *gin.Context)
	CancelVerifyCodeWait(c *gin.Context)
	ClearCookie(c *gin.Context)
	GetLastFailure(c *gin.Context)
	ClearLastFailure(c *gin.Context)
}

type sdsLoginRouteHandler interface {
	Health(c *gin.Context)
	Status(c *gin.Context)
	Login(c *gin.Context)
	ManualLogin(c *gin.Context)
	GetAuthState(c *gin.Context)
	ClearState(c *gin.Context)
	AdminPage(c *gin.Context)
}

type routeDescriptor struct {
	Method  string
	Path    string
	Module  string
	Handler gin.HandlerFunc
}
