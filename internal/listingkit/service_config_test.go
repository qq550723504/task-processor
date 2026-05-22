package listingkit

import sheinpub "task-processor/internal/publishing/shein"

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
