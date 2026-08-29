package httpapi

import (
	"github.com/sirupsen/logrus"

	a1688handoff "task-processor/internal/compatibility/listingkit/sourcehandoff/a1688"
	a1688httpapi "task-processor/internal/compatibility/listingkit/sourcehandoff/a1688/httpapi"
	localagent "task-processor/internal/localagent"
	localagenthttpapi "task-processor/internal/localagent/httpapi"
	"task-processor/internal/sourceaccount"
)

type httpFeatureCompositionBuilder struct {
	buildProduct       productModuleBuilder
	buildImage         imageModuleBuilder
	buildAmazonListing amazonListingModuleBuilder
	buildSheinLogin    sheinLoginModuleBuilder
	buildSDSLogin      sdsLoginModuleBuilder
	buildListingKit    listingKitModuleBuilder
	buildPrompt        promptModuleBuilder
	buildTaskRPC       taskRPCModuleBuilder
	buildSDS           sdsModuleBuilder
	buildSourceAccount sourceAccountRepositoryBuilder
	buildImageAgent    imageAgentModuleBuilder
}

func newHTTPFeatureCompositionBuilder() httpFeatureCompositionBuilder {
	return httpFeatureCompositionBuilder{
		buildProduct:       buildProductModuleResult,
		buildImage:         buildImageModuleResult,
		buildAmazonListing: buildAmazonListingModuleResult,
		buildSheinLogin:    buildSheinLoginModuleResult,
		buildSDSLogin:      buildSDSLoginModuleResult,
		buildListingKit:    buildListingKitModuleResult,
		buildPrompt:        buildPromptModuleResult,
		buildTaskRPC:       buildTaskRPCModuleResult,
		buildSDS:           buildSDSModuleResult,
		buildSourceAccount: buildSourceAccountRepository,
		buildImageAgent:    buildImageAgentModuleResult,
	}
}

func (b httpFeatureCompositionBuilder) build(logger *logrus.Logger, deps *runtimeDeps) (httpFeatureComposition, error) {
	var composition httpFeatureComposition
	timer := newStartupTimer(logger)

	done := timer.phase("buildProductImageModules")
	listingKitFeatures, err := listingKitFeatureBuilder{
		buildProduct:    b.buildProduct,
		buildImage:      b.buildImage,
		buildListingKit: b.buildListingKit,
	}.build(logger, deps, listingKitFeatureBuildOptions{includeImage: true})
	done()
	if err != nil {
		return composition, err
	}
	composition.productModule = listingKitFeatures.productModule
	composition.imageModule = listingKitFeatures.imageModule

	done = timer.phase("buildAmazonListingModule")
	amazonListingModule, err := amazonListingFeatureBuilder{
		buildAmazonListing: b.buildAmazonListing,
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
	listingKitFeatures, err = listingKitFeatureBuilder{
		buildProduct:    b.buildProduct,
		buildImage:      b.buildImage,
		buildListingKit: b.buildListingKit,
	}.build(logger, deps, listingKitFeatureBuildOptions{
		includeListingKit: true,
		skipProduct:       true,
	})
	done()
	if err != nil {
		return composition, err
	}
	composition.listingKitModule = listingKitFeatures.listingKitModule
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
			a1688handoff.NewTaskCommandService(composition.listingKitModule.TaskLifecycleService, composition.listingKitModule.StoreAccessValidator, sourceValidator),
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
		if workspaceErr := attachImageAgentWorkspace(composition.listingKitModule, imageAgentModule); workspaceErr != nil {
			return composition, workspaceErr
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
	timer.total("buildHTTPFeatureComposition")

	return composition, nil
}
