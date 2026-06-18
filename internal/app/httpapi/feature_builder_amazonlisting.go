package httpapi

import (
	"github.com/sirupsen/logrus"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
)

type amazonListingFeatureBuilder struct {
	buildAmazonListing func(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error)
}

func (b amazonListingFeatureBuilder) build(logger *logrus.Logger, deps *runtimeDeps) (*amazonlistinghttpapi.Module, error) {
	amazonListingModule, err := b.buildAmazonListing(amazonlistinghttpapi.RuntimeBuildInput{
		Logger:         logger,
		Config:         deps.shared.cfg,
		ProductService: deps.features.productService,
		ImageService:   deps.features.imageService,
	})
	if err != nil {
		return nil, err
	}
	deps.attachAmazonListingModule(amazonListingModule)
	return amazonListingModule, nil
}
