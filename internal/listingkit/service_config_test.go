package listingkit

import (
	"testing"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
)

type testServiceConfigOption func(*ServiceConfig)

func newTestServiceConfig(repo Repository, opts ...testServiceConfigOption) *ServiceConfig {
	cfg := &ServiceConfig{
		Core: ServiceCoreDependencies{
			Repository:     repo,
			ProductService: stubSubmitProductService{},
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return cfg
}

func withTestProductService(productSvc ProductService) testServiceConfigOption {
	return func(cfg *ServiceConfig) {
		cfg.Core.ProductService = productSvc
	}
}

func withTestAssembler(assembler Assembler) testServiceConfigOption {
	return func(cfg *ServiceConfig) {
		cfg.Assets.Assembler = assembler
	}
}

func withTestTaskSubmitter(taskSubmitter TaskSubmitter) testServiceConfigOption {
	return func(cfg *ServiceConfig) {
		cfg.Core.TaskSubmitter = taskSubmitter
	}
}

func withTestConfig(apply func(*ServiceConfig)) testServiceConfigOption {
	return func(cfg *ServiceConfig) {
		if apply != nil {
			apply(cfg)
		}
	}
}

func withTestSheinProductAPIBuilder(builder sheinpub.ProductAPIBuilder) testServiceConfigOption {
	return func(cfg *ServiceConfig) {
		cfg.Shein.SheinProductAPIBuilder = builder
	}
}

func withTestSheinImageAPIBuilder(builder sheinpub.ImageAPIBuilder) testServiceConfigOption {
	return func(cfg *ServiceConfig) {
		cfg.Shein.SheinImageAPIBuilder = builder
	}
}

func withTestSheinPublishWorkflow(client SheinPublishWorkflowClient, enabled bool) testServiceConfigOption {
	return func(cfg *ServiceConfig) {
		cfg.Workflow.SheinPublishWorkflowClient = client
		cfg.Workflow.SheinPublishWorkflowEnabled = enabled
	}
}

func withDefaultTestSheinImageAPI() testServiceConfigOption {
	return withTestSheinImageAPIBuilder(stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}})
}

type testConfigCategoryResolver struct{}

func (testConfigCategoryResolver) Resolve(req *sheinpub.BuildRequest, canonical *canonical.Product, pkg *sheinpub.Package) *sheinpub.CategoryResolution {
	return &sheinpub.CategoryResolution{Status: "resolved"}
}

func TestNewServiceWithConfigInitializesSubmitLockManager(t *testing.T) {
	t.Parallel()

	svc := newServiceWithConfig(newTestServiceConfig(&stubSubmitRepo{}))
	if svc == nil {
		t.Fatal("expected service instance")
	}
	if svc.submission.sheinSubmitLocks == nil {
		t.Fatal("expected shein submit locks to be initialized")
	}
}

func TestNewServiceWithConfigSeedsDependencyGroupsBeforeLegacyMirrors(t *testing.T) {
	t.Parallel()

	sessionRepo := &studioBatchRunExecutorSessionRepoStub{}
	syncSvc := &stubWorkflowSDSSyncService{}
	statusProvider := stubSDSLoginStatusProvider{}
	credentialStore := &fakeAIClientCredentialStore{}
	submitter := noopTaskSubmitter{}
	workflowClient := &stubStandardProductWorkflowClient{}

	svc := newServiceWithConfig(newTestServiceConfig(
		&stubSubmitRepo{},
		withTestTaskSubmitter(submitter),
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Core.StudioSessionRepository = sessionRepo
			cfg.Core.SDSSyncService = syncSvc
			cfg.Core.SDSLoginStatusProvider = statusProvider
			cfg.Core.AIClientCredentialStore = credentialStore
			cfg.Workflow.StandardProductWorkflowClient = workflowClient
			cfg.Workflow.StandardProductWorkflowEnabled = true
		}),
	))

	if svc.repo == nil {
		t.Fatal("expected repository to remain eagerly available")
	}
	if svc.taskDeps.taskSubmitter != submitter {
		t.Fatalf("task deps submitter = %v, want seeded submitter", svc.taskDeps.taskSubmitter)
	}
	if svc.studioDeps.sessionRepo != sessionRepo {
		t.Fatalf("studio deps session repo = %v, want seeded repo", svc.studioDeps.sessionRepo)
	}
	if svc.supportDeps.sdsSyncService != syncSvc {
		t.Fatalf("support deps sync service = %v, want seeded service", svc.supportDeps.sdsSyncService)
	}
	if svc.taskDeps.sdsLoginStatusProvider != statusProvider {
		t.Fatalf("task deps status provider = %v, want seeded provider", svc.taskDeps.sdsLoginStatusProvider)
	}
	if svc.adminDeps.aiCredentialStore != credentialStore {
		t.Fatalf("admin deps AI credential store = %v, want seeded store", svc.adminDeps.aiCredentialStore)
	}
	if svc.taskDeps.standardWorkflowClient != workflowClient || !svc.taskDeps.standardWorkflowEnabled {
		t.Fatalf("task deps standard workflow = (%v, %v), want seeded+enabled", svc.taskDeps.standardWorkflowClient, svc.taskDeps.standardWorkflowEnabled)
	}

	if svc.runtime.taskSubmitter != nil {
		t.Fatalf("legacy taskSubmitter runtime mirror = %v, want nil before resolver sync", svc.runtime.taskSubmitter)
	}
	if svc.mirrors.sdsSyncSvc != nil {
		t.Fatalf("legacy sdsSyncSvc mirror = %v, want nil before resolver sync", svc.mirrors.sdsSyncSvc)
	}
	if svc.mirrors.sdsLoginStatusProvider != nil {
		t.Fatalf("legacy sdsLoginStatusProvider mirror = %v, want nil before resolver sync", svc.mirrors.sdsLoginStatusProvider)
	}
	if svc.runtime.standardProductWorkflowClient != nil || svc.runtime.standardProductWorkflowEnabled {
		t.Fatalf("legacy standard workflow runtime = (%v, %v), want nil+disabled before resolver sync", svc.runtime.standardProductWorkflowClient, svc.runtime.standardProductWorkflowEnabled)
	}

	if got := resolveTaskSubmitter(svc); got != submitter {
		t.Fatalf("resolveTaskSubmitter() = %v, want seeded submitter", got)
	}
	if got := resolveStudioSessionRepo(svc); got != sessionRepo {
		t.Fatalf("resolveStudioSessionRepo() = %v, want seeded repo", got)
	}
	if got := resolveSDSSyncService(svc); got != syncSvc {
		t.Fatalf("resolveSDSSyncService() = %v, want seeded service", got)
	}
	if got := resolveSDSLoginStatusProvider(svc); got != statusProvider {
		t.Fatalf("resolveSDSLoginStatusProvider() = %v, want seeded provider", got)
	}
	if got := resolveAdminAICredentialStore(svc); got != credentialStore {
		t.Fatalf("resolveAdminAICredentialStore() = %v, want seeded store", got)
	}
	if got, enabled := resolveStandardWorkflowClient(svc); got != workflowClient || !enabled {
		t.Fatalf("resolveStandardWorkflowClient() = (%v, %v), want seeded+enabled", got, enabled)
	}

	if svc.runtime.taskSubmitter != submitter {
		t.Fatalf("legacy taskSubmitter runtime mirror = %v, want hydrated submitter", svc.runtime.taskSubmitter)
	}
	if svc.mirrors.sdsSyncSvc != syncSvc {
		t.Fatalf("legacy sdsSyncSvc mirror = %v, want hydrated service", svc.mirrors.sdsSyncSvc)
	}
	if svc.mirrors.sdsLoginStatusProvider != statusProvider {
		t.Fatalf("legacy sdsLoginStatusProvider mirror = %v, want hydrated provider", svc.mirrors.sdsLoginStatusProvider)
	}
	if svc.runtime.standardProductWorkflowClient != workflowClient || !svc.runtime.standardProductWorkflowEnabled {
		t.Fatalf("legacy standard workflow runtime = (%v, %v), want hydrated+enabled", svc.runtime.standardProductWorkflowClient, svc.runtime.standardProductWorkflowEnabled)
	}
}

func TestNewServiceWithConfigSeedsSubmissionDependenciesWithoutLegacyMirrors(t *testing.T) {
	t.Parallel()

	storeProfileRepo := newInMemoryStoreProfileRepository()
	productBuilder := stubSheinProductAPIBuilder{api: &stubSheinProductAPI{}}
	imageBuilder := stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}}
	translateBuilder := stubSheinTranslateAPIBuilder{api: &stubSheinTranslateAPI{}}

	svc := newServiceWithConfig(newTestServiceConfig(
		&stubSubmitRepo{},
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Core.StoreProfileRepository = storeProfileRepo
			cfg.Shein.SheinProductAPIBuilder = productBuilder
			cfg.Shein.SheinImageAPIBuilder = imageBuilder
			cfg.Shein.SheinTranslateAPIBuilder = translateBuilder
		}),
	))

	if svc.submissionDeps.storeProfileRepo != storeProfileRepo {
		t.Fatalf("submission deps store profile repo = %v, want seeded repo", svc.submissionDeps.storeProfileRepo)
	}
	if svc.submissionDeps.sheinProductAPIBuilder != productBuilder {
		t.Fatalf("submission deps shein product api builder = %v, want seeded builder", svc.submissionDeps.sheinProductAPIBuilder)
	}
	if svc.submissionDeps.sheinImageAPIBuilder != imageBuilder {
		t.Fatalf("submission deps shein image api builder = %v, want seeded builder", svc.submissionDeps.sheinImageAPIBuilder)
	}
	if svc.submissionDeps.sheinTranslateAPIBuilder != translateBuilder {
		t.Fatalf("submission deps shein translate api builder = %v, want seeded builder", svc.submissionDeps.sheinTranslateAPIBuilder)
	}

	if got := resolveSubmissionStoreProfileRepo(svc); got != storeProfileRepo {
		t.Fatalf("resolveSubmissionStoreProfileRepo() = %v, want seeded repo", got)
	}
	if got := resolveSubmissionProductAPIBuilder(svc); got != productBuilder {
		t.Fatalf("resolveSubmissionProductAPIBuilder() = %v, want seeded builder", got)
	}
	if got := resolveSubmissionImageAPIBuilder(svc); got != imageBuilder {
		t.Fatalf("resolveSubmissionImageAPIBuilder() = %v, want seeded builder", got)
	}
	if got := resolveSubmissionTranslateAPIBuilder(svc); got != translateBuilder {
		t.Fatalf("resolveSubmissionTranslateAPIBuilder() = %v, want seeded builder", got)
	}
}

func TestNewServiceWithConfigSeedsSheinRuntimeDependenciesWithoutLegacyMirrors(t *testing.T) {
	t.Parallel()

	resolutionCacheStore := &submitResolutionCacheStore{}
	categoryResolver := testConfigCategoryResolver{}
	attributeResolver := stubRevisionSheinAttributeResolver{}
	saleAttributeResolver := stubRevisionSheinSaleResolver{}
	pricingPolicy := sheinpub.PricingPolicy{
		Enabled:        true,
		MarkupRate:     0.12,
		FixedMarkup:    1,
		ShippingCost:   2,
		CommissionRate: 0.1,
		RoundTo:        0.01,
	}

	svc := newServiceWithConfig(newTestServiceConfig(
		&stubSubmitRepo{},
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Shein.SheinResolutionCacheStore = resolutionCacheStore
			cfg.Shein.SheinCategoryResolver = categoryResolver
			cfg.Shein.SheinAttributeResolver = attributeResolver
			cfg.Shein.SheinSaleAttributeResolver = saleAttributeResolver
			cfg.Shein.SheinPricingPolicy = pricingPolicy
		}),
	))

	if svc.sheinRuntimeDeps.resolutionCacheStore != resolutionCacheStore {
		t.Fatalf("shein runtime deps resolution cache store = %v, want seeded store", svc.sheinRuntimeDeps.resolutionCacheStore)
	}
	if svc.sheinRuntimeDeps.categoryResolver != categoryResolver {
		t.Fatalf("shein runtime deps category resolver = %v, want seeded resolver", svc.sheinRuntimeDeps.categoryResolver)
	}
	if svc.sheinRuntimeDeps.attributeResolver != attributeResolver {
		t.Fatalf("shein runtime deps attribute resolver = %v, want seeded resolver", svc.sheinRuntimeDeps.attributeResolver)
	}
	if svc.sheinRuntimeDeps.saleAttributeResolver != saleAttributeResolver {
		t.Fatalf("shein runtime deps sale attribute resolver = %v, want seeded resolver", svc.sheinRuntimeDeps.saleAttributeResolver)
	}
	if svc.sheinRuntimeDeps.pricingPolicy != pricingPolicy {
		t.Fatalf("shein runtime deps pricing policy = %+v, want seeded policy %+v", svc.sheinRuntimeDeps.pricingPolicy, pricingPolicy)
	}

	if got := resolveSheinResolutionCacheStore(svc); got != resolutionCacheStore {
		t.Fatalf("resolveSheinResolutionCacheStore() = %v, want seeded store", got)
	}
	if got := resolveSheinCategoryResolver(svc); got != categoryResolver {
		t.Fatalf("resolveSheinCategoryResolver() = %v, want seeded resolver", got)
	}
	if got := resolveSheinAttributeResolver(svc); got != attributeResolver {
		t.Fatalf("resolveSheinAttributeResolver() = %v, want seeded resolver", got)
	}
	if got := resolveSheinSaleAttributeResolver(svc); got != saleAttributeResolver {
		t.Fatalf("resolveSheinSaleAttributeResolver() = %v, want seeded resolver", got)
	}
	if got := resolveSheinPricingPolicy(svc); got != pricingPolicy {
		t.Fatalf("resolveSheinPricingPolicy() = %+v, want seeded policy %+v", got, pricingPolicy)
	}
}

func TestNewServiceWithConfigSeedsSharedSheinDependenciesPerOwnerGroup(t *testing.T) {
	t.Parallel()

	storeCatalog := &stubSheinStoreCatalog{}
	apiFactory := stubSheinAPIClientFactory{}
	contentOptimizer := &stubSheinContentAI{}

	svc := newServiceWithConfig(newTestServiceConfig(
		&stubSubmitRepo{},
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Shein.SheinStoreCatalog = storeCatalog
			cfg.Shein.SheinAPIClientFactory = apiFactory
			cfg.Shein.SheinContentOptimizer = contentOptimizer
		}),
	))

	if svc.submissionDeps.sheinStoreCatalog != storeCatalog {
		t.Fatalf("submission deps shein store catalog = %v, want seeded catalog", svc.submissionDeps.sheinStoreCatalog)
	}
	if svc.submissionDeps.sheinAPIClientFactory != apiFactory {
		t.Fatalf("submission deps shein api client factory = %v, want seeded factory", svc.submissionDeps.sheinAPIClientFactory)
	}
	if svc.submissionDeps.sheinContentOptimizer != contentOptimizer {
		t.Fatalf("submission deps shein content optimizer = %v, want seeded optimizer", svc.submissionDeps.sheinContentOptimizer)
	}
	if svc.sheinRuntimeDeps.storeCatalog != storeCatalog {
		t.Fatalf("shein runtime deps store catalog = %v, want seeded catalog", svc.sheinRuntimeDeps.storeCatalog)
	}
	if svc.sheinRuntimeDeps.apiClientFactory != apiFactory {
		t.Fatalf("shein runtime deps api client factory = %v, want seeded factory", svc.sheinRuntimeDeps.apiClientFactory)
	}
	if svc.workflowDeps.sheinContentOptimizer != contentOptimizer {
		t.Fatalf("workflow deps shein content optimizer = %v, want seeded optimizer", svc.workflowDeps.sheinContentOptimizer)
	}

	if got := resolveSubmissionStoreCatalog(svc); got != storeCatalog {
		t.Fatalf("resolveSubmissionStoreCatalog() = %v, want seeded catalog", got)
	}
	if got := resolveSubmissionAPIClientFactory(svc); got != apiFactory {
		t.Fatalf("resolveSubmissionAPIClientFactory() = %v, want seeded factory", got)
	}
	if got := resolveSubmissionContentOptimizer(svc); got != contentOptimizer {
		t.Fatalf("resolveSubmissionContentOptimizer() = %v, want seeded optimizer", got)
	}
	if got := resolveSheinStoreCatalog(svc); got != storeCatalog {
		t.Fatalf("resolveSheinStoreCatalog() = %v, want seeded catalog", got)
	}
	if got := resolveSheinAPIClientFactory(svc); got != apiFactory {
		t.Fatalf("resolveSheinAPIClientFactory() = %v, want seeded factory", got)
	}
	if got := resolveWorkflowSheinContentOptimizer(svc); got != contentOptimizer {
		t.Fatalf("resolveWorkflowSheinContentOptimizer() = %v, want seeded optimizer", got)
	}
}

func TestNewServiceWithConfigSeedsWorkflowDependenciesWithoutLegacyMirrors(t *testing.T) {
	t.Parallel()

	productSvc := &stubWorkflowProductService{}
	imageSvc := &stubWorkflowImageService{}
	assetRepository := assetrepo.NewMemRepository()
	assetRecipeResolver := newDefaultAssetRecipeResolver()
	assetBundleBuilder := newDefaultAssetBundleBuilder()
	assetGenerator := newDefaultAssetGenerationService()

	svc := newServiceWithConfig(newTestServiceConfig(
		&stubSubmitRepo{},
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Core.ProductService = productSvc
			cfg.Core.ImageService = imageSvc
			cfg.Assets.AssetRepository = assetRepository
			cfg.Assets.AssetRecipeResolver = assetRecipeResolver
			cfg.Assets.AssetBundleBuilder = assetBundleBuilder
			cfg.Assets.AssetGenerationService = assetGenerator
		}),
	))

	if svc.workflowDeps.productService != productSvc {
		t.Fatalf("workflow deps product service = %v, want seeded service", svc.workflowDeps.productService)
	}
	if svc.workflowDeps.imageService != imageSvc {
		t.Fatalf("workflow deps image service = %v, want seeded service", svc.workflowDeps.imageService)
	}
	if svc.workflowDeps.assetRepository != assetRepository {
		t.Fatalf("workflow deps asset repository = %v, want seeded repository", svc.workflowDeps.assetRepository)
	}
	if svc.workflowDeps.assetRecipeResolver != assetRecipeResolver {
		t.Fatalf("workflow deps asset recipe resolver = %v, want seeded resolver", svc.workflowDeps.assetRecipeResolver)
	}
	if svc.workflowDeps.assetBundleBuilder != assetBundleBuilder {
		t.Fatalf("workflow deps asset bundle builder = %v, want seeded builder", svc.workflowDeps.assetBundleBuilder)
	}
	if svc.workflowDeps.assetGenerationService != assetGenerator {
		t.Fatalf("workflow deps asset generation service = %v, want seeded service", svc.workflowDeps.assetGenerationService)
	}

	if got := resolveWorkflowProductService(svc); got != productSvc {
		t.Fatalf("resolveWorkflowProductService() = %v, want seeded service", got)
	}
	if got := resolveWorkflowImageService(svc); got != imageSvc {
		t.Fatalf("resolveWorkflowImageService() = %v, want seeded service", got)
	}
	if got := resolveWorkflowAssetRepository(svc); got != assetRepository {
		t.Fatalf("resolveWorkflowAssetRepository() = %v, want seeded repository", got)
	}
	if got := resolveWorkflowAssetRecipeResolver(svc); got != assetRecipeResolver {
		t.Fatalf("resolveWorkflowAssetRecipeResolver() = %v, want seeded resolver", got)
	}
	if got := resolveWorkflowAssetBundleBuilder(svc); got != assetBundleBuilder {
		t.Fatalf("resolveWorkflowAssetBundleBuilder() = %v, want seeded builder", got)
	}
	if got := resolveWorkflowAssetGenerationService(svc); got != assetGenerator {
		t.Fatalf("resolveWorkflowAssetGenerationService() = %v, want seeded service", got)
	}
}
