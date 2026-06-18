package httpapi

import (
	"github.com/sirupsen/logrus"

	listingkithttpapi "task-processor/internal/listingkit/httpapi"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

type httpFeatureCompositionBuilder struct {
	buildProduct       func(input productenrichhttpapi.RuntimeBuildInput) (*productenrichhttpapi.Module, error)
	buildImage         func(input productimagehttpapi.RuntimeBuildInput) (*productimagehttpapi.Module, error)
	buildAmazonListing func(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error)
	buildSheinLogin    sheinLoginModuleBuilder
	buildSDSLogin      sdsLoginModuleBuilder
	buildListingKit    func(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error)
	buildPrompt        promptModuleBuilder
	buildTaskRPC       taskRPCModuleBuilder
	buildSDS           sdsModuleBuilder
}

func newHTTPFeatureCompositionBuilder() httpFeatureCompositionBuilder {
	return httpFeatureCompositionBuilder{
		buildProduct:       productenrichhttpapi.BuildRuntimeModule,
		buildImage:         productimagehttpapi.BuildRuntimeModule,
		buildAmazonListing: amazonlistinghttpapi.BuildRuntimeModule,
		buildSheinLogin:    buildSheinLoginModuleResult,
		buildSDSLogin:      buildSDSLoginModuleResult,
		buildListingKit:    listingkithttpapi.BuildRuntimeModule,
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
	sheinLoginResult, sheinLoginCloser, err := b.buildSheinLogin(deps)
	done()
	if err != nil {
		return composition, err
	}
	deps.addClosers(sheinLoginCloser)
	composition.sheinLoginResult = sheinLoginResult

	done = timer.phase("buildSDSLoginModule")
	sdsLoginResult, sdsLoginCloser, err := b.buildSDSLogin(deps)
	done()
	if err != nil {
		return composition, err
	}
	deps.addClosers(sdsLoginCloser)
	deps.attachSDSLoginResult(sdsLoginResult)
	composition.sdsLoginResult = sdsLoginResult

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
