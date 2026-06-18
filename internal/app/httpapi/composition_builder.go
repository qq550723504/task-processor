package httpapi

import (
	"github.com/sirupsen/logrus"
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
	done = timer.phase("buildPromptModule")
	composition.promptModule = b.buildPrompt(deps.shared.tenantPromptStore)
	done()

	done = timer.phase("buildIntermediateRuntimeBundle")
	runtimeBundle, err := buildRuntimeBundleFromModules(deps.shared.cfg, composition.runtimeModules())
	done()
	if err != nil {
		return composition, err
	}

	done = timer.phase("buildTaskRPCModule")
	taskRPCResult, err := b.buildTaskRPC(deps.managementClient(), runtimeBundle.localTaskHealthProvider())
	done()
	if err != nil {
		return composition, err
	}
	composition.taskRPCResult = taskRPCResult
	done = timer.phase("buildSDSModule")
	composition.sdsModule = b.buildSDS(logger, deps.shared.cfg)
	done()
	timer.total("buildHTTPFeatureComposition")

	return composition, nil
}
