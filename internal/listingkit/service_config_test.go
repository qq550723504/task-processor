package listingkit

import (
	"testing"

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
