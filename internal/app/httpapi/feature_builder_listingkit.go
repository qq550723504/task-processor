package httpapi

import (
	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

type listingKitFeatureBuildOptions struct {
	includeImage      bool
	includeListingKit bool
	skipProduct       bool
}

type listingKitFeatureSet struct {
	productModule    *productModuleResult
	imageModule      *imageModuleResult
	listingKitModule *listingKitModuleResult
}

type listingKitFeatureBuilder struct {
	buildProduct    productModuleBuilder
	buildImage      imageModuleBuilder
	buildListingKit listingKitModuleBuilder
}

func newListingKitFeatureBuilder() listingKitFeatureBuilder {
	return listingKitFeatureBuilder{
		buildProduct:    buildProductModuleResult,
		buildImage:      buildImageModuleResult,
		buildListingKit: buildListingKitModuleResult,
	}
}

func (b listingKitFeatureBuilder) build(logger *logrus.Logger, deps *runtimeDeps, options listingKitFeatureBuildOptions) (listingKitFeatureSet, error) {
	var features listingKitFeatureSet

	if !options.skipProduct {
		productModule, err := b.buildProduct(productenrichhttpapi.RuntimeBuildInput{
			Logger:        logger,
			Config:        deps.shared.cfg,
			LLMManager:    deps.shared.llmMgr,
			InputParser:   deps.shared.inputParser,
			Understanding: deps.shared.understanding,
		})
		if err != nil {
			return features, err
		}
		deps.attachProductModule(productModule)
		features.productModule = productModule
	}

	if options.includeImage {
		imageModule, err := b.buildImage(productimagehttpapi.RuntimeBuildInput{
			Logger:        logger,
			Config:        deps.shared.cfg,
			LLMManager:    deps.shared.llmMgr,
			OpenAIManager: deps.shared.openaiMgr,
			InputParser:   deps.shared.inputParser,
			Understanding: deps.shared.understanding,
			ImageWorkDir:  deps.shared.imageWorkDir,
		})
		if err != nil {
			return features, err
		}
		deps.attachImageModule(imageModule)
		features.imageModule = imageModule
	}

	if options.includeListingKit {
		listingKitModule, err := b.buildListingKit(newListingKitRuntimeBuildInput(logger, deps))
		if err != nil {
			return features, err
		}
		deps.attachListingKitModule(listingKitModule)
		features.listingKitModule = listingKitModule
	}

	return features, nil
}

func newListingKitRuntimeBuildInput(logger *logrus.Logger, deps *runtimeDeps) listingkithttpapi.RuntimeBuildInput {
	return listingkithttpapi.RuntimeBuildInput{
		Logger: logger,
		Runtime: listingkithttpapi.RuntimeDependencies{
			Config:                     deps.shared.cfg,
			ProductService:             deps.features.productService,
			ImageService:               deps.features.imageService,
			ImageSubjectExtractor:      deps.features.imageSubjectExtractor,
			ImageWhiteBackgroundRender: deps.features.imageWhiteBgRenderer,
			ImageSceneRenderer:         deps.features.imageSceneRenderer,
			AICredentialStore:          deps.shared.aiCredentialStore,
			Support: listingkithttpapi.BuildRuntimeSupport(listingkithttpapi.RuntimeSupportInput{
				SheinCookieStore:          ensureListingKitSheinCookieStore(logger, deps),
				SDSSyncService:            buildSDSSyncService(logger, deps),
				SDSLoginStatusProvider:    deps.features.sdsLoginStatusProvider,
				SDSBaselineRemoteProvider: buildSDSBaselineRemoteProvider(logger, deps),
			}),
			ShouldStartTemporalWorkerInProcess: appruntime.ShouldStartListingKitSheinPublishTemporalWorkerInProcess(),
		},
	}
}
