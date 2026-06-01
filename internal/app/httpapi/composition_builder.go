package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	prompt "task-processor/internal/prompt"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
	sheinloginbootstrap "task-processor/internal/sheinlogin/bootstrap"
	"task-processor/internal/taskrpcapi"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

type httpFeatureCompositionBuilder struct {
	buildProduct       func(input productenrichhttpapi.RuntimeBuildInput) (*productenrichhttpapi.Module, error)
	buildImage         func(input productimagehttpapi.RuntimeBuildInput) (*productimagehttpapi.Module, error)
	buildAmazonListing func(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error)
	buildSheinLogin    func(deps *runtimeDeps) (*sheinloginbootstrap.BuildResult, func() error, error)
	buildSDSLogin      func(deps *runtimeDeps) (*sdsloginbootstrap.BuildResult, func() error, error)
	buildListingKit    func(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error)
	buildPrompt        func(store prompt.TenantPromptStore) *promptmgmtapi.BuildResult
	buildTaskRPC       func(provider taskrpcapi.ClientProvider, localStatusProvider taskrpcapi.LocalStatusProvider) (*taskrpcapi.BuildResult, error)
	buildSDS           func(logger *logrus.Logger, cfg *config.Config) *sdshttpapi.BuildResult
}

func newHTTPFeatureCompositionBuilder() httpFeatureCompositionBuilder {
	return httpFeatureCompositionBuilder{
		buildProduct:       productenrichhttpapi.BuildRuntimeModule,
		buildImage:         productimagehttpapi.BuildRuntimeModule,
		buildAmazonListing: amazonlistinghttpapi.BuildRuntimeModule,
		buildSheinLogin:    buildSheinLoginModuleResult,
		buildSDSLogin:      buildSDSLoginModuleResult,
		buildListingKit:    listingkithttpapi.BuildRuntimeModule,
		buildPrompt:        promptmgmtapi.BuildModule,
		buildTaskRPC:       taskrpcapi.BuildModule,
		buildSDS:           sdshttpapi.BuildModule,
	}
}

func (b httpFeatureCompositionBuilder) build(logger *logrus.Logger, deps *runtimeDeps) (httpFeatureComposition, error) {
	var composition httpFeatureComposition
	timer := newStartupTimer(logger)

	done := timer.phase("buildProductModule")
	productModule, err := b.buildProduct(productenrichhttpapi.RuntimeBuildInput{
		Logger:        logger,
		Config:        deps.shared.cfg,
		LLMManager:    deps.shared.llmMgr,
		InputParser:   deps.shared.inputParser,
		Understanding: deps.shared.understanding,
	})
	done()
	if err != nil {
		return composition, err
	}
	deps.attachProductModule(productModule)
	composition.productModule = productModule

	done = timer.phase("buildImageModule")
	imageModule, err := b.buildImage(productimagehttpapi.RuntimeBuildInput{
		Logger:        logger,
		Config:        deps.shared.cfg,
		LLMManager:    deps.shared.llmMgr,
		OpenAIManager: deps.shared.openaiMgr,
		InputParser:   deps.shared.inputParser,
		Understanding: deps.shared.understanding,
		ImageWorkDir:  deps.shared.imageWorkDir,
	})
	done()
	if err != nil {
		return composition, err
	}
	deps.attachImageModule(imageModule)
	composition.imageModule = imageModule

	done = timer.phase("buildAmazonListingModule")
	amazonListingModule, err := b.buildAmazonListing(amazonlistinghttpapi.RuntimeBuildInput{
		Logger:         logger,
		Config:         deps.shared.cfg,
		ProductService: deps.features.productService,
		ImageService:   deps.features.imageService,
	})
	done()
	if err != nil {
		return composition, err
	}
	deps.attachAmazonListingModule(amazonListingModule)
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
	listingKitModule, err := b.buildListingKit(newListingKitRuntimeBuildInput(logger, deps))
	done()
	if err != nil {
		return composition, err
	}
	deps.attachListingKitModule(listingKitModule)
	composition.listingKitModule = listingKitModule
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
