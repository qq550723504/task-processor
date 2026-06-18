package httpapi

import (
	"github.com/sirupsen/logrus"

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
