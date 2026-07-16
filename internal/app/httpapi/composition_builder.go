package httpapi

import (
	"github.com/sirupsen/logrus"

	a1688handoff "task-processor/internal/product/sourcehandoff/a1688"
	sourcea1688httpapi "task-processor/internal/productenrich/httpapi/sourcea1688"
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
	if composition.listingKitModule != nil && composition.listingKitModule.TaskLifecycleService != nil && composition.listingKitModule.StoreAccessValidator != nil {
		composition.productSourcingModule = sourcea1688httpapi.BuildModule(
			a1688handoff.NewTaskCommandService(composition.listingKitModule.TaskLifecycleService, composition.listingKitModule.StoreAccessValidator),
		)
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
