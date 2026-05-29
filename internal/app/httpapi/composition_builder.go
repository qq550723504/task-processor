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
	buildProduct       func(logger *logrus.Logger, deps *runtimeDeps) (*productenrichhttpapi.Module, error)
	buildImage         func(logger *logrus.Logger, deps *runtimeDeps) (*productimagehttpapi.Module, error)
	buildAmazonListing func(logger *logrus.Logger, deps *runtimeDeps) (*amazonlistinghttpapi.Module, error)
	buildSheinLogin    func(deps *runtimeDeps) (*sheinloginbootstrap.BuildResult, func() error, error)
	buildSDSLogin      func(deps *runtimeDeps) (*sdsloginbootstrap.BuildResult, func() error, error)
	buildListingKit    func(logger *logrus.Logger, deps *runtimeDeps) (*listingkithttpapi.Module, error)
	buildPrompt        func(store prompt.TenantPromptStore) *promptmgmtapi.BuildResult
	buildTaskRPC       func(provider taskrpcapi.ClientProvider, localStatusProvider taskrpcapi.LocalStatusProvider) (*taskrpcapi.BuildResult, error)
	buildSDS           func(logger *logrus.Logger, cfg *config.Config) *sdshttpapi.BuildResult
}

func newHTTPFeatureCompositionBuilder() httpFeatureCompositionBuilder {
	return httpFeatureCompositionBuilder{
		buildProduct:       buildProductModule,
		buildImage:         buildImageModule,
		buildAmazonListing: buildAmazonListingModule,
		buildSheinLogin:    buildSheinLoginModuleResult,
		buildSDSLogin:      buildSDSLoginModuleResult,
		buildListingKit:    buildListingKitModule,
		buildPrompt:        promptmgmtapi.BuildModule,
		buildTaskRPC:       taskrpcapi.BuildModule,
		buildSDS:           sdshttpapi.BuildModule,
	}
}

func (b httpFeatureCompositionBuilder) build(logger *logrus.Logger, deps *runtimeDeps) (httpFeatureComposition, error) {
	var composition httpFeatureComposition

	productModule, err := b.buildProduct(logger, deps)
	if err != nil {
		return composition, err
	}
	deps.attachProductModule(productModule)
	composition.productModule = productModule

	imageModule, err := b.buildImage(logger, deps)
	if err != nil {
		return composition, err
	}
	deps.attachImageModule(imageModule)
	composition.imageModule = imageModule

	amazonListingModule, err := b.buildAmazonListing(logger, deps)
	if err != nil {
		return composition, err
	}
	deps.attachAmazonListingModule(amazonListingModule)
	composition.amazonListingModule = amazonListingModule

	sheinLoginResult, sheinLoginCloser, err := b.buildSheinLogin(deps)
	if err != nil {
		return composition, err
	}
	deps.addClosers(sheinLoginCloser)
	composition.sheinLoginResult = sheinLoginResult

	sdsLoginResult, sdsLoginCloser, err := b.buildSDSLogin(deps)
	if err != nil {
		return composition, err
	}
	deps.addClosers(sdsLoginCloser)
	deps.attachSDSLoginResult(sdsLoginResult)
	composition.sdsLoginResult = sdsLoginResult

	listingKitModule, err := b.buildListingKit(logger, deps)
	if err != nil {
		return composition, err
	}
	deps.attachListingKitModule(listingKitModule)
	composition.listingKitModule = listingKitModule
	composition.promptModule = b.buildPrompt(deps.tenantPromptStore)

	taskRPCResult, err := b.buildTaskRPC(deps.managementClient(), composition.localTaskHealthProvider())
	if err != nil {
		return composition, err
	}
	composition.taskRPCResult = taskRPCResult
	composition.sdsModule = b.buildSDS(logger, deps.cfg)

	return composition, nil
}
