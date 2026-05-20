package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	"task-processor/internal/httpbootstrap"
	"task-processor/internal/infra/clients/management"
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
	Handler              listingkit.Handler
	StudioSessionHandler listingkit.StudioSessionHandler
	Service              listingkit.Service
	Pool                 worker.WorkerPool
	Closers              []func() error
}

type ServiceBundle struct {
	Service                        listingkit.Service
	TaskRepository                 listingkit.Repository
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
}

type BuildModuleInput struct {
	ServiceInput                       BuildServiceInput
	ShouldStartTemporalWorkerInProcess bool
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
	ManagementClient           *management.ClientManager
	AICredentialStore          interface {
		listingkit.AIClientCredentialStore
		openaiclient.ClientConfigResolver
	}

	TaskRepositoryBuilder                 func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error)
	StoreRepositoryBuilder                func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error)
	StoreStatisticsRepositoryBuilder      func(*config.Config, *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error)
	ImportTaskRepositoryBuilder           func(*config.Config, *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error)
	FilterRuleRepositoryBuilder           func(*config.Config, *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error)
	ProfitRuleRepositoryBuilder           func(*config.Config, *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error)
	PricingRuleRepositoryBuilder          func(*config.Config, *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error)
	OperationStrategyRepositoryBuilder    func(*config.Config, *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error)
	SensitiveWordRepositoryBuilder        func(*config.Config, *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error)
	ProductImportMappingRepositoryBuilder func(*config.Config, *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error)
	CategoryRepositoryBuilder             func(*config.Config, *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error)
	ProductDataRepositoryBuilder          func(*config.Config, *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error)
	SubscriptionRepositoryBuilder         func(*config.Config, *logrus.Logger) (listingsubscription.Repository, []func() error, error)
	AssetRepositoryBuilder                func(*config.Config, *logrus.Logger) (assetrepo.Repository, []func() error, error)
	ReviewRepositoryBuilder               func(*config.Config, *logrus.Logger) (reviewstore.Repository, []func() error, error)
	StudioSessionRepositoryBuilder        func(*config.Config, *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error)
	UploadedImageRepositoryBuilder        func(*config.Config, *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error)
	StoreProfileRepositoryBuilder         func(*config.Config, *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error)
	StoreRoutingSettingsRepositoryBuilder func(*config.Config, *logrus.Logger) (listingkit.StoreRoutingSettingsRepository, []func() error, error)
	ResolutionCacheStoreBuilder           func(*config.Config, *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error)

	SheinPricingPolicyBuilder        func(*config.Config) sheinpub.PricingPolicy
	ImageUploadStoreBuilder          func(*config.Config, *logrus.Logger) listingkit.ImageUploadStore
	LegacyTenantResolverConfigurator func(*config.Config, *logrus.Logger) (func() error, error)
	SheinCategoryLLMClientBuilder    func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter
	SheinSaleAttributeLLMBuilder     func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter
	StudioImageGeneratorBuilder      func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ImageGenerator
	DefaultSheinStoreIDResolver      func([]int64) int64
	ConfigureZitadelAuth             func(config.ListingKitZitadelConfig)
	ConfigureAuthorization           func([]string, []string) error
}

func BuildModule(input BuildModuleInput) (*Module, error) {
	bundle, err := BuildService(input.ServiceInput)
	if err != nil {
		return nil, err
	}

	var closers []func() error
	closers = append(closers, bundle.Closers...)

	var temporalWorkerCloser func() error
	defer func() {
		if err == nil || temporalWorkerCloser == nil {
			return
		}
		_ = temporalWorkerCloser()
	}()
	if input.ShouldStartTemporalWorkerInProcess {
		temporalWorkerCloser, err = appruntime.StartListingKitSheinPublishTemporalWorker(bundle.Service, input.ServiceInput.Logger)
		if err != nil {
			return nil, fmt.Errorf("start listing kit shein publish temporal worker: %w", err)
		}
	}

	processor, err := listingkit.NewProcessor(bundle.Service, bundle.TaskRepository, input.ServiceInput.Logger, 2)
	if err != nil {
		return nil, fmt.Errorf("create listing kit processor: %w", err)
	}
	pool := httpbootstrap.NewWorkerPool(processor, input.ServiceInput.Config)
	submitter := &httpbootstrap.PoolSubmitter{Pool: pool}
	bundle.Service.SetTaskSubmitter(submitter)
	processor.SetTaskSubmitter(submitter)

	handler, err := listingkitapi.NewHandler(
		bundle.Service,
		listingkitapi.WithStudioAsyncJobStorePath(input.ServiceInput.Config.ListingKit.StudioAsyncJobStorePath),
		listingkitapi.WithPlatformSubscriptionAccess(input.ServiceInput.Config.ListingKit.PlatformAdminUsers, input.ServiceInput.Config.ListingKit.PlatformAdminRoles),
		listingkitapi.WithStoreRepository(bundle.StoreRepository),
		listingkitapi.WithStoreStatisticsRepository(bundle.StoreStatisticsRepository),
		listingkitapi.WithImportTaskRepository(bundle.ImportTaskRepository),
		listingkitapi.WithFilterRuleRepository(bundle.FilterRuleRepository),
		listingkitapi.WithProfitRuleRepository(bundle.ProfitRuleRepository),
		listingkitapi.WithPricingRuleRepository(bundle.PricingRuleRepository),
		listingkitapi.WithOperationStrategyRepository(bundle.OperationStrategyRepository),
		listingkitapi.WithSensitiveWordRepository(bundle.SensitiveWordRepository),
		listingkitapi.WithProductImportMappingRepository(bundle.ProductImportMappingRepository),
		listingkitapi.WithCategoryRepository(bundle.CategoryRepository),
		listingkitapi.WithProductDataRepository(bundle.ProductDataRepository),
		listingkitapi.WithSubscriptionService(bundle.SubscriptionService),
	)
	if err != nil {
		return nil, fmt.Errorf("create listing kit handler: %w", err)
	}

	studioSessionService, ok := bundle.Service.(listingkit.StudioSessionHandlerService)
	if !ok {
		return nil, fmt.Errorf("listing kit service does not implement studio session handler service")
	}
	studioSessionHandler, err := listingkitapi.NewStudioSessionHandler(studioSessionService)
	if err != nil {
		return nil, fmt.Errorf("create listing kit studio session handler: %w", err)
	}

	if temporalWorkerCloser != nil {
		closers = append(closers, temporalWorkerCloser)
	}
	return &Module{
		Handler:              handler,
		StudioSessionHandler: studioSessionHandler,
		Service:              bundle.Service,
		Pool:                 pool,
		Closers:              closers,
	}, nil
}

func BuildService(input BuildServiceInput) (*ServiceBundle, error) {
	repo, repoClosers, err := input.TaskRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers := append([]func() error{}, repoClosers...)

	storeRepo, repoClosers, err := input.StoreRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	storeStatisticsRepo, repoClosers, err := input.StoreStatisticsRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	importTaskRepo, repoClosers, err := input.ImportTaskRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	filterRuleRepo, repoClosers, err := input.FilterRuleRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	profitRuleRepo, repoClosers, err := input.ProfitRuleRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	pricingRuleRepo, repoClosers, err := input.PricingRuleRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	operationStrategyRepo, repoClosers, err := input.OperationStrategyRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	sensitiveWordRepo, repoClosers, err := input.SensitiveWordRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	productImportMappingRepo, repoClosers, err := input.ProductImportMappingRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	categoryRepo, repoClosers, err := input.CategoryRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	productDataRepo, repoClosers, err := input.ProductDataRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	subscriptionRepo, repoClosers, err := input.SubscriptionRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)
	subscriptionService, err := listingsubscription.NewService(subscriptionRepo)
	if err != nil {
		return nil, fmt.Errorf("create listing subscription service: %w", err)
	}

	assetRepository, repoClosers, err := input.AssetRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	reviewRepository, repoClosers, err := input.ReviewRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	studioSessionRepository, repoClosers, err := input.StudioSessionRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	uploadedImageRepository, repoClosers, err := input.UploadedImageRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	storeProfileRepository, repoClosers, err := input.StoreProfileRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	storeRoutingSettingsRepository, repoClosers, err := input.StoreRoutingSettingsRepositoryBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	resolutionCacheStore, repoClosers, err := input.ResolutionCacheStoreBuilder(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, repoClosers...)

	sheinCategoryLLMClient := input.SheinCategoryLLMClientBuilder(input.Config, input.AICredentialStore)
	sheinSaleAttributeLLMClient := input.SheinSaleAttributeLLMBuilder(input.Config, input.AICredentialStore)
	sheinCategoryResolver := sheinpub.NewCachedCategoryResolver(sheinpub.NewManagedCategoryResolver(input.ManagementClient, sheinCategoryLLMClient), resolutionCacheStore)
	sheinAttributeResolver := sheinpub.NewCachedAttributeResolver(sheinpub.NewManagedAttributeResolver(input.ManagementClient, sheinSaleAttributeLLMClient), resolutionCacheStore)
	sheinSaleAttributeResolver := sheinpub.NewCachedSaleAttributeResolver(sheinpub.NewManagedSaleAttributeResolver(input.ManagementClient, sheinSaleAttributeLLMClient), resolutionCacheStore)
	sheinProductAPIBuilder := sheinpub.NewManagedProductAPIBuilder(input.ManagementClient)
	sheinImageAPIBuilder := sheinpub.NewManagedImageAPIBuilder(input.ManagementClient)
	sheinTranslateAPIBuilder := sheinpub.NewManagedTranslateAPIBuilder(input.ManagementClient)
	sheinPricingPolicy := input.SheinPricingPolicyBuilder(input.Config)

	listingkit.ConfigureSheinSubmitDebugDumpDir(input.Config.ListingKit.SheinSubmitDebugDumpDir)
	listingkit.ConfigureOwnerScopeRequired(input.Config.ListingKit.OwnerScopeRequired)
	listingadmin.ConfigureOwnerScopeRequired(input.Config.ListingKit.OwnerScopeRequired)
	input.ConfigureZitadelAuth(input.Config.ListingKit.Zitadel)
	if err := input.ConfigureAuthorization(input.Config.ListingKit.PlatformAdminUsers, input.Config.ListingKit.PlatformAdminRoles); err != nil {
		return nil, fmt.Errorf("configure listing kit authorization: %w", err)
	}
	if legacyTenantResolverCloser, err := input.LegacyTenantResolverConfigurator(input.Config, input.Logger); err != nil {
		return nil, fmt.Errorf("configure listing kit legacy tenant resolver: %w", err)
	} else if legacyTenantResolverCloser != nil {
		closers = append(closers, legacyTenantResolverCloser)
	}

	svc, err := listingkit.NewService(&listingkit.ServiceConfig{
		Repository:                     repo,
		StudioSessionRepository:        studioSessionRepository,
		UploadedImageRepository:        uploadedImageRepository,
		StoreProfileRepository:         storeProfileRepository,
		StoreRoutingSettingsRepository: storeRoutingSettingsRepository,
		ProductService:                 input.ProductService,
		ImageService:                   input.ImageService,
		SDSSyncService:                 input.SDSSyncService,
		SheinDefaultStoreID:            input.DefaultSheinStoreIDResolver(input.Config.Management.StoreIDs),
		ImageUploadStore:               input.ImageUploadStoreBuilder(input.Config, input.Logger),
		AssetRepository:                assetRepository,
		ReviewRepository:               reviewRepository,
		AssetRecipeResolver:            assetrecipe.NewStaticResolver(),
		AssetBundleBuilder:             assetbundle.NewBuilder(),
		AssetGenerationService: assetgeneration.NewService(assetgeneration.Config{
			SubjectExtractor:        input.ImageSubjectExtractor,
			WhiteBackgroundRenderer: input.ImageWhiteBackgroundRender,
			DeferredRenderer:        assetgeneration.NewProductImageDeferredRenderer(input.ImageSceneRenderer),
		}),
		SheinManagementClient:      input.ManagementClient,
		SheinCategoryResolver:      sheinCategoryResolver,
		SheinAttributeResolver:     sheinAttributeResolver,
		SheinSaleAttributeResolver: sheinSaleAttributeResolver,
		SheinPricingPolicy:         sheinPricingPolicy,
		SheinProductAPIBuilder:     sheinProductAPIBuilder,
		SheinImageAPIBuilder:       sheinImageAPIBuilder,
		SheinTranslateAPIBuilder:   sheinTranslateAPIBuilder,
		SheinContentOptimizer:      sheinCategoryLLMClient,
		StudioImageGenerator:       input.StudioImageGeneratorBuilder(input.Config, input.AICredentialStore),
		AIClientCredentialStore:    input.AICredentialStore,
		Assembler: listingkit.NewAssemblerWithConfig(listingkit.AssemblerConfig{
			SheinCategoryResolver:      sheinCategoryResolver,
			SheinAttributeResolver:     sheinAttributeResolver,
			SheinSaleAttributeResolver: sheinSaleAttributeResolver,
			SheinPricingPolicy:         sheinPricingPolicy,
			SheinTitleOptimizer:        sheinCategoryLLMClient,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("create listing kit service: %w", err)
	}

	temporalWorkflowClient, temporalCloser, err := appruntime.DialListingKitSheinPublishTemporalClient(input.Logger)
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
		closers = append(closers, temporalCloser)
	}

	return &ServiceBundle{
		Service:                        svc,
		TaskRepository:                 repo,
		StoreRepository:                storeRepo,
		StoreStatisticsRepository:      storeStatisticsRepo,
		ImportTaskRepository:           importTaskRepo,
		FilterRuleRepository:           filterRuleRepo,
		ProfitRuleRepository:           profitRuleRepo,
		PricingRuleRepository:          pricingRuleRepo,
		OperationStrategyRepository:    operationStrategyRepo,
		SensitiveWordRepository:        sensitiveWordRepo,
		ProductImportMappingRepository: productImportMappingRepo,
		CategoryRepository:             categoryRepo,
		ProductDataRepository:          productDataRepo,
		SubscriptionService:            subscriptionService,
		Closers:                        closers,
	}, nil
}
