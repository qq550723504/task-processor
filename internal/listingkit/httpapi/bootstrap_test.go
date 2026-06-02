package httpapi

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/productenrich"
	sheinpub "task-processor/internal/publishing/shein"
	sheinimageapi "task-processor/internal/shein/api/image"
	sheinproductapi "task-processor/internal/shein/api/product"
	sheintranslateapi "task-processor/internal/shein/api/translate"
	sheinclient "task-processor/internal/shein/client"
)

func buildServiceInputFixture() BuildServiceInput {
	return BuildServiceInput{
		Config: &config.Config{},
		Repositories: BuildServiceRepositories{
			Core: CoreRepositoryBuilders{
				Task: func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error) {
					return nil, nil, nil
				},
				StudioAsyncJob: func(*config.Config, *logrus.Logger) (listingkit.StudioAsyncJobRepository, []func() error, error) {
					return nil, nil, nil
				},
				StudioBatch: func(*config.Config, *logrus.Logger) (listingkit.StudioBatchRepository, []func() error, error) {
					return nil, nil, nil
				},
				StudioBatchRun: func(*config.Config, *logrus.Logger) (listingkit.StudioBatchRunRepository, []func() error, error) {
					return nil, nil, nil
				},
				Subscription: func(*config.Config, *logrus.Logger) (listingsubscription.Repository, []func() error, error) {
					return nil, nil, nil
				},
				Asset: func(*config.Config, *logrus.Logger) (assetrepo.Repository, []func() error, error) {
					return nil, nil, nil
				},
				Review: func(*config.Config, *logrus.Logger) (reviewstore.Repository, []func() error, error) {
					return nil, nil, nil
				},
				StudioSession: func(*config.Config, *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error) {
					return nil, nil, nil
				},
				UploadedImage: func(*config.Config, *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error) {
					return nil, nil, nil
				},
				StoreProfile: func(*config.Config, *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error) {
					return nil, nil, nil
				},
				StoreRoutingSettings: func(*config.Config, *logrus.Logger) (listingkit.StoreRoutingSettingsRepository, []func() error, error) {
					return nil, nil, nil
				},
				SheinResolutionCache: func(*config.Config, *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
					return nil, nil, nil
				},
			},
			Admin: AdminRepositoryBuilders{
				Store: func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
					return nil, nil, nil
				},
				StoreStatistics: func(*config.Config, *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
					return nil, nil, nil
				},
				ImportTask: func(*config.Config, *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error) {
					return nil, nil, nil
				},
				FilterRule: func(*config.Config, *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error) {
					return nil, nil, nil
				},
				ProfitRule: func(*config.Config, *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error) {
					return nil, nil, nil
				},
				PricingRule: func(*config.Config, *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error) {
					return nil, nil, nil
				},
				OperationStrategy: func(*config.Config, *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error) {
					return nil, nil, nil
				},
				SensitiveWord: func(*config.Config, *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error) {
					return nil, nil, nil
				},
				GenerationTopicPolicy: func(*config.Config, *logrus.Logger) (listingadmin.GenerationTopicPolicyRepository, []func() error, error) {
					return nil, nil, nil
				},
				ProductImportMapping: func(*config.Config, *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error) {
					return nil, nil, nil
				},
				Category: func(*config.Config, *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error) {
					return nil, nil, nil
				},
				ProductData: func(*config.Config, *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error) {
					return nil, nil, nil
				},
			},
		},
		Hooks: BuildServiceHooks{
			SheinPricingPolicyBuilder: func(*config.Config) sheinpub.PricingPolicy { return sheinpub.PricingPolicy{} },
			ImageUploadStoreBuilder:   func(*config.Config, *logrus.Logger) listingkit.ImageUploadStore { return nil },
			LegacyTenantResolverConfigurator: func(*config.Config, *logrus.Logger) (func() error, error) {
				return nil, nil
			},
			SheinCategoryLLMClientBuilder: func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
				return nil
			},
			SheinSaleAttributeLLMBuilder: func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
				return nil
			},
			SheinCategoryResolverBuilder: func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver {
				return nil
			},
			SheinAttributeResolverBuilder: func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.AttributeResolver {
				return nil
			},
			SheinSaleAttributeResolverBuilder: func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.SaleAttributeResolver {
				return nil
			},
			SheinProductAPIBuilderFactory: func(listingadmin.StoreRepository) sheinpub.ProductAPIBuilder {
				return nil
			},
			SheinImageAPIBuilderFactory: func(listingadmin.StoreRepository) sheinpub.ImageAPIBuilder {
				return nil
			},
			SheinTranslateAPIBuilderFactory: func(listingadmin.StoreRepository) sheinpub.TranslateAPIBuilder {
				return nil
			},
			SheinAPIClientFactoryBuilder: func(listingadmin.StoreRepository) listingkit.SheinAPIClientFactory {
				return nil
			},
			StudioImageGeneratorBuilder: func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ImageGenerator {
				return nil
			},
			DefaultSheinStoreIDResolver: func([]int64) int64 { return 0 },
			ConfigureZitadelAuth:        func(config.ListingKitZitadelConfig) {},
			ConfigureAuthorization:      func([]string, []string) error { return nil },
		},
	}
}

func TestBootstrapFileDoesNotOwnRepositoryAssemblyHelpers(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("bootstrap.go")
	if err != nil {
		t.Fatalf("read bootstrap.go: %v", err)
	}
	content := string(src)
	for _, needle := range []string{
		"type builtRepositories struct",
		"func buildCoreRepositories(",
		"func buildLateCoreRepositories(",
		"func buildAdminRepositories(",
		"func assembleRepositories(",
		"func buildRepositories(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("bootstrap.go should not contain %q", needle)
		}
	}
}

func TestBootstrapFileDoesNotOwnServiceOrRuntimeAssemblyHelpers(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("bootstrap.go")
	if err != nil {
		t.Fatalf("read bootstrap.go: %v", err)
	}
	content := string(src)
	for _, needle := range []string{
		"type buildListingKitServiceConfigInput struct",
		"func buildListingKitServiceConfig(",
		"func buildListingKitCoreDependencies(",
		"func buildListingKitAssetDependencies(",
		"func buildListingKitSheinDependencies(",
		"func buildListingKitWorkflowDependencies(",
		"type serviceRuntimeModules struct",
		"func buildServiceRuntimeModules(",
		"func assembleServiceRuntime(",
		"func buildServiceRuntime(",
		"func assembleModuleRuntime(",
		"func createModuleRuntime(",
		"func buildModuleRuntime(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("bootstrap.go should not contain %q", needle)
		}
	}
}

func TestBootstrapFileDelegatesToExtractedAssemblers(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("bootstrap.go")
	if err != nil {
		t.Fatalf("read bootstrap.go: %v", err)
	}
	content := string(src)
	for _, needle := range []string{
		"buildListingKitServiceConfig(buildListingKitServiceConfigInput{",
		"buildModuleRuntime(input, bundle)",
		"buildRepositories(input, closers)",
		"buildServiceRuntime(input, repositories, closers)",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("bootstrap.go should delegate through %q", needle)
		}
	}
}

func TestBuildServiceClosesAcquiredResourcesWhenBuilderFails(t *testing.T) {
	t.Parallel()

	var closed []string
	input := buildServiceInputFixture()
	input.Repositories.Core.Task = func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error) {
		return nil, []func() error{
			func() error {
				closed = append(closed, "task")
				return nil
			},
		}, nil
	}
	input.Repositories.Admin.Store = func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
		return nil, nil, errors.New("store repo boom")
	}

	if _, err := BuildService(input); err == nil {
		t.Fatal("expected builder failure")
	}
	if !reflect.DeepEqual(closed, []string{"task"}) {
		t.Fatalf("closed = %v, want [task]", closed)
	}
}

func TestBuildServiceValidateRequiresGroupedBuildersAndHooks(t *testing.T) {
	t.Parallel()

	input := buildServiceInputFixture()
	input.Repositories.Core.Task = nil

	_, err := BuildService(input)
	if err == nil {
		t.Fatal("expected validation failure")
	}
	if !strings.Contains(err.Error(), "core repository builder task is required") {
		t.Fatalf("err = %v, want missing task builder error", err)
	}
}

func TestBuildServiceClosesResourcesInReverseOrderOnFailure(t *testing.T) {
	t.Parallel()

	var closed []string
	input := buildServiceInputFixture()
	input.Repositories.Core.Task = func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error) {
		return nil, []func() error{
			func() error {
				closed = append(closed, "task")
				return nil
			},
		}, nil
	}
	input.Repositories.Admin.Store = func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
		return nil, []func() error{
			func() error {
				closed = append(closed, "store")
				return nil
			},
		}, nil
	}
	input.Repositories.Admin.StoreStatistics = func(*config.Config, *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
		return nil, nil, errors.New("stats repo boom")
	}

	if _, err := BuildService(input); err == nil {
		t.Fatal("expected builder failure")
	}
	if !reflect.DeepEqual(closed, []string{"store", "task"}) {
		t.Fatalf("closed = %v, want [store task]", closed)
	}
}

func TestPrepareModuleServiceEnvironmentAddsLegacyTenantResolverCloser(t *testing.T) {
	t.Parallel()

	input := buildServiceInputFixture()
	input.Config = &config.Config{}
	closed := false
	authorized := false
	input.Hooks.ConfigureAuthorization = func([]string, []string) error {
		authorized = true
		return nil
	}
	input.Hooks.LegacyTenantResolverConfigurator = func(*config.Config, *logrus.Logger) (func() error, error) {
		return func() error {
			closed = true
			return nil
		}, nil
	}

	closers := &closerStack{}
	if err := prepareModuleServiceEnvironment(input, closers); err != nil {
		t.Fatalf("prepareModuleServiceEnvironment: %v", err)
	}
	if !authorized {
		t.Fatal("expected authorization hook to run")
	}
	if len(closers.Snapshot()) != 1 {
		t.Fatalf("expected one closer, got %d", len(closers.Snapshot()))
	}
	if err := closers.Close(); err != nil {
		t.Fatalf("close closers: %v", err)
	}
	if !closed {
		t.Fatal("expected legacy tenant resolver closer to run")
	}
}

func TestConfigureModuleServiceAuthorizationWrapsAuthorizationError(t *testing.T) {
	t.Parallel()

	input := buildServiceInputFixture()
	input.Config = &config.Config{}
	input.Hooks.ConfigureAuthorization = func([]string, []string) error {
		return errors.New("auth boom")
	}

	err := configureModuleServiceAuthorization(input, &closerStack{})
	if err == nil {
		t.Fatal("expected authorization error")
	}
	if !strings.Contains(err.Error(), "configure listing kit authorization") {
		t.Fatalf("err = %v, want wrapped authorization error", err)
	}
}

func TestAssembleServiceBundleMapsRuntimeDependenciesAndClosers(t *testing.T) {
	t.Parallel()

	taskRepo := listingkitstore.NewMemTaskRepository()
	svc, err := listingkit.NewService(&listingkit.ServiceConfig{
		Core: listingkit.ServiceCoreDependencies{
			Repository:     taskRepo,
			ProductService: httpapiStubProductService{},
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	moduleSvc, ok := svc.(moduleService)
	if !ok {
		t.Fatalf("service type = %T, want moduleService", svc)
	}
	repos := &builtRepositories{
		taskRepository:      taskRepo,
		subscriptionService: &listingsubscription.Service{},
	}
	closers := []func() error{func() error { return nil }}

	task := buildTaskModule(taskModuleInput{
		TaskRepository:      taskRepo,
		SubscriptionService: repos.subscriptionService,
	})
	admin := buildAdminModule(adminModuleInput{})
	temporal := buildTemporalModule(temporalModuleInput{Service: moduleSvc})

	bundle := assembleServiceBundle(repos, moduleSvc, temporal.workerService, task.handlerDependenciesWithAdmin(admin), closers)
	if bundle == nil {
		t.Fatal("expected bundle")
	}
	if bundle.TaskRepository != repos.taskRepository {
		t.Fatal("expected task repository to be mapped")
	}
	if bundle.SubscriptionService != repos.subscriptionService {
		t.Fatal("expected subscription service to be mapped")
	}
	if bundle.TemporalWorkerService != moduleSvc {
		t.Fatal("expected temporal worker service to be mapped")
	}
	if bundle.runtime.handlerDependencies.Subscription.Service != repos.subscriptionService {
		t.Fatal("expected runtime handler dependencies to preserve subscription service")
	}
	if !reflect.DeepEqual(bundle.Closers, closers) {
		t.Fatalf("closers = %#v, want %#v", bundle.Closers, closers)
	}
}

func TestBuildListingKitServiceConfigMapsRegistrarOutputs(t *testing.T) {
	t.Parallel()

	taskRepo := listingkitstore.NewMemTaskRepository()
	assetRepo := assetrepo.NewMemRepository()
	reviewRepo := reviewstore.NewMemRepository()
	assembler := listingkit.NewAssemblerWithConfig(listingkit.AssemblerConfig{})
	uploadStore := &httpapiStubImageUploadStore{}
	apiFactory := &httpapiStubSheinAPIClientFactory{}

	cfg := buildListingKitServiceConfig(buildListingKitServiceConfigInput{
		input: BuildServiceInput{
			Config: &config.Config{},
		},
		repositories: &builtRepositories{
			taskRepository:        taskRepo,
			studioBatchRepository: listingkit.NewMemStudioBatchRepository(),
			assetRepository:       assetRepo,
			reviewRepository:      reviewRepo,
		},
		submit: submitModule{
			assets: submitAssetDependencies{
				assembler:        assembler,
				imageUploadStore: uploadStore,
			},
			shein: submitSheinDependencies{
				pricingPolicy:    sheinpub.PricingPolicy{},
				apiClientFactory: apiFactory,
				defaultStoreID:   903,
			},
		},
	})

	if cfg.Core.Repository != taskRepo {
		t.Fatal("expected task repository to be mapped into service config")
	}
	if cfg.Core.StudioBatchRepository == nil {
		t.Fatal("expected studio batch repository to be mapped into service config")
	}
	if cfg.Core.ImageUploadStore != uploadStore {
		t.Fatal("expected submit image upload store to be mapped into service config")
	}
	if cfg.Assets.Assembler != assembler {
		t.Fatal("expected assembler to be mapped into service config")
	}
	if cfg.Assets.AssetRepository != assetRepo {
		t.Fatal("expected asset repository to be mapped into service config")
	}
	if cfg.Assets.ReviewRepository != reviewRepo {
		t.Fatal("expected review repository to be mapped into service config")
	}
	if cfg.Shein.SheinStoreCatalog == nil {
		t.Fatal("expected shein store catalog to be set")
	}
	if cfg.Shein.SheinAPIClientFactory != apiFactory {
		t.Fatal("expected shein api client factory to be mapped from submit module")
	}
	if cfg.Shein.SheinDefaultStoreID != 903 {
		t.Fatalf("default shein store id = %d, want 903", cfg.Shein.SheinDefaultStoreID)
	}
}

func TestBuildListingKitWorkflowDependenciesDefaultsToDisabledClients(t *testing.T) {
	t.Parallel()

	workflow := buildListingKitWorkflowDependencies()
	if workflow.SheinPublishWorkflowEnabled {
		t.Fatal("expected shein publish workflow to default disabled")
	}
	if workflow.StandardProductWorkflowEnabled {
		t.Fatal("expected standard product workflow to default disabled")
	}
	if workflow.PlatformAdaptWorkflowEnabled {
		t.Fatal("expected platform adapt workflow to default disabled")
	}
	if workflow.SheinPublishWorkflowClient != nil || workflow.StandardProductWorkflowClient != nil || workflow.PlatformAdaptWorkflowClient != nil {
		t.Fatal("expected workflow clients to default nil")
	}
}

func TestNewSubmitModuleInputScopesBuildServiceDependencies(t *testing.T) {
	t.Parallel()

	input := buildServiceInputFixture()
	repos := &builtRepositories{
		storeRepository:      &listingadmin.GormStoreRepository{},
		resolutionCacheStore: &httpapiStubResolutionCacheStore{},
	}

	submitInput := newSubmitModuleInput(input, repos)
	if submitInput.Config != input.Config {
		t.Fatal("expected submit input to preserve config")
	}
	if submitInput.Logger != input.Logger {
		t.Fatal("expected submit input to preserve logger")
	}
	if submitInput.AICredentialStore != input.AICredentialStore {
		t.Fatal("expected submit input to preserve ai credential store")
	}
	if submitInput.StoreRepository != repos.storeRepository {
		t.Fatal("expected submit input to use submit-scoped store repository")
	}
	if submitInput.ResolutionCacheStore != repos.resolutionCacheStore {
		t.Fatal("expected submit input to use submit-scoped resolution cache store")
	}
	if reflect.ValueOf(submitInput.Hooks.SheinCategoryLLMClientBuilder).Pointer() != reflect.ValueOf(input.Hooks.SheinCategoryLLMClientBuilder).Pointer() {
		t.Fatal("expected submit input to preserve shein category llm builder")
	}
	if reflect.ValueOf(submitInput.Hooks.ImageUploadStoreBuilder).Pointer() != reflect.ValueOf(input.Hooks.ImageUploadStoreBuilder).Pointer() {
		t.Fatal("expected submit input to preserve image upload store builder")
	}
}

func TestMergeBuiltRepositoriesCombinesCoreAndAdminGroups(t *testing.T) {
	t.Parallel()

	taskRepo := listingkitstore.NewMemTaskRepository()
	subscriptionSvc := &listingsubscription.Service{}
	assetRepo := assetrepo.NewMemRepository()
	reviewRepo := reviewstore.NewMemRepository()
	adminStoreRepo := listingadmin.StoreRepository(nil)
	adminCategoryRepo := listingadmin.CategoryRepository(nil)

	repos := mergeBuiltRepositories(
		&builtCoreRepositories{
			taskRepository: taskRepo,
		},
		&builtLateCoreRepositories{
			subscriptionService: subscriptionSvc,
			assetRepository:     assetRepo,
			reviewRepository:    reviewRepo,
		},
		&builtAdminRepositories{
			storeRepository:    adminStoreRepo,
			categoryRepository: adminCategoryRepo,
		},
	)

	if repos.taskRepository != taskRepo {
		t.Fatal("expected core task repository to be preserved")
	}
	if repos.subscriptionService != subscriptionSvc {
		t.Fatal("expected core subscription service to be preserved")
	}
	if repos.assetRepository != assetRepo {
		t.Fatal("expected core asset repository to be preserved")
	}
	if repos.reviewRepository != reviewRepo {
		t.Fatal("expected core review repository to be preserved")
	}
	if repos.storeRepository != adminStoreRepo {
		t.Fatal("expected admin store repository to be preserved")
	}
	if repos.categoryRepository != adminCategoryRepo {
		t.Fatal("expected admin category repository to be preserved")
	}
}

func TestMergeBuiltRepositoriesPreservesPartialPhaseInputs(t *testing.T) {
	t.Parallel()

	taskRepo := listingkitstore.NewMemTaskRepository()
	repos := mergeBuiltRepositories(
		&builtCoreRepositories{taskRepository: taskRepo},
		nil,
		&builtAdminRepositories{},
	)

	if repos.taskRepository != taskRepo {
		t.Fatal("expected partial core repositories to be preserved")
	}
	if repos.subscriptionService != nil {
		t.Fatal("expected missing late core repositories to remain unset")
	}
}

func TestAssembleRepositoriesBuildsAllRepositoryPhases(t *testing.T) {
	t.Parallel()

	input := buildSuccessfulServiceInputFixture()
	closers := &closerStack{}

	assembly, err := assembleRepositories(input, closers)
	if err != nil {
		t.Fatalf("assembleRepositories: %v", err)
	}
	if assembly.core == nil {
		t.Fatal("expected core repositories to be built")
	}
	if assembly.admin == nil {
		t.Fatal("expected admin repositories to be built")
	}
	if assembly.lateCore == nil {
		t.Fatal("expected late core repositories to be built")
	}
	if assembly.merged == nil {
		t.Fatal("expected merged repositories to be built")
	}
	if assembly.merged.taskRepository != assembly.core.taskRepository {
		t.Fatal("expected merged repositories to preserve core task repository")
	}
	if assembly.merged.storeRepository != assembly.admin.storeRepository {
		t.Fatal("expected merged repositories to preserve admin store repository")
	}
	if assembly.merged.subscriptionService != assembly.lateCore.subscriptionService {
		t.Fatal("expected merged repositories to preserve late core subscription service")
	}
}

func TestBuildLateCoreRepositoriesSeparatesSubscriptionAndRepositoryDependencies(t *testing.T) {
	t.Parallel()

	input := buildSuccessfulServiceInputFixture()
	input.Repositories.Core.Asset = func(*config.Config, *logrus.Logger) (assetrepo.Repository, []func() error, error) {
		return assetrepo.NewMemRepository(), nil, nil
	}
	input.Repositories.Core.Review = func(*config.Config, *logrus.Logger) (reviewstore.Repository, []func() error, error) {
		return reviewstore.NewMemRepository(), nil, nil
	}
	input.Repositories.Core.SheinResolutionCache = func(*config.Config, *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
		return &httpapiStubResolutionCacheStore{}, nil, nil
	}
	closers := &closerStack{}

	lateCore, err := buildLateCoreRepositories(input, closers)
	if err != nil {
		t.Fatalf("buildLateCoreRepositories: %v", err)
	}
	if lateCore.subscriptionService == nil {
		t.Fatal("expected late core subscription service")
	}
	if lateCore.assetRepository == nil {
		t.Fatal("expected late core asset repository")
	}
	if lateCore.reviewRepository == nil {
		t.Fatal("expected late core review repository")
	}
	if lateCore.resolutionCacheStore == nil {
		t.Fatal("expected late core resolution cache store")
	}
}

func TestBuildAdminRepositoriesComposesCatalogAndRulePhases(t *testing.T) {
	t.Parallel()

	input := buildSuccessfulServiceInputFixture()
	input.Repositories.Admin.Store = func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
		return &listingadmin.GormStoreRepository{}, nil, nil
	}
	input.Repositories.Admin.ImportTask = func(*config.Config, *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error) {
		return &listingadmin.GormImportTaskRepository{}, nil, nil
	}
	input.Repositories.Admin.FilterRule = func(*config.Config, *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error) {
		return &listingadmin.GormFilterRuleRepository{}, nil, nil
	}
	input.Repositories.Admin.PricingRule = func(*config.Config, *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error) {
		return &listingadmin.GormPricingRuleRepository{}, nil, nil
	}
	closers := &closerStack{}

	adminRepos, err := buildAdminRepositories(input, closers)
	if err != nil {
		t.Fatalf("buildAdminRepositories: %v", err)
	}
	if adminRepos.storeRepository == nil {
		t.Fatal("expected admin catalog store repository")
	}
	if adminRepos.importTaskRepository == nil {
		t.Fatal("expected admin catalog import task repository")
	}
	if adminRepos.filterRuleRepository == nil {
		t.Fatal("expected admin rule filter repository")
	}
	if adminRepos.pricingRuleRepository == nil {
		t.Fatal("expected admin rule pricing repository")
	}
}

func TestBuildCoreRepositoriesComposesTaskAndAsyncPhases(t *testing.T) {
	t.Parallel()

	input := buildSuccessfulServiceInputFixture()
	input.Repositories.Core.Task = func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error) {
		return listingkitstore.NewMemTaskRepository(), nil, nil
	}
	input.Repositories.Core.StudioAsyncJob = func(*config.Config, *logrus.Logger) (listingkit.StudioAsyncJobRepository, []func() error, error) {
		return listingkit.NewMemStudioAsyncJobRepository(), nil, nil
	}
	closers := &closerStack{}

	coreRepos, err := buildCoreRepositories(input, closers)
	if err != nil {
		t.Fatalf("buildCoreRepositories: %v", err)
	}
	if coreRepos.taskRepository == nil {
		t.Fatal("expected core task repository")
	}
	if coreRepos.studioAsyncJobRepository == nil {
		t.Fatal("expected core studio async job repository")
	}
}

func TestBuildSubmitModuleResolvesSheinRegistrarDependencies(t *testing.T) {
	t.Parallel()

	uploadStore := &httpapiStubImageUploadStore{}
	apiFactory := &httpapiStubSheinAPIClientFactory{}
	categoryLLM := &httpapiStubChatCompleter{id: "category"}
	saleAttrLLM := &httpapiStubChatCompleter{id: "sale-attr"}
	productAPIBuilder := &httpapiStubProductAPIBuilder{}
	imageAPIBuilder := &httpapiStubImageAPIBuilder{}
	translateAPIBuilder := &httpapiStubTranslateAPIBuilder{}
	imageGenerator := &httpapiStubImageGenerator{}
	storeRepo := &listingadmin.GormStoreRepository{}
	resolutionCache := &httpapiStubResolutionCacheStore{}
	categoryResolverBuilt := false
	attributeResolverBuilt := false
	saleAttributeResolverBuilt := false

	module := buildSubmitModule(submitModuleInput{
		Config:            &config.Config{Management: config.ManagementConfig{StoreIDs: []int64{903}}},
		Logger:            logrus.New(),
		AICredentialStore: nil,
		Hooks: submitModuleHooks{
			SheinPricingPolicyBuilder: func(*config.Config) sheinpub.PricingPolicy {
				return sheinpub.PricingPolicy{}
			},
			ImageUploadStoreBuilder: func(*config.Config, *logrus.Logger) listingkit.ImageUploadStore {
				return uploadStore
			},
			SheinCategoryLLMClientBuilder: func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
				return categoryLLM
			},
			SheinSaleAttributeLLMBuilder: func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
				return saleAttrLLM
			},
			SheinCategoryResolverBuilder: func(repo listingadmin.StoreRepository, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver {
				categoryResolverBuilt = true
				if repo != storeRepo || llm != categoryLLM || cache != resolutionCache {
					t.Fatal("expected category resolver builder to receive submit-scoped dependencies")
				}
				return nil
			},
			SheinAttributeResolverBuilder: func(repo listingadmin.StoreRepository, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.AttributeResolver {
				attributeResolverBuilt = true
				if repo != storeRepo || llm != saleAttrLLM || cache != resolutionCache {
					t.Fatal("expected attribute resolver builder to receive submit-scoped dependencies")
				}
				return nil
			},
			SheinSaleAttributeResolverBuilder: func(repo listingadmin.StoreRepository, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.SaleAttributeResolver {
				saleAttributeResolverBuilt = true
				if repo != storeRepo || llm != saleAttrLLM || cache != resolutionCache {
					t.Fatal("expected sale attribute resolver builder to receive submit-scoped dependencies")
				}
				return nil
			},
			SheinProductAPIBuilderFactory: func(repo listingadmin.StoreRepository) sheinpub.ProductAPIBuilder {
				if repo != storeRepo {
					t.Fatal("expected product api builder to receive store repository")
				}
				return productAPIBuilder
			},
			SheinImageAPIBuilderFactory: func(repo listingadmin.StoreRepository) sheinpub.ImageAPIBuilder {
				if repo != storeRepo {
					t.Fatal("expected image api builder to receive store repository")
				}
				return imageAPIBuilder
			},
			SheinTranslateAPIBuilderFactory: func(repo listingadmin.StoreRepository) sheinpub.TranslateAPIBuilder {
				if repo != storeRepo {
					t.Fatal("expected translate api builder to receive store repository")
				}
				return translateAPIBuilder
			},
			SheinAPIClientFactoryBuilder: func(repo listingadmin.StoreRepository) listingkit.SheinAPIClientFactory {
				if repo != storeRepo {
					t.Fatal("expected api client factory builder to receive store repository")
				}
				return apiFactory
			},
			StudioImageGeneratorBuilder: func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ImageGenerator {
				return imageGenerator
			},
			DefaultSheinStoreIDResolver: func(ids []int64) int64 {
				if !reflect.DeepEqual(ids, []int64{903}) {
					t.Fatalf("store ids = %v, want [903]", ids)
				}
				return ids[0]
			},
		},
		StoreRepository:      storeRepo,
		ResolutionCacheStore: resolutionCache,
	})

	if module.assets.imageUploadStore != uploadStore {
		t.Fatal("expected submit asset dependencies to preserve image upload store")
	}
	if module.shein.contentOptimizer != categoryLLM {
		t.Fatal("expected shein category llm client to be built by submit module")
	}
	if module.shein.productAPIBuilder != productAPIBuilder {
		t.Fatal("expected shein product api builder to be built by submit module")
	}
	if module.shein.imageAPIBuilder != imageAPIBuilder {
		t.Fatal("expected shein image api builder to be built by submit module")
	}
	if module.shein.translateAPIBuilder != translateAPIBuilder {
		t.Fatal("expected shein translate api builder to be built by submit module")
	}
	if module.shein.apiClientFactory != apiFactory {
		t.Fatal("expected shein api client factory to be built by submit module")
	}
	if module.studio.imageGenerator != imageGenerator {
		t.Fatal("expected studio image generator to be built by submit module")
	}
	if module.shein.defaultStoreID != 903 {
		t.Fatalf("default shein store id = %d, want 903", module.shein.defaultStoreID)
	}
	if module.assets.assembler == nil {
		t.Fatal("expected assembler to be built from submit-scoped dependencies")
	}
	if module.shein.categoryResolver != nil {
		t.Fatal("expected category resolver output to reflect builder return value")
	}
	if module.shein.attributeResolver != nil {
		t.Fatal("expected attribute resolver output to reflect builder return value")
	}
	if module.shein.saleAttributeResolver != nil {
		t.Fatal("expected sale attribute resolver output to reflect builder return value")
	}
	if !categoryResolverBuilt || !attributeResolverBuilt || !saleAttributeResolverBuilt {
		t.Fatal("expected shein resolver builders to be invoked by submit module")
	}
}

func TestBuildTaskModuleScopesRuntimeHandlerDependencies(t *testing.T) {
	t.Parallel()

	subscriptionService := &listingsubscription.Service{}
	module := buildTaskModule(taskModuleInput{
		TaskRepository:           listingkitstore.NewMemTaskRepository(),
		SubscriptionService:      subscriptionService,
		PlatformAdminUsers:       []string{"task-admin"},
		PlatformAdminRoles:       []string{"task-role"},
		StudioAsyncJobRepository: nil,
	})

	if module.taskRepository == nil {
		t.Fatal("expected task repository to be preserved")
	}
	if module.handlerDependencies.Subscription.Service != subscriptionService {
		t.Fatal("expected subscription service to be mapped into handler dependencies")
	}
	if !reflect.DeepEqual(module.handlerDependencies.Subscription.PlatformAdminUsers, []string{"task-admin"}) {
		t.Fatalf("platform admin users = %v, want [task-admin]", module.handlerDependencies.Subscription.PlatformAdminUsers)
	}
	if !reflect.DeepEqual(module.handlerDependencies.Subscription.PlatformAdminRoles, []string{"task-role"}) {
		t.Fatalf("platform admin roles = %v, want [task-role]", module.handlerDependencies.Subscription.PlatformAdminRoles)
	}
}

func TestBuildAdminModuleMapsAdminRepositories(t *testing.T) {
	t.Parallel()

	storeRepo := &listingadmin.GormStoreRepository{}
	categoryRepo := &listingadmin.GormCategoryRepository{}

	module := buildAdminModule(adminModuleInput{
		StoreRepository:    storeRepo,
		CategoryRepository: categoryRepo,
	})
	if module.handlerDependencies.StoreRepository != storeRepo {
		t.Fatal("expected store repository to be mapped into admin handler dependencies")
	}
	if module.handlerDependencies.CategoryRepository != categoryRepo {
		t.Fatal("expected category repository to be mapped into admin handler dependencies")
	}
}

func TestBuildSubmitModuleResolvesSubmitScopedHooks(t *testing.T) {
	t.Parallel()

	uploadStore := &httpapiStubImageUploadStore{}
	module := buildSubmitModule(submitModuleInput{
		Config: &config.Config{
			Management: config.ManagementConfig{StoreIDs: []int64{701}},
		},
		Logger: logrus.New(),
		Hooks: submitModuleHooks{
			ImageUploadStoreBuilder: func(*config.Config, *logrus.Logger) listingkit.ImageUploadStore {
				return uploadStore
			},
			DefaultSheinStoreIDResolver: func(ids []int64) int64 {
				if len(ids) == 1 {
					return ids[0]
				}
				return 0
			},
		},
	})

	if module.assets.imageUploadStore != uploadStore {
		t.Fatal("expected submit image upload store to be built from scoped hooks")
	}
	if module.shein.defaultStoreID != 701 {
		t.Fatalf("default shein store id = %d, want 701", module.shein.defaultStoreID)
	}
	if module.shein.contentOptimizer != nil {
		t.Fatal("expected omitted shein optimizer hook to leave zero value output")
	}
	if module.shein.productAPIBuilder != nil {
		t.Fatal("expected omitted submit builders to leave zero value outputs")
	}
	if module.studio.imageGenerator != nil {
		t.Fatal("expected omitted studio image generator hook to leave zero value output")
	}
	if module.assets.assembler == nil {
		t.Fatal("expected assembler to still be built when optional hooks are omitted")
	}
}

func TestBuildTemporalModuleExposesWorkerRuntime(t *testing.T) {
	t.Parallel()

	taskRepo := listingkitstore.NewMemTaskRepository()
	svc, err := listingkit.NewService(&listingkit.ServiceConfig{
		Core: listingkit.ServiceCoreDependencies{
			Repository:     taskRepo,
			ProductService: httpapiStubProductService{},
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	moduleSvc, ok := svc.(moduleService)
	if !ok {
		t.Fatalf("service type = %T, want moduleService", svc)
	}

	module := buildTemporalModule(temporalModuleInput{Service: moduleSvc})
	if module.workerService == nil {
		t.Fatal("expected temporal worker service boundary")
	}
	workerSvc, ok := any(module.workerService).(TemporalWorkerService)
	if !ok {
		t.Fatalf("worker service type = %T, want TemporalWorkerService", module.workerService)
	}
	if workerSvc != moduleSvc {
		t.Fatal("expected temporal module to preserve worker-capable service")
	}
}

func TestBuildServiceAssemblesRuntimeDependenciesFromRegistrars(t *testing.T) {
	t.Parallel()

	bundle, err := BuildService(buildSuccessfulServiceInputFixture())
	if err != nil {
		t.Fatalf("build service: %v", err)
	}
	if bundle.runtime.handlerDependencies.StudioAsyncJobRepository != bundle.StudioAsyncJobRepository {
		t.Fatal("expected runtime handler dependencies to preserve studio async job repository")
	}
	if bundle.runtime.handlerDependencies.Subscription.Service != bundle.SubscriptionService {
		t.Fatal("expected runtime handler dependencies to preserve subscription service")
	}
	if bundle.runtime.handlerDependencies.Admin.StoreRepository != bundle.StoreRepository {
		t.Fatal("expected runtime handler dependencies to preserve admin store repository")
	}
	if bundle.TemporalWorkerService == nil {
		t.Fatal("expected temporal worker service to be exposed")
	}
}

func TestBuildServiceRuntimeAssemblesBundleFromRegistrars(t *testing.T) {
	t.Parallel()

	input := buildSuccessfulServiceInputFixture()
	closers := &closerStack{}
	repositories, err := buildRepositories(input, closers)
	if err != nil {
		t.Fatalf("buildRepositories: %v", err)
	}

	bundle, err := buildServiceRuntime(input, repositories, closers)
	if err != nil {
		t.Fatalf("buildServiceRuntime: %v", err)
	}
	if bundle == nil {
		t.Fatal("expected service bundle")
	}
	if bundle.runtime.service == nil {
		t.Fatal("expected runtime service")
	}
	if bundle.runtime.handlerDependencies.Subscription.Service != bundle.SubscriptionService {
		t.Fatal("expected runtime handler dependencies to preserve subscription service")
	}
}

func TestBuildServiceRuntimeModulesComposeTaskAdminAndSubmitRegistrars(t *testing.T) {
	t.Parallel()

	input := buildSuccessfulServiceInputFixture()
	input.Hooks.ImageUploadStoreBuilder = func(*config.Config, *logrus.Logger) listingkit.ImageUploadStore {
		return &httpapiStubImageUploadStore{}
	}
	closers := &closerStack{}
	repositories, err := buildRepositories(input, closers)
	if err != nil {
		t.Fatalf("buildRepositories: %v", err)
	}

	modules := buildServiceRuntimeModules(input, repositories)
	if modules.task.handlerDependencies.Admin.StoreRepository != nil {
		t.Fatal("expected task module to remain admin-agnostic before composition")
	}
	if modules.admin.handlerDependencies.StoreRepository != repositories.storeRepository {
		t.Fatal("expected admin module to preserve store repository")
	}
	if modules.submit.assets.imageUploadStore == nil {
		t.Fatal("expected submit module to build image upload store")
	}
}

func TestAssembleServiceRuntimeBuildsTemporalAndHandlerDependencies(t *testing.T) {
	t.Parallel()

	input := buildSuccessfulServiceInputFixture()
	closers := &closerStack{}
	repositories, err := buildRepositories(input, closers)
	if err != nil {
		t.Fatalf("buildRepositories: %v", err)
	}

	assembly, err := assembleServiceRuntime(input, repositories, closers)
	if err != nil {
		t.Fatalf("assembleServiceRuntime: %v", err)
	}
	if assembly.service == nil {
		t.Fatal("expected runtime assembly service")
	}
	if assembly.modules.temporal.workerService == nil {
		t.Fatal("expected runtime assembly temporal worker service")
	}
	if assembly.handlerDependencies.Subscription.Service != repositories.subscriptionService {
		t.Fatal("expected runtime assembly to preserve subscription service")
	}
	if assembly.handlerDependencies.Admin.StoreRepository != repositories.storeRepository {
		t.Fatal("expected runtime assembly to preserve admin store repository")
	}
}

func TestBuildModuleRuntimeUsesPrivateRuntimePayload(t *testing.T) {
	t.Parallel()

	moduleSvc := newTestModuleService(t)
	runtimeTaskRepo := listingkitstore.NewMemTaskRepository()
	bundle := &ServiceBundle{
		TemporalWorkerService: nil,
		TaskRepository:        nil,
		runtime: serviceBundleRuntime{
			temporalWorkerService: moduleSvc,
			service:               moduleSvc,
			taskRepository:        runtimeTaskRepo,
			handlerDependencies: buildTaskModule(taskModuleInput{
				TaskRepository:           runtimeTaskRepo,
				StudioAsyncJobRepository: nil,
				SubscriptionService:      &listingsubscription.Service{},
			}).handlerDependenciesWithAdmin(buildAdminModule(adminModuleInput{})),
		},
	}

	module, err := buildModuleRuntime(BuildModuleInput{
		ServiceInput: BuildServiceInput{
			Config: buildSuccessfulServiceInputFixture().Config,
			Logger: logrus.New(),
		},
	}, bundle)
	if err != nil {
		t.Fatalf("build module runtime: %v", err)
	}
	if module == nil {
		t.Fatal("expected module runtime")
	}
	if module.Pool == nil {
		t.Fatal("expected worker pool when runtime payload provides processor dependencies")
	}
}

func TestAssembleModuleRuntimeBuildsProcessorAndPool(t *testing.T) {
	t.Parallel()

	moduleSvc := newTestModuleService(t)
	runtimeTaskRepo := listingkitstore.NewMemTaskRepository()
	bundle := &ServiceBundle{
		runtime: serviceBundleRuntime{
			service:        moduleSvc,
			taskRepository: runtimeTaskRepo,
			handlerDependencies: buildTaskModule(taskModuleInput{
				TaskRepository:           runtimeTaskRepo,
				StudioAsyncJobRepository: nil,
				SubscriptionService:      &listingsubscription.Service{},
			}).handlerDependenciesWithAdmin(buildAdminModule(adminModuleInput{})),
		},
	}

	assembly, err := assembleModuleRuntime(BuildModuleInput{
		ServiceInput: BuildServiceInput{
			Config: buildSuccessfulServiceInputFixture().Config,
			Logger: logrus.New(),
		},
	}, bundle)
	if err != nil {
		t.Fatalf("assembleModuleRuntime: %v", err)
	}
	if assembly.processor == nil {
		t.Fatal("expected runtime assembly processor")
	}
	if assembly.pool == nil {
		t.Fatal("expected runtime assembly pool")
	}
}

func TestPrepareModuleRuntimeClosersPreservesExistingRuntimeClosers(t *testing.T) {
	t.Parallel()

	closed := false
	bundle := &ServiceBundle{
		runtime: serviceBundleRuntime{
			closers: []func() error{
				func() error {
					closed = true
					return nil
				},
			},
		},
	}

	closers, err := prepareModuleRuntimeClosers(BuildModuleInput{
		ServiceInput: BuildServiceInput{
			Logger: logrus.New(),
		},
	}, bundle)
	if err != nil {
		t.Fatalf("prepareModuleRuntimeClosers: %v", err)
	}
	if closers == nil {
		t.Fatal("expected closer stack")
	}
	if len(closers.Snapshot()) != 1 {
		t.Fatalf("expected one inherited closer, got %d", len(closers.Snapshot()))
	}
	if err := closers.Close(); err != nil {
		t.Fatalf("close closers: %v", err)
	}
	if !closed {
		t.Fatal("expected inherited runtime closer to run")
	}
}

func TestCloseTemporalWorkflowClientOnErrorOnlyClosesOnFailure(t *testing.T) {
	t.Parallel()

	closed := false
	closer := func() error {
		closed = true
		return nil
	}

	if err := closeTemporalWorkflowClientOnError(nil, closer); err != nil {
		t.Fatalf("closeTemporalWorkflowClientOnError(nil): %v", err)
	}
	if closed {
		t.Fatal("expected nil error to skip closer")
	}

	expectedErr := errors.New("boom")
	if err := closeTemporalWorkflowClientOnError(expectedErr, closer); !errors.Is(err, expectedErr) {
		t.Fatalf("err = %v, want %v", err, expectedErr)
	}
	if !closed {
		t.Fatal("expected non-nil error to trigger closer")
	}
}

func TestBuildModuleAssemblesRuntimeFromModuleRegistrars(t *testing.T) {
	t.Parallel()

	module, err := BuildModule(BuildModuleInput{
		ServiceInput: buildSuccessfulServiceInputFixture(),
	})
	if err != nil {
		t.Fatalf("build module: %v", err)
	}
	if module == nil {
		t.Fatal("expected module")
	}
	if module.Handler == nil {
		t.Fatal("expected handler from public BuildModule entrypoint")
	}
	if module.StudioSessionHandler == nil {
		t.Fatal("expected studio session handler from public BuildModule entrypoint")
	}
	if module.Pool == nil {
		t.Fatal("expected worker pool from public BuildModule entrypoint")
	}
}

func TestBuildServiceComposesSubmitRegistrarThroughPublicEntryPoint(t *testing.T) {
	t.Parallel()

	input := buildSuccessfulServiceInputFixture()
	storeRepo := &listingadmin.GormStoreRepository{}
	resolutionCache := &httpapiStubResolutionCacheStore{}
	categoryLLM := &httpapiStubChatCompleter{id: "category"}
	imageUploadStore := &httpapiStubImageUploadStore{}
	categoryResolverBuilt := false
	imageUploadStoreBuilt := false

	input.Repositories.Admin.Store = func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
		return storeRepo, nil, nil
	}
	input.Repositories.Core.SheinResolutionCache = func(*config.Config, *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
		return resolutionCache, nil, nil
	}
	input.Hooks.SheinCategoryLLMClientBuilder = func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
		return categoryLLM
	}
	input.Hooks.SheinCategoryResolverBuilder = func(repo listingadmin.StoreRepository, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver {
		categoryResolverBuilt = true
		if repo != storeRepo || llm != categoryLLM || cache != resolutionCache {
			t.Fatal("expected submit registrar to resolve shein category dependencies through BuildService")
		}
		return nil
	}
	input.Hooks.ImageUploadStoreBuilder = func(*config.Config, *logrus.Logger) listingkit.ImageUploadStore {
		imageUploadStoreBuilt = true
		return imageUploadStore
	}

	bundle, err := BuildService(input)
	if err != nil {
		t.Fatalf("build service: %v", err)
	}
	if !categoryResolverBuilt {
		t.Fatal("expected submit registrar category resolver builder to run")
	}
	if !imageUploadStoreBuilt {
		t.Fatal("expected submit registrar image upload store builder to run")
	}
	if bundle == nil {
		t.Fatal("expected bundle")
	}
}

func buildSuccessfulServiceInputFixture() BuildServiceInput {
	input := buildServiceInputFixture()
	input.Logger = logrus.New()
	input.ProductService = httpapiStubProductService{}
	input.Repositories.Core.Task = func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error) {
		return listingkitstore.NewMemTaskRepository(), nil, nil
	}
	input.Repositories.Core.StudioAsyncJob = func(*config.Config, *logrus.Logger) (listingkit.StudioAsyncJobRepository, []func() error, error) {
		return listingkit.NewMemStudioAsyncJobRepository(), nil, nil
	}
	input.Repositories.Core.StudioBatch = func(*config.Config, *logrus.Logger) (listingkit.StudioBatchRepository, []func() error, error) {
		return listingkit.NewMemStudioBatchRepository(), nil, nil
	}
	input.Repositories.Core.StudioBatchRun = func(*config.Config, *logrus.Logger) (listingkit.StudioBatchRunRepository, []func() error, error) {
		return listingkit.NewMemStudioBatchRunRepository(), nil, nil
	}
	input.Repositories.Core.Subscription = func(*config.Config, *logrus.Logger) (listingsubscription.Repository, []func() error, error) {
		return listingsubscription.NewMemRepository(), nil, nil
	}
	return input
}

func newTestModuleService(t *testing.T) moduleService {
	t.Helper()

	svc, err := listingkit.NewService(&listingkit.ServiceConfig{
		Core: listingkit.ServiceCoreDependencies{
			Repository:     listingkitstore.NewMemTaskRepository(),
			ProductService: httpapiStubProductService{},
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	moduleSvc, ok := svc.(moduleService)
	if !ok {
		t.Fatalf("service type = %T, want moduleService", svc)
	}
	return moduleSvc
}

type httpapiStubProductService struct{}

type httpapiStubChatCompleter struct {
	id string
}

func (s *httpapiStubChatCompleter) CreateChatCompletion(context.Context, *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return nil, nil
}

func (s *httpapiStubChatCompleter) Generate(context.Context, string) (string, error) {
	return "", nil
}

func (s *httpapiStubChatCompleter) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", nil
}

func (s *httpapiStubChatCompleter) GetDefaultModel() string {
	return s.id
}

type httpapiStubProductAPIBuilder struct{}

func (*httpapiStubProductAPIBuilder) BuildProductAPI(context.Context, int64) (sheinproductapi.ProductAPI, string) {
	return nil, ""
}

type httpapiStubImageAPIBuilder struct{}

func (*httpapiStubImageAPIBuilder) BuildImageAPI(context.Context, int64) (sheinimageapi.ImageAPI, string) {
	return nil, ""
}

type httpapiStubTranslateAPIBuilder struct{}

func (*httpapiStubTranslateAPIBuilder) BuildTranslateAPI(context.Context, int64) (sheintranslateapi.TranslateAPI, string) {
	return nil, ""
}

type httpapiStubImageGenerator struct{}

func (*httpapiStubImageGenerator) GenerateImage(context.Context, *openaiclient.ImageGenerateRequest) (*openaiclient.ImageResponse, error) {
	return nil, nil
}

func (*httpapiStubImageGenerator) EditImage(context.Context, *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error) {
	return nil, nil
}

func (*httpapiStubImageGenerator) GetDefaultModel() string {
	return ""
}

type httpapiStubResolutionCacheStore struct{}

func (*httpapiStubResolutionCacheStore) GetResolutionCache(context.Context, string, string, string) (*sheinpub.SheinResolutionCacheEntry, error) {
	return nil, nil
}

func (*httpapiStubResolutionCacheStore) SaveResolutionCache(context.Context, *sheinpub.SheinResolutionCacheEntry) error {
	return nil
}

func (*httpapiStubResolutionCacheStore) DeleteResolutionCache(context.Context, string, string, string) error {
	return nil
}

type httpapiStubImageUploadStore struct{}

func (httpapiStubImageUploadStore) Save(context.Context, *listingkit.ImageUploadInput) (*listingkit.StoredUploadedImage, error) {
	return nil, nil
}
func (httpapiStubImageUploadStore) Open(context.Context, string) (*listingkit.StoredUploadedImage, error) {
	return nil, nil
}
func (httpapiStubImageUploadStore) Delete(context.Context, string) error { return nil }

type httpapiStubSheinAPIClientFactory struct{}

func (httpapiStubSheinAPIClientFactory) NewSheinAPIClient(int64, *listingkit.SheinStoreInfo) *sheinclient.APIClient {
	return nil
}

func (httpapiStubProductService) CreateGenerateTask(context.Context, *productenrich.GenerateRequest) (*productenrich.Task, error) {
	return nil, nil
}

func (httpapiStubProductService) GetTaskResult(context.Context, string) (*productenrich.TaskResult, error) {
	return nil, nil
}

func (httpapiStubProductService) ProcessProduct(context.Context, *productenrich.Task) (*productenrich.ProductJSON, error) {
	return nil, nil
}

func (httpapiStubProductService) SetTaskSubmitter(productenrich.TaskSubmitter) {}
