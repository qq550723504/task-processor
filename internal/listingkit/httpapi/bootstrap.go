package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
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
	TemporalWorkerService           TemporalWorkerService
	TaskRepository                  listingkit.Repository
	StudioAsyncJobRepository        listingkit.StudioAsyncJobRepository
	StoreRepository                 listingadmin.StoreRepository
	StoreStatisticsRepository       listingadmin.StoreStatisticsRepository
	ImportTaskRepository            listingadmin.ImportTaskRepository
	FilterRuleRepository            listingadmin.FilterRuleRepository
	ProfitRuleRepository            listingadmin.ProfitRuleRepository
	PricingRuleRepository           listingadmin.PricingRuleRepository
	OperationStrategyRepository     listingadmin.OperationStrategyRepository
	SensitiveWordRepository         listingadmin.SensitiveWordRepository
	GenerationTopicPolicyRepository listingadmin.GenerationTopicPolicyRepository
	ProductImportMappingRepository  listingadmin.ProductImportMappingRepository
	CategoryRepository              listingadmin.CategoryRepository
	ProductDataRepository           listingadmin.ProductDataRepository
	SubscriptionService             *listingsubscription.Service
	Closers                         []func() error

	runtime serviceBundleRuntime
}

type serviceBundleRuntime struct {
	temporalWorkerService  TemporalWorkerService
	taskRepository         listingkit.Repository
	service                moduleService
	sheinSyncRepository    listingkit.SheinSyncRepository
	sheinSyncService       listingkit.SheinSyncService
	sheinCandidateService  listingkit.SheinCandidateService
	sheinEnrollmentService listingkit.SheinEnrollmentService
	handlerDependencies    listingkitapi.HandlerDependencies
	closers                []func() error
}

type TemporalWorkerService interface {
	listingkit.SheinPublishActivityHostSource
	listingkit.LayerWorkflowActivityHostSource
}

type moduleService interface {
	listingkit.TaskLifecycleService
	listingkit.GenerationTaskService
	listingkit.StudioMediaService
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
	Store                   func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error)
	StoreStatistics         func(*config.Config, *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error)
	ImportTask              func(*config.Config, *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error)
	FilterRule              func(*config.Config, *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error)
	ProfitRule              func(*config.Config, *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error)
	PricingRule             func(*config.Config, *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error)
	OperationStrategy       func(*config.Config, *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error)
	SensitiveWord           func(*config.Config, *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error)
	GenerationTopicOverride func(*config.Config, *logrus.Logger) (listingadmin.GenerationTopicOverrideRepository, []func() error, error)
	GenerationTopicPolicy   func(*config.Config, *logrus.Logger) (listingadmin.GenerationTopicPolicyRepository, []func() error, error)
	ProductImportMapping    func(*config.Config, *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error)
	Category                func(*config.Config, *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error)
	ProductData             func(*config.Config, *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error)
}

type CoreRepositoryBuilders struct {
	Task                 func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error)
	StudioAsyncJob       func(*config.Config, *logrus.Logger) (listingkit.StudioAsyncJobRepository, []func() error, error)
	StudioBatch          func(*config.Config, *logrus.Logger) (listingkit.StudioBatchRepository, []func() error, error)
	StudioBatchRun       func(*config.Config, *logrus.Logger) (listingkit.StudioBatchRunRepository, []func() error, error)
	SheinSync            func(*config.Config, *logrus.Logger) (listingkit.SheinSyncRepository, []func() error, error)
	Subscription         func(*config.Config, *logrus.Logger) (listingsubscription.Repository, []func() error, error)
	Asset                func(*config.Config, *logrus.Logger) (assetrepo.Repository, []func() error, error)
	Review               func(*config.Config, *logrus.Logger) (reviewstore.Repository, []func() error, error)
	StudioSession        func(*config.Config, *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error)
	UploadedImage        func(*config.Config, *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error)
	StoreProfile         func(*config.Config, *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error)
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
	case b.StudioBatch == nil:
		return fmt.Errorf("core repository builder studio batch is required")
	case b.StudioBatchRun == nil:
		return fmt.Errorf("core repository builder studio batch run is required")
	case b.SheinSync == nil:
		return fmt.Errorf("core repository builder shein sync is required")
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
	case b.GenerationTopicOverride == nil:
		return fmt.Errorf("admin repository builder generation topic override is required")
	case b.GenerationTopicPolicy == nil:
		return fmt.Errorf("admin repository builder generation topic policy is required")
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
	SDSLoginStatusProvider     listingkit.SDSLoginStatusProvider
	SDSBaselineRemoteProvider  listingkit.SDSBaselineRemoteProvider
	ImageSubjectExtractor      productimage.SubjectExtractor
	ImageWhiteBackgroundRender productimage.WhiteBackgroundRenderer
	ImageSceneRenderer         productimage.SceneRenderer
	AICredentialStore          aiCredentialStore
	Repositories               BuildServiceRepositories
	Hooks                      BuildServiceHooks
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

func buildNamedWithClosers[T any](name string, builder func(*config.Config, *logrus.Logger) (T, []func() error, error), cfg *config.Config, logger *logrus.Logger, closers *closerStack) (T, error) {
	startedAt := time.Now()
	if logger != nil {
		logger.WithField("component", "listingkit/httpapi").WithField("repository", name).Info("listingkit repository build begin")
	}
	value, err := buildWithClosers(builder, cfg, logger, closers)
	if err != nil {
		if logger != nil {
			logger.WithError(err).WithField("component", "listingkit/httpapi").WithField("repository", name).Warn("listingkit repository build failed")
		}
		var zero T
		return zero, err
	}
	if logger != nil {
		logger.WithField("component", "listingkit/httpapi").WithField("repository", name).WithField("elapsed", time.Since(startedAt)).Info("listingkit repository build done")
	}
	return value, nil
}

func prepareModuleServiceEnvironment(input BuildServiceInput, closers *closerStack) error {
	configureModuleServicePolicies(input)
	return configureModuleServiceAuthorization(input, closers)
}

func configureModuleServicePolicies(input BuildServiceInput) {
	listingkit.ConfigureSheinSubmitDebugDumpDir(input.Config.ListingKit.SheinSubmitDebugDumpDir)
	listingkit.ConfigureOwnerScopeRequired(true)
	listingadmin.ConfigureOwnerScopeRequired(true)
	input.Hooks.ConfigureZitadelAuth(input.Config.ListingKit.Zitadel)
}

func configureModuleServiceAuthorization(input BuildServiceInput, closers *closerStack) error {
	hooks := input.Hooks
	if err := hooks.ConfigureAuthorization(input.Config.ListingKit.PlatformAdminUsers, input.Config.ListingKit.PlatformAdminRoles); err != nil {
		return fmt.Errorf("configure listing kit authorization: %w", err)
	}
	legacyTenantResolverCloser, err := hooks.LegacyTenantResolverConfigurator(input.Config, input.Logger)
	if err != nil {
		return fmt.Errorf("configure listing kit legacy tenant resolver: %w", err)
	}
	if legacyTenantResolverCloser != nil {
		closers.Add(legacyTenantResolverCloser)
	}
	return nil
}

func createModuleService(input BuildServiceInput, repos *builtRepositories, submit submitModule) (moduleService, error) {
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
	return moduleSvc, nil
}

func buildModuleService(input BuildServiceInput, repos *builtRepositories, submit submitModule, closers *closerStack) (moduleService, error) {
	if err := prepareModuleServiceEnvironment(input, closers); err != nil {
		return nil, err
	}

	moduleSvc, err := createModuleService(input, repos, submit)
	if err != nil {
		return nil, err
	}

	return wireTemporalWorkflowClients(moduleSvc, input.Logger, closers)
}

func wireTemporalWorkflowClients(svc moduleService, logger *logrus.Logger, closers *closerStack) (moduleService, error) {
	temporalWorkflowClient, temporalCloser, err := appruntime.DialListingKitSheinPublishTemporalClient(logger)
	if err != nil {
		return nil, fmt.Errorf("connect listing kit shein publish temporal client: %w", err)
	}
	if temporalWorkflowClient != nil {
		if err := listingkit.ConfigureSheinPublishWorkflowClient(svc, temporalWorkflowClient, true); err != nil {
			return nil, closeTemporalWorkflowClientOnError(err, temporalCloser)
		}
		if standardClient, ok := temporalWorkflowClient.(listingkit.StandardProductWorkflowClient); ok {
			if err := listingkit.ConfigureStandardProductWorkflowClient(svc, standardClient, true); err != nil {
				return nil, closeTemporalWorkflowClientOnError(err, temporalCloser)
			}
		}
		if platformClient, ok := temporalWorkflowClient.(listingkit.PlatformAdaptWorkflowClient); ok {
			if err := listingkit.ConfigurePlatformAdaptWorkflowClient(svc, platformClient, true); err != nil {
				return nil, closeTemporalWorkflowClientOnError(err, temporalCloser)
			}
		}
	}
	if temporalCloser != nil {
		closers.Add(temporalCloser)
	}
	return svc, nil
}

func closeTemporalWorkflowClientOnError(err error, temporalCloser func() error) error {
	if err != nil && temporalCloser != nil {
		_ = temporalCloser()
	}
	return err
}

func assembleServiceBundle(repositories *builtRepositories, moduleSvc moduleService, runtimeServices sheinSyncRuntimeServices, workerService TemporalWorkerService, handlerDependencies listingkitapi.HandlerDependencies, closers []func() error) *ServiceBundle {
	return &ServiceBundle{
		TemporalWorkerService:           workerService,
		TaskRepository:                  repositories.taskRepository,
		StudioAsyncJobRepository:        repositories.studioAsyncJobRepository,
		StoreRepository:                 repositories.storeRepository,
		StoreStatisticsRepository:       repositories.storeStatisticsRepository,
		ImportTaskRepository:            repositories.importTaskRepository,
		FilterRuleRepository:            repositories.filterRuleRepository,
		ProfitRuleRepository:            repositories.profitRuleRepository,
		PricingRuleRepository:           repositories.pricingRuleRepository,
		OperationStrategyRepository:     repositories.operationStrategyRepository,
		SensitiveWordRepository:         repositories.sensitiveWordRepository,
		GenerationTopicPolicyRepository: repositories.generationTopicPolicyRepository,
		ProductImportMappingRepository:  repositories.productImportMappingRepository,
		CategoryRepository:              repositories.categoryRepository,
		ProductDataRepository:           repositories.productDataRepository,
		SubscriptionService:             repositories.subscriptionService,
		Closers:                         closers,
		runtime: serviceBundleRuntime{
			temporalWorkerService:  workerService,
			taskRepository:         repositories.taskRepository,
			service:                moduleSvc,
			sheinSyncRepository:    repositories.sheinSyncRepository,
			sheinSyncService:       runtimeServices.syncService,
			sheinCandidateService:  runtimeServices.candidateService,
			sheinEnrollmentService: runtimeServices.enrollmentService,
			handlerDependencies:    handlerDependencies,
			closers:                closers,
		},
	}
}

func buildHandlerOptions(runtime serviceBundleRuntime) []listingkitapi.HandlerOption {
	return []listingkitapi.HandlerOption{
		listingkitapi.WithTaskLifecycleService(runtime.service),
		listingkitapi.WithGenerationTaskService(runtime.service),
		listingkitapi.WithStudioMediaService(runtime.service),
		listingkitapi.WithDependencies(runtime.handlerDependencies),
		listingkitapi.WithSheinSyncRepository(runtime.sheinSyncRepository),
		listingkitapi.WithSheinSyncServices(runtime.sheinSyncService, runtime.sheinCandidateService, runtime.sheinEnrollmentService),
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
	if input.Logger != nil {
		input.Logger.WithField("component", "listingkit/httpapi").Info("listingkit repositories ready")
	}
	return buildServiceRuntime(input, repositories, closers)
}
