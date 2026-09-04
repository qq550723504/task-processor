package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	a1688handoff "task-processor/internal/compatibility/listingkit/sourcehandoff/a1688"
	a1688httpapi "task-processor/internal/compatibility/listingkit/sourcehandoff/a1688/httpapi"
	localagent "task-processor/internal/localagent"
	localagenthttpapi "task-processor/internal/localagent/httpapi"
	"task-processor/internal/sourceaccount"
)

type httpFeatureCompositionBuilder struct {
	buildAmazonListing    amazonListingModuleBuilder
	buildAmazonRepo       amazonListingRepositoryBuilder
	buildSheinLogin       sheinLoginModuleBuilder
	buildSDSLogin         sdsLoginModuleBuilder
	buildListingKit       listingKitModuleBuilder
	buildListingRepos     listingKitRepositoryBuilder
	buildPrompt           promptModuleBuilder
	buildTaskRPC          taskRPCModuleBuilder
	buildSDS              sdsModuleBuilder
	buildSourceAccount    sourceAccountRepositoryBuilder
	buildImageAgent       imageAgentModuleBuilder
	buildWorkbenchContext workbenchContextModuleBuilder
	buildStoreCenter      storeCenterModuleBuilder
}

func newHTTPFeatureCompositionBuilder() httpFeatureCompositionBuilder {
	return httpFeatureCompositionBuilder{
		buildAmazonListing:    buildAmazonListingModuleResult,
		buildAmazonRepo:       newDBAmazonListingTaskRepository,
		buildSheinLogin:       buildSheinLoginModuleResult,
		buildSDSLogin:         buildSDSLoginModuleResult,
		buildListingKit:       buildListingKitModuleResult,
		buildListingRepos:     buildListingKitPersistentRepositories,
		buildPrompt:           buildPromptModuleResult,
		buildTaskRPC:          buildTaskRPCModuleResult,
		buildSDS:              buildSDSModuleResult,
		buildSourceAccount:    buildSourceAccountRepository,
		buildImageAgent:       buildImageAgentModuleResult,
		buildWorkbenchContext: buildDefaultWorkbenchContextModule,
		buildStoreCenter:      buildDefaultStoreCenterModule,
	}
}

func (b httpFeatureCompositionBuilder) build(logger *logrus.Logger, deps *runtimeDeps) (httpFeatureComposition, error) {
	var composition httpFeatureComposition
	timer := newStartupTimer(logger)
	if err := initializeProductSnapshotReader(deps); err != nil {
		return composition, err
	}
	if deps == nil || deps.shared == nil || deps.shared.cfg == nil {
		return composition, fmt.Errorf("build product repositories: runtime config is required")
	}
	if b.buildListingRepos == nil {
		return composition, fmt.Errorf("build product repositories: listingkit repository builder is required")
	}
	listingKitRepositories, listingKitCloser, err := b.buildListingRepos(deps.shared.cfg.Database, logger)
	if err != nil {
		return composition, fmt.Errorf("build listingkit repositories: %w", err)
	}
	deps.ensureListingKitSupport().repositories = listingKitRepositories
	deps.addClosers(listingKitCloser)

	done := timer.phase("buildAmazonListingModule")
	amazonListingModule, err := amazonListingFeatureBuilder{
		buildAmazonListing: b.buildAmazonListing,
		buildRepository:    b.buildAmazonRepo,
	}.build(logger, deps)
	done()
	if err != nil {
		return composition, err
	}
	composition.amazonListingModule = amazonListingModule

	done = timer.phase("buildSheinLoginModule")
	loginFeatures, err := loginFeatureBuilder{
		buildSheinLogin: b.buildSheinLogin,
		buildSDSLogin:   b.buildSDSLogin,
	}.build(deps)
	done()
	if err != nil {
		return composition, err
	}
	composition.sheinLoginResult = loginFeatures.sheinLoginResult
	composition.sdsLoginResult = loginFeatures.sdsLoginResult

	done = timer.phase("buildListingKitModule")
	listingKitModule, err := listingKitFeatureBuilder{buildListingKit: b.buildListingKit}.build(logger, deps)
	done()
	if err != nil {
		return composition, err
	}
	composition.listingKitModule = listingKitModule
	var sourceRepository sourceaccount.Repository
	var sourceValidator sourceaccount.AccessValidator
	if composition.listingKitModule != nil {
		var sourceErr error
		var sourceClosers []func() error
		if b.buildSourceAccount != nil {
			sourceRepository, sourceClosers, sourceErr = b.buildSourceAccount(deps.shared.cfg, logger)
		}
		if sourceErr != nil {
			logger.Warn("1688 source-account repository unavailable; public crawling remains enabled")
		}
		deps.addClosers(sourceClosers...)
		if validator, ok := sourceRepository.(sourceaccount.AccessValidator); ok {
			sourceValidator = validator
		}
	}
	if composition.listingKitModule != nil && deps.shared.cfg != nil && deps.shared.cfg.Platforms.Alibaba1688.Enabled {
		crawlerModule := newCrawler1688HTTPModule(deps.shared.cfg, logger, sourceRepository)
		composition.crawler1688Module = crawlerModule
		deps.addClosers(crawlerModule.Close)
	}
	if composition.listingKitModule != nil && composition.listingKitModule.TaskLifecycleService != nil && composition.listingKitModule.StoreAccessValidator != nil {
		composition.productSourcingModule = a1688httpapi.BuildModule(
			a1688handoff.NewTaskCommandService(composition.listingKitModule.TaskLifecycleService, composition.listingKitModule.StoreAccessValidator, sourceValidator, deps.features.productSnapshotPublisher),
		)
	}
	if composition.listingKitModule != nil {
		composition.localAgentModule = localagenthttpapi.BuildModule(localagent.NewService(nil))
	}
	if b.buildImageAgent != nil {
		done = timer.phase("buildImageAgentModule")
		imageAgentModule, imageAgentErr := b.buildImageAgent(deps.shared.cfg, logger)
		done()
		if imageAgentErr != nil {
			return composition, imageAgentErr
		}
		composition.imageAgentModule = imageAgentModule
		if imageAgentModule != nil {
			deps.addClosers(imageAgentModule.Closers...)
		}
	}

	done = timer.phase("buildSupportModules")
	supportFeatures, err := supportFeatureBuilder{
		buildPrompt:  b.buildPrompt,
		buildTaskRPC: b.buildTaskRPC,
		buildSDS:     b.buildSDS,
	}.build(logger, deps, composition)
	done()
	if err != nil {
		return composition, err
	}
	composition.promptModule = supportFeatures.promptModule
	composition.taskRPCResult = supportFeatures.taskRPCResult
	composition.sdsModule = supportFeatures.sdsModule

	done = timer.phase("buildWorkbenchModules")
	if err := b.buildWorkbenchModules(logger, deps, &composition); err != nil {
		done()
		return composition, err
	}
	done()
	timer.total("buildHTTPFeatureComposition")

	return composition, nil
}

func (b httpFeatureCompositionBuilder) buildWorkbenchModules(logger *logrus.Logger, deps *runtimeDeps, composition *httpFeatureComposition) error {
	if deps == nil || deps.shared == nil || composition == nil {
		return fmt.Errorf("build Workbench modules: runtime dependencies are required")
	}
	if b.buildWorkbenchContext != nil {
		workbenchResult, err := b.buildWorkbenchContext(deps.shared.cfg, logger)
		if err != nil {
			return err
		}
		composition.workbenchContextModule = workbenchResult.module
		composition.workbenchAuthDependencies = workbenchResult.authDependencies
	}
	if b.buildStoreCenter == nil {
		return nil
	}
	if deps.shared.cfg != nil && deps.shared.cfg.Workbench.Enabled && (composition.workbenchContextModule == nil || composition.workbenchAuthDependencies == nil) {
		return fmt.Errorf("build Store Center: Workbench context dependencies are required")
	}
	storeResult, err := b.buildStoreCenter(deps.shared.cfg, logger)
	if err != nil {
		return err
	}
	composition.storeCenterModule = storeResult.module
	deps.addClosers(storeResult.closer)
	return nil
}
