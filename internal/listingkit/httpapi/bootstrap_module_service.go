package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	listingkitapi "task-processor/internal/listingkit/api"
)

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
		listingkitapi.WithChildTaskRetryService(runtime.service),
		listingkitapi.WithStoreAdminService(runtime.service),
		listingkitapi.WithStudioMediaService(runtime.service),
		listingkitapi.WithStudioBatchRunService(runtime.service),
		listingkitapi.WithDependencies(runtime.handlerDependencies),
		listingkitapi.WithSheinSyncRepository(runtime.sheinSyncRepository),
		listingkitapi.WithSheinSyncServices(runtime.sheinSyncService, runtime.sheinCandidateService, runtime.sheinEnrollmentService),
	}
}
