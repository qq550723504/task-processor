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
