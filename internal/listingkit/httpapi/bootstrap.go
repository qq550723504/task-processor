package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	"task-processor/internal/httpbootstrap"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	listingkitapi "task-processor/internal/listingkit/api"
	"task-processor/internal/listingkit/reviewstore"
	"task-processor/internal/listingsubscription"
	productenrich "task-processor/internal/productenrich"
	productimage "task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
	sdsusecase "task-processor/internal/sds/usecase"
)

type Module struct {
	Handler              RouteHandler
	StudioSessionHandler listingkit.StudioSessionHandler
	Pool                 worker.WorkerPool
	Closers              []func() error
}

type ServiceBundle struct {
	TemporalWorkerService          TemporalWorkerService
	TaskRepository                 listingkit.Repository
	StudioAsyncJobRepository       listingkit.StudioAsyncJobRepository
	StoreRepository                listingadmin.StoreRepository
	StoreStatisticsRepository      listingadmin.StoreStatisticsRepository
	ImportTaskRepository           listingadmin.ImportTaskRepository
	FilterRuleRepository           listingadmin.FilterRuleRepository
	ProfitRuleRepository           listingadmin.ProfitRuleRepository
	PricingRuleRepository          listingadmin.PricingRuleRepository
	OperationStrategyRepository    listingadmin.OperationStrategyRepository
	SensitiveWordRepository        listingadmin.SensitiveWordRepository
	ProductImportMappingRepository listingadmin.ProductImportMappingRepository
	CategoryRepository             listingadmin.CategoryRepository
	ProductDataRepository          listingadmin.ProductDataRepository
	SubscriptionService            *listingsubscription.Service
	Closers                        []func() error

	runtime serviceBundleRuntime
}

type serviceBundleRuntime struct {
	temporalWorkerService TemporalWorkerService
	taskRepository        listingkit.Repository
	service               moduleService
	handlerDependencies   listingkitapi.HandlerDependencies
	closers               []func() error
}

type TemporalWorkerService interface {
	listingkit.SheinPublishActivityHostSource
	listingkit.LayerWorkflowActivityHostSource
}

type moduleService interface {
	listingkitapi.HandlerService
	listingkit.InternalListingKitService
	listingkit.TaskSubmitterConfigurer
	listingkit.StudioSessionHandlerService
	listingkit.WorkflowClientConfigurer
	TemporalWorkerService
}

type aiCredentialStore interface {
	listingkit.AIClientCredentialStore
	openaiclient.ClientConfigResolver
}

type sheinManagementStoreCatalog struct {
	repo listingadmin.StoreRepository
}

func (c sheinManagementStoreCatalog) GetStoreInfo(ctx context.Context, tenantID, storeID int64) (*listingkit.SheinStoreInfo, error) {
	if c.repo == nil {
		return nil, fmt.Errorf("listing admin store repository is not configured")
	}
	store, err := c.repo.GetStore(ctx, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	if store == nil || store.ID <= 0 {
		return nil, fmt.Errorf("store info is unavailable")
	}
	return &listingkit.SheinStoreInfo{
		ID:       store.ID,
		TenantID: store.TenantID,
		StoreID:  strings.TrimSpace(store.StoreID),
		Name:     strings.TrimSpace(store.Name),
		Platform: strings.TrimSpace(store.Platform),
		Region:   strings.TrimSpace(store.Region),
		LoginURL: strings.TrimSpace(store.LoginURL),
		Proxy:    strings.TrimSpace(store.Proxy),
	}, nil
}

func (c sheinManagementStoreCatalog) ListStoreOptions(ctx context.Context, tenantID int64) ([]listingkit.SheinStoreOption, error) {
	if c.repo == nil {
		return nil, fmt.Errorf("listing admin store repository is not configured")
	}
	page, err := c.repo.ListStores(ctx, listingadmin.StoreQuery{
		TenantID: tenantID,
		Platform: "shein",
		Page:     1,
		PageSize: 200,
	})
	if err != nil || page == nil || len(page.Items) == 0 {
		return nil, err
	}
	options := make([]listingkit.SheinStoreOption, 0, len(page.Items))
	for _, item := range page.Items {
		if item.ID <= 0 {
			continue
		}
		options = append(options, listingkit.SheinStoreOption{
			ID:       item.ID,
			StoreID:  strings.TrimSpace(item.StoreID),
			Name:     strings.TrimSpace(item.Name),
			Platform: strings.TrimSpace(item.Platform),
			Region:   strings.TrimSpace(item.Region),
		})
	}
	return options, nil
}

type BuildModuleInput struct {
	ServiceInput                       BuildServiceInput
	ShouldStartTemporalWorkerInProcess bool
}

type AdminRepositoryBuilders struct {
	Store                func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error)
	StoreStatistics      func(*config.Config, *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error)
	ImportTask           func(*config.Config, *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error)
	FilterRule           func(*config.Config, *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error)
	ProfitRule           func(*config.Config, *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error)
	PricingRule          func(*config.Config, *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error)
	OperationStrategy    func(*config.Config, *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error)
	SensitiveWord        func(*config.Config, *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error)
	ProductImportMapping func(*config.Config, *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error)
	Category             func(*config.Config, *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error)
	ProductData          func(*config.Config, *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error)
}

type CoreRepositoryBuilders struct {
	Task                 func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error)
	StudioAsyncJob       func(*config.Config, *logrus.Logger) (listingkit.StudioAsyncJobRepository, []func() error, error)
	Subscription         func(*config.Config, *logrus.Logger) (listingsubscription.Repository, []func() error, error)
	Asset                func(*config.Config, *logrus.Logger) (assetrepo.Repository, []func() error, error)
	Review               func(*config.Config, *logrus.Logger) (reviewstore.Repository, []func() error, error)
	StudioSession        func(*config.Config, *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error)
	UploadedImage        func(*config.Config, *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error)
	StoreProfile         func(*config.Config, *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error)
	StoreRoutingSettings func(*config.Config, *logrus.Logger) (listingkit.StoreRoutingSettingsRepository, []func() error, error)
	SheinResolutionCache func(*config.Config, *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error)
}

type BuildServiceRepositories struct {
	Core  CoreRepositoryBuilders
	Admin AdminRepositoryBuilders
}

type BuildServiceHooks struct {
	SheinPricingPolicyBuilder         func(*config.Config) sheinpub.PricingPolicy
	ImageUploadStoreBuilder           func(*config.Config, *logrus.Logger) listingkit.ImageUploadStore
	LegacyTenantResolverConfigurator  func(*config.Config, *logrus.Logger) (func() error, error)
	SheinCategoryLLMClientBuilder     func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter
	SheinSaleAttributeLLMBuilder      func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter
	SheinCategoryResolverBuilder      func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver
	SheinAttributeResolverBuilder     func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.AttributeResolver
	SheinSaleAttributeResolverBuilder func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.SaleAttributeResolver
	SheinProductAPIBuilderFactory     func(listingadmin.StoreRepository) sheinpub.ProductAPIBuilder
	SheinImageAPIBuilderFactory       func(listingadmin.StoreRepository) sheinpub.ImageAPIBuilder
	SheinTranslateAPIBuilderFactory   func(listingadmin.StoreRepository) sheinpub.TranslateAPIBuilder
	SheinAPIClientFactoryBuilder      func(listingadmin.StoreRepository) listingkit.SheinAPIClientFactory
	StudioImageGeneratorBuilder       func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ImageGenerator
	DefaultSheinStoreIDResolver       func([]int64) int64
	ConfigureZitadelAuth              func(config.ListingKitZitadelConfig)
	ConfigureAuthorization            func([]string, []string) error
}

func (b CoreRepositoryBuilders) Validate() error {
	switch {
	case b.Task == nil:
		return fmt.Errorf("core repository builder task is required")
	case b.StudioAsyncJob == nil:
		return fmt.Errorf("core repository builder studio async job is required")
	case b.Subscription == nil:
		return fmt.Errorf("core repository builder subscription is required")
	case b.Asset == nil:
		return fmt.Errorf("core repository builder asset is required")
	case b.Review == nil:
		return fmt.Errorf("core repository builder review is required")
	case b.StudioSession == nil:
		return fmt.Errorf("core repository builder studio session is required")
	case b.UploadedImage == nil:
		return fmt.Errorf("core repository builder uploaded image is required")
	case b.StoreProfile == nil:
		return fmt.Errorf("core repository builder store profile is required")
	case b.StoreRoutingSettings == nil:
		return fmt.Errorf("core repository builder store routing settings is required")
	case b.SheinResolutionCache == nil:
		return fmt.Errorf("core repository builder shein resolution cache is required")
	default:
		return nil
	}
}

func (b AdminRepositoryBuilders) Validate() error {
	switch {
	case b.Store == nil:
		return fmt.Errorf("admin repository builder store is required")
	case b.StoreStatistics == nil:
		return fmt.Errorf("admin repository builder store statistics is required")
	case b.ImportTask == nil:
		return fmt.Errorf("admin repository builder import task is required")
	case b.FilterRule == nil:
		return fmt.Errorf("admin repository builder filter rule is required")
	case b.ProfitRule == nil:
		return fmt.Errorf("admin repository builder profit rule is required")
	case b.PricingRule == nil:
		return fmt.Errorf("admin repository builder pricing rule is required")
	case b.OperationStrategy == nil:
		return fmt.Errorf("admin repository builder operation strategy is required")
	case b.SensitiveWord == nil:
		return fmt.Errorf("admin repository builder sensitive word is required")
	case b.ProductImportMapping == nil:
		return fmt.Errorf("admin repository builder product import mapping is required")
	case b.Category == nil:
		return fmt.Errorf("admin repository builder category is required")
	case b.ProductData == nil:
		return fmt.Errorf("admin repository builder product data is required")
	default:
		return nil
	}
}

func (h BuildServiceHooks) Validate() error {
	switch {
	case h.SheinPricingPolicyBuilder == nil:
		return fmt.Errorf("build service hook shein pricing policy is required")
	case h.ImageUploadStoreBuilder == nil:
		return fmt.Errorf("build service hook image upload store is required")
	case h.LegacyTenantResolverConfigurator == nil:
		return fmt.Errorf("build service hook legacy tenant resolver is required")
	case h.SheinCategoryLLMClientBuilder == nil:
		return fmt.Errorf("build service hook shein category llm client is required")
	case h.SheinSaleAttributeLLMBuilder == nil:
		return fmt.Errorf("build service hook shein sale attribute llm client is required")
	case h.SheinCategoryResolverBuilder == nil:
		return fmt.Errorf("build service hook shein category resolver is required")
	case h.SheinAttributeResolverBuilder == nil:
		return fmt.Errorf("build service hook shein attribute resolver is required")
	case h.SheinSaleAttributeResolverBuilder == nil:
		return fmt.Errorf("build service hook shein sale attribute resolver is required")
	case h.SheinProductAPIBuilderFactory == nil:
		return fmt.Errorf("build service hook shein product api builder is required")
	case h.SheinImageAPIBuilderFactory == nil:
		return fmt.Errorf("build service hook shein image api builder is required")
	case h.SheinTranslateAPIBuilderFactory == nil:
		return fmt.Errorf("build service hook shein translate api builder is required")
	case h.SheinAPIClientFactoryBuilder == nil:
		return fmt.Errorf("build service hook shein api client factory is required")
	case h.StudioImageGeneratorBuilder == nil:
		return fmt.Errorf("build service hook studio image generator is required")
	case h.DefaultSheinStoreIDResolver == nil:
		return fmt.Errorf("build service hook default shein store id resolver is required")
	case h.ConfigureZitadelAuth == nil:
		return fmt.Errorf("build service hook configure zitadel auth is required")
	case h.ConfigureAuthorization == nil:
		return fmt.Errorf("build service hook configure authorization is required")
	default:
		return nil
	}
}

type BuildServiceInput struct {
	Config                     *config.Config
	Logger                     *logrus.Logger
	ProductService             productenrich.ProductService
	ImageService               productimage.Service
	SDSSyncService             sdsusecase.Service
	ImageSubjectExtractor      productimage.SubjectExtractor
	ImageWhiteBackgroundRender productimage.WhiteBackgroundRenderer
	ImageSceneRenderer         productimage.SceneRenderer
	AICredentialStore          aiCredentialStore
	Repositories               BuildServiceRepositories
	Hooks                      BuildServiceHooks
}

type builtRepositories struct {
	taskRepository                 listingkit.Repository
	studioAsyncJobRepository       listingkit.StudioAsyncJobRepository
	storeRepository                listingadmin.StoreRepository
	storeStatisticsRepository      listingadmin.StoreStatisticsRepository
	importTaskRepository           listingadmin.ImportTaskRepository
	filterRuleRepository           listingadmin.FilterRuleRepository
	profitRuleRepository           listingadmin.ProfitRuleRepository
	pricingRuleRepository          listingadmin.PricingRuleRepository
	operationStrategyRepository    listingadmin.OperationStrategyRepository
	sensitiveWordRepository        listingadmin.SensitiveWordRepository
	productImportMappingRepository listingadmin.ProductImportMappingRepository
	categoryRepository             listingadmin.CategoryRepository
	productDataRepository          listingadmin.ProductDataRepository
	subscriptionService            *listingsubscription.Service
	assetRepository                assetrepo.Repository
	reviewRepository               reviewstore.Repository
	studioSessionRepository        listingkit.StudioSessionRepository
	uploadedImageRepository        listingkit.UploadedImageRepository
	storeProfileRepository         listingkit.StoreProfileRepository
	storeRoutingSettingsRepository listingkit.StoreRoutingSettingsRepository
	resolutionCacheStore           sheinpub.ResolutionCacheStore
}

type builtCoreRepositories struct {
	taskRepository           listingkit.Repository
	studioAsyncJobRepository listingkit.StudioAsyncJobRepository
}

type builtLateCoreRepositories struct {
	subscriptionService            *listingsubscription.Service
	assetRepository                assetrepo.Repository
	reviewRepository               reviewstore.Repository
	studioSessionRepository        listingkit.StudioSessionRepository
	uploadedImageRepository        listingkit.UploadedImageRepository
	storeProfileRepository         listingkit.StoreProfileRepository
	storeRoutingSettingsRepository listingkit.StoreRoutingSettingsRepository
	resolutionCacheStore           sheinpub.ResolutionCacheStore
}

type builtAdminRepositories struct {
	storeRepository                listingadmin.StoreRepository
	storeStatisticsRepository      listingadmin.StoreStatisticsRepository
	importTaskRepository           listingadmin.ImportTaskRepository
	filterRuleRepository           listingadmin.FilterRuleRepository
	profitRuleRepository           listingadmin.ProfitRuleRepository
	pricingRuleRepository          listingadmin.PricingRuleRepository
	operationStrategyRepository    listingadmin.OperationStrategyRepository
	sensitiveWordRepository        listingadmin.SensitiveWordRepository
	productImportMappingRepository listingadmin.ProductImportMappingRepository
	categoryRepository             listingadmin.CategoryRepository
	productDataRepository          listingadmin.ProductDataRepository
}

type buildListingKitServiceConfigInput struct {
	input        BuildServiceInput
	repositories *builtRepositories
	submit       submitModule
}

func (in BuildServiceInput) Validate() error {
	if in.Config == nil {
		return fmt.Errorf("build service config is required")
	}
	if err := in.Repositories.Core.Validate(); err != nil {
		return err
	}
	if err := in.Repositories.Admin.Validate(); err != nil {
		return err
	}
	return in.Hooks.Validate()
}

type closerStack struct {
	items []func() error
}

func (s *closerStack) Add(items ...func() error) {
	for _, item := range items {
		if item != nil {
			s.items = append(s.items, item)
		}
	}
}

func (s *closerStack) Snapshot() []func() error {
	return append([]func() error{}, s.items...)
}

func (s *closerStack) Close() error {
	var closeErr error
	for i := len(s.items) - 1; i >= 0; i-- {
		if err := s.items[i](); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}
	return closeErr
}

func buildWithClosers[T any](builder func(*config.Config, *logrus.Logger) (T, []func() error, error), cfg *config.Config, logger *logrus.Logger, closers *closerStack) (T, error) {
	value, items, err := builder(cfg, logger)
	if err != nil {
		var zero T
		return zero, err
	}
	closers.Add(items...)
	return value, nil
}

func buildCoreRepositories(input BuildServiceInput, closers *closerStack) (*builtCoreRepositories, error) {
	repoBuilders := input.Repositories.Core

	taskRepository, err := buildWithClosers(repoBuilders.Task, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	studioAsyncJobRepository, err := buildWithClosers(repoBuilders.StudioAsyncJob, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	return &builtCoreRepositories{
		taskRepository:           taskRepository,
		studioAsyncJobRepository: studioAsyncJobRepository,
	}, nil
}

func buildLateCoreRepositories(input BuildServiceInput, closers *closerStack) (*builtLateCoreRepositories, error) {
	repoBuilders := input.Repositories.Core

	subscriptionRepository, err := buildWithClosers(repoBuilders.Subscription, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	subscriptionService, err := listingsubscription.NewService(subscriptionRepository)
	if err != nil {
		return nil, fmt.Errorf("create listing subscription service: %w", err)
	}
	assetRepository, err := buildWithClosers(repoBuilders.Asset, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	reviewRepository, err := buildWithClosers(repoBuilders.Review, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	studioSessionRepository, err := buildWithClosers(repoBuilders.StudioSession, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	uploadedImageRepository, err := buildWithClosers(repoBuilders.UploadedImage, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	storeProfileRepository, err := buildWithClosers(repoBuilders.StoreProfile, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	storeRoutingSettingsRepository, err := buildWithClosers(repoBuilders.StoreRoutingSettings, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	resolutionCacheStore, err := buildWithClosers(repoBuilders.SheinResolutionCache, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}

	return &builtLateCoreRepositories{
		subscriptionService:            subscriptionService,
		assetRepository:                assetRepository,
		reviewRepository:               reviewRepository,
		studioSessionRepository:        studioSessionRepository,
		uploadedImageRepository:        uploadedImageRepository,
		storeProfileRepository:         storeProfileRepository,
		storeRoutingSettingsRepository: storeRoutingSettingsRepository,
		resolutionCacheStore:           resolutionCacheStore,
	}, nil
}

func buildAdminRepositories(input BuildServiceInput, closers *closerStack) (*builtAdminRepositories, error) {
	repoBuilders := input.Repositories.Admin

	storeRepository, err := buildWithClosers(repoBuilders.Store, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	storeStatisticsRepository, err := buildWithClosers(repoBuilders.StoreStatistics, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	importTaskRepository, err := buildWithClosers(repoBuilders.ImportTask, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	filterRuleRepository, err := buildWithClosers(repoBuilders.FilterRule, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	profitRuleRepository, err := buildWithClosers(repoBuilders.ProfitRule, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	pricingRuleRepository, err := buildWithClosers(repoBuilders.PricingRule, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	operationStrategyRepository, err := buildWithClosers(repoBuilders.OperationStrategy, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	sensitiveWordRepository, err := buildWithClosers(repoBuilders.SensitiveWord, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	productImportMappingRepository, err := buildWithClosers(repoBuilders.ProductImportMapping, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	categoryRepository, err := buildWithClosers(repoBuilders.Category, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	productDataRepository, err := buildWithClosers(repoBuilders.ProductData, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}

	return &builtAdminRepositories{
		storeRepository:                storeRepository,
		storeStatisticsRepository:      storeStatisticsRepository,
		importTaskRepository:           importTaskRepository,
		filterRuleRepository:           filterRuleRepository,
		profitRuleRepository:           profitRuleRepository,
		pricingRuleRepository:          pricingRuleRepository,
		operationStrategyRepository:    operationStrategyRepository,
		sensitiveWordRepository:        sensitiveWordRepository,
		productImportMappingRepository: productImportMappingRepository,
		categoryRepository:             categoryRepository,
		productDataRepository:          productDataRepository,
	}, nil
}

func mergeBuiltRepositories(core *builtCoreRepositories, lateCore *builtLateCoreRepositories, admin *builtAdminRepositories) *builtRepositories {
	repos := &builtRepositories{}
	if core != nil {
		repos.taskRepository = core.taskRepository
		repos.studioAsyncJobRepository = core.studioAsyncJobRepository
	}
	if lateCore != nil {
		repos.subscriptionService = lateCore.subscriptionService
		repos.assetRepository = lateCore.assetRepository
		repos.reviewRepository = lateCore.reviewRepository
		repos.studioSessionRepository = lateCore.studioSessionRepository
		repos.uploadedImageRepository = lateCore.uploadedImageRepository
		repos.storeProfileRepository = lateCore.storeProfileRepository
		repos.storeRoutingSettingsRepository = lateCore.storeRoutingSettingsRepository
		repos.resolutionCacheStore = lateCore.resolutionCacheStore
	}
	if admin != nil {
		repos.storeRepository = admin.storeRepository
		repos.storeStatisticsRepository = admin.storeStatisticsRepository
		repos.importTaskRepository = admin.importTaskRepository
		repos.filterRuleRepository = admin.filterRuleRepository
		repos.profitRuleRepository = admin.profitRuleRepository
		repos.pricingRuleRepository = admin.pricingRuleRepository
		repos.operationStrategyRepository = admin.operationStrategyRepository
		repos.sensitiveWordRepository = admin.sensitiveWordRepository
		repos.productImportMappingRepository = admin.productImportMappingRepository
		repos.categoryRepository = admin.categoryRepository
		repos.productDataRepository = admin.productDataRepository
	}
	return repos
}

func buildRepositories(input BuildServiceInput, closers *closerStack) (*builtRepositories, error) {
	coreRepos, err := buildCoreRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	adminRepos, err := buildAdminRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	lateCoreRepos, err := buildLateCoreRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	return mergeBuiltRepositories(coreRepos, lateCoreRepos, adminRepos), nil
}

func buildModuleService(input BuildServiceInput, repos *builtRepositories, submit submitModule, closers *closerStack) (moduleService, error) {
	hooks := input.Hooks

	listingkit.ConfigureSheinSubmitDebugDumpDir(input.Config.ListingKit.SheinSubmitDebugDumpDir)
	listingkit.ConfigureOwnerScopeRequired(input.Config.ListingKit.OwnerScopeRequired)
	listingadmin.ConfigureOwnerScopeRequired(input.Config.ListingKit.OwnerScopeRequired)
	hooks.ConfigureZitadelAuth(input.Config.ListingKit.Zitadel)
	if err := hooks.ConfigureAuthorization(input.Config.ListingKit.PlatformAdminUsers, input.Config.ListingKit.PlatformAdminRoles); err != nil {
		return nil, fmt.Errorf("configure listing kit authorization: %w", err)
	}
	if legacyTenantResolverCloser, err := hooks.LegacyTenantResolverConfigurator(input.Config, input.Logger); err != nil {
		return nil, fmt.Errorf("configure listing kit legacy tenant resolver: %w", err)
	} else if legacyTenantResolverCloser != nil {
		closers.Add(legacyTenantResolverCloser)
	}

	serviceConfig := buildListingKitServiceConfig(buildListingKitServiceConfigInput{
		input:        input,
		repositories: repos,
		submit:       submit,
	})

	svc, err := listingkit.NewService(serviceConfig)
	if err != nil {
		return nil, fmt.Errorf("create listing kit service: %w", err)
	}

	moduleSvc, ok := svc.(moduleService)
	if !ok {
		return nil, fmt.Errorf("listing kit service does not implement module service contract")
	}

	return wireTemporalWorkflowClients(moduleSvc, input.Logger, closers)
}

func buildListingKitServiceConfig(in buildListingKitServiceConfigInput) *listingkit.ServiceConfig {
	return &listingkit.ServiceConfig{
		Core: listingkit.ServiceCoreDependencies{
			Repository:                     in.repositories.taskRepository,
			StudioSessionRepository:        in.repositories.studioSessionRepository,
			ProductService:                 in.input.ProductService,
			ImageService:                   in.input.ImageService,
			SDSSyncService:                 in.input.SDSSyncService,
			ImageUploadStore:               in.submit.assets.imageUploadStore,
			UploadedImageRepository:        in.repositories.uploadedImageRepository,
			StoreProfileRepository:         in.repositories.storeProfileRepository,
			StoreRoutingSettingsRepository: in.repositories.storeRoutingSettingsRepository,
			AIClientCredentialStore:        in.input.AICredentialStore,
		},
		Assets: listingkit.ServiceAssetDependencies{
			Assembler:           in.submit.assets.assembler,
			AssetRepository:     in.repositories.assetRepository,
			ReviewRepository:    in.repositories.reviewRepository,
			AssetRecipeResolver: assetrecipe.NewStaticResolver(),
			AssetBundleBuilder:  assetbundle.NewBuilder(),
			AssetGenerationService: assetgeneration.NewService(assetgeneration.Config{
				SubjectExtractor:        in.input.ImageSubjectExtractor,
				WhiteBackgroundRenderer: in.input.ImageWhiteBackgroundRender,
				DeferredRenderer:        assetgeneration.NewProductImageDeferredRenderer(in.input.ImageSceneRenderer),
			}),
		},
		Shein: listingkit.ServiceSheinDependencies{
			SheinDefaultStoreID:        in.submit.shein.defaultStoreID,
			SheinStoreCatalog:          sheinManagementStoreCatalog{repo: in.repositories.storeRepository},
			SheinAPIClientFactory:      in.submit.shein.apiClientFactory,
			SheinCategoryResolver:      in.submit.shein.categoryResolver,
			SheinResolutionCacheStore:  in.repositories.resolutionCacheStore,
			SheinAttributeResolver:     in.submit.shein.attributeResolver,
			SheinSaleAttributeResolver: in.submit.shein.saleAttributeResolver,
			SheinPricingPolicy:         in.submit.shein.pricingPolicy,
			SheinProductAPIBuilder:     in.submit.shein.productAPIBuilder,
			SheinImageAPIBuilder:       in.submit.shein.imageAPIBuilder,
			SheinTranslateAPIBuilder:   in.submit.shein.translateAPIBuilder,
			SheinContentOptimizer:      in.submit.shein.contentOptimizer,
			StudioPromptDiversifier:    in.submit.shein.contentOptimizer,
			StudioImageGenerator:       in.submit.studio.imageGenerator,
		},
		Workflow: listingkit.ServiceWorkflowDependencies{
			SheinPublishWorkflowClient:     nil,
			SheinPublishWorkflowEnabled:    false,
			StandardProductWorkflowClient:  nil,
			StandardProductWorkflowEnabled: false,
			PlatformAdaptWorkflowClient:    nil,
			PlatformAdaptWorkflowEnabled:   false,
		},
	}
}

func wireTemporalWorkflowClients(svc moduleService, logger *logrus.Logger, closers *closerStack) (moduleService, error) {
	temporalWorkflowClient, temporalCloser, err := appruntime.DialListingKitSheinPublishTemporalClient(logger)
	if err != nil {
		return nil, fmt.Errorf("connect listing kit shein publish temporal client: %w", err)
	}
	if temporalWorkflowClient != nil {
		if err := listingkit.ConfigureSheinPublishWorkflowClient(svc, temporalWorkflowClient, true); err != nil {
			if temporalCloser != nil {
				_ = temporalCloser()
			}
			return nil, err
		}
		if standardClient, ok := temporalWorkflowClient.(listingkit.StandardProductWorkflowClient); ok {
			if err := listingkit.ConfigureStandardProductWorkflowClient(svc, standardClient, true); err != nil {
				if temporalCloser != nil {
					_ = temporalCloser()
				}
				return nil, err
			}
		}
		if platformClient, ok := temporalWorkflowClient.(listingkit.PlatformAdaptWorkflowClient); ok {
			if err := listingkit.ConfigurePlatformAdaptWorkflowClient(svc, platformClient, true); err != nil {
				if temporalCloser != nil {
					_ = temporalCloser()
				}
				return nil, err
			}
		}
	}
	if temporalCloser != nil {
		closers.Add(temporalCloser)
	}
	return svc, nil
}

func buildModuleRuntime(input BuildModuleInput, bundle *ServiceBundle) (_ *Module, err error) {
	closers := &closerStack{}
	closers.Add(bundle.runtime.closers...)
	defer func() {
		if err == nil {
			return
		}
		_ = closers.Close()
	}()
	if input.ShouldStartTemporalWorkerInProcess {
		temporalWorkerCloser, startErr := appruntime.StartListingKitSheinPublishTemporalWorker(bundle.runtime.temporalWorkerService, input.ServiceInput.Logger)
		if startErr != nil {
			return nil, fmt.Errorf("start listing kit shein publish temporal worker: %w", startErr)
		}
		closers.Add(temporalWorkerCloser)
	}

	processor, err := listingkit.NewProcessor(bundle.runtime.service, bundle.runtime.taskRepository, input.ServiceInput.Logger, 2)
	if err != nil {
		return nil, fmt.Errorf("create listing kit processor: %w", err)
	}
	pool := httpbootstrap.NewWorkerPool(processor, input.ServiceInput.Config)
	submitter := &httpbootstrap.PoolSubmitter{Pool: pool}
	bundle.runtime.service.SetTaskSubmitter(submitter)
	processor.SetTaskSubmitter(submitter)

	handler, err := listingkitapi.NewHandler(bundle.runtime.service, buildHandlerOptions(bundle.runtime.handlerDependencies)...)
	if err != nil {
		return nil, fmt.Errorf("create listing kit handler: %w", err)
	}

	studioSessionHandler, err := listingkitapi.NewStudioSessionHandler(bundle.runtime.service)
	if err != nil {
		return nil, fmt.Errorf("create listing kit studio session handler: %w", err)
	}

	return &Module{
		Handler:              handler,
		StudioSessionHandler: studioSessionHandler,
		Pool:                 pool,
		Closers:              closers.Snapshot(),
	}, nil
}

func assembleServiceBundle(repositories *builtRepositories, moduleSvc moduleService, workerService TemporalWorkerService, handlerDependencies listingkitapi.HandlerDependencies, closers []func() error) *ServiceBundle {
	return &ServiceBundle{
		TemporalWorkerService:          workerService,
		TaskRepository:                 repositories.taskRepository,
		StudioAsyncJobRepository:       repositories.studioAsyncJobRepository,
		StoreRepository:                repositories.storeRepository,
		StoreStatisticsRepository:      repositories.storeStatisticsRepository,
		ImportTaskRepository:           repositories.importTaskRepository,
		FilterRuleRepository:           repositories.filterRuleRepository,
		ProfitRuleRepository:           repositories.profitRuleRepository,
		PricingRuleRepository:          repositories.pricingRuleRepository,
		OperationStrategyRepository:    repositories.operationStrategyRepository,
		SensitiveWordRepository:        repositories.sensitiveWordRepository,
		ProductImportMappingRepository: repositories.productImportMappingRepository,
		CategoryRepository:             repositories.categoryRepository,
		ProductDataRepository:          repositories.productDataRepository,
		SubscriptionService:            repositories.subscriptionService,
		Closers:                        closers,
		runtime: serviceBundleRuntime{
			temporalWorkerService: workerService,
			taskRepository:        repositories.taskRepository,
			service:               moduleSvc,
			handlerDependencies:   handlerDependencies,
			closers:               closers,
		},
	}
}

func buildHandlerOptions(handlerDependencies listingkitapi.HandlerDependencies) []listingkitapi.HandlerOption {
	return []listingkitapi.HandlerOption{
		listingkitapi.WithDependencies(handlerDependencies),
	}
}

func BuildModule(input BuildModuleInput) (_ *Module, err error) {
	bundle, err := BuildService(input.ServiceInput)
	if err != nil {
		return nil, err
	}
	return buildModuleRuntime(input, bundle)
}

func BuildService(input BuildServiceInput) (_ *ServiceBundle, err error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	closers := &closerStack{}
	defer func() {
		if err == nil {
			return
		}
		_ = closers.Close()
	}()
	repositories, err := buildRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	task := buildTaskModule(newTaskModuleInput(input, repositories))
	admin := buildAdminModule(newAdminModuleInput(repositories))
	submit := buildSubmitModule(newSubmitModuleInput(input, repositories))
	moduleSvc, err := buildModuleService(input, repositories, submit, closers)
	if err != nil {
		return nil, err
	}
	temporal := buildTemporalModule(temporalModuleInput{Service: moduleSvc})
	handlerDependencies := task.handlerDependenciesWithAdmin(admin)

	return assembleServiceBundle(repositories, moduleSvc, temporal.workerService, handlerDependencies, closers.Snapshot()), nil
}
