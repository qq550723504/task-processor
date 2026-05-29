package httpapi

import (
	"github.com/sirupsen/logrus"

	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

type listingKitFeatureBuildOptions struct {
	includeImage      bool
	includeListingKit bool
}

type listingKitFeatureSet struct {
	productModule    *productenrichhttpapi.Module
	imageModule      *productimagehttpapi.Module
	listingKitModule *listingkithttpapi.Module
}

type listingKitFeatureBuilder struct {
	buildProduct    func(logger *logrus.Logger, deps *runtimeDeps) (*productenrichhttpapi.Module, error)
	buildImage      func(logger *logrus.Logger, deps *runtimeDeps) (*productimagehttpapi.Module, error)
	buildListingKit func(logger *logrus.Logger, deps *runtimeDeps) (*listingkithttpapi.Module, error)
}

func newListingKitFeatureBuilder() listingKitFeatureBuilder {
	return listingKitFeatureBuilder{
		buildProduct:    buildProductModule,
		buildImage:      buildImageModule,
		buildListingKit: buildListingKitModule,
	}
}

func (b listingKitFeatureBuilder) build(logger *logrus.Logger, deps *runtimeDeps, options listingKitFeatureBuildOptions) (listingKitFeatureSet, error) {
	var features listingKitFeatureSet

	productModule, err := b.buildProduct(logger, deps)
	if err != nil {
		return features, err
	}
	deps.attachProductModule(productModule)
	features.productModule = productModule

	if options.includeImage {
		imageModule, err := b.buildImage(logger, deps)
		if err != nil {
			return features, err
		}
		deps.attachImageModule(imageModule)
		features.imageModule = imageModule
	}

	if options.includeListingKit {
		listingKitModule, err := b.buildListingKit(logger, deps)
		if err != nil {
			return features, err
		}
		deps.attachListingKitModule(listingKitModule)
		features.listingKitModule = listingKitModule
	}

	return features, nil
}
