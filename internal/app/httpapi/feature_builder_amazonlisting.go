package httpapi

import (
	"context"

	"github.com/sirupsen/logrus"

	"task-processor/internal/amazonlisting"
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/product/catalog"
)

type amazonListingProductSnapshotReader func(context.Context, string, string) (catalog.ProductSnapshot, error)

func (r amazonListingProductSnapshotReader) GetProductSnapshot(ctx context.Context, query amazonlisting.ProductSnapshotQuery) (catalog.ProductSnapshot, error) {
	return r(ctx, query.TenantID, query.ProductKey)
}

type amazonListingFeatureBuilder struct {
	buildAmazonListing amazonListingModuleBuilder
}

func (b amazonListingFeatureBuilder) build(logger *logrus.Logger, deps *runtimeDeps) (*amazonlistinghttpapi.Module, error) {
	if deps == nil || deps.features == nil || deps.features.productSnapshotReader == nil {
		return nil, nil
	}
	approvedAssets := ensureApprovedAssetReader(logger, deps)
	if approvedAssets == nil {
		return nil, nil
	}
	productSnapshots := amazonListingProductSnapshotReader(func(ctx context.Context, tenantID, productKey string) (catalog.ProductSnapshot, error) {
		return readProductSnapshotForHTTPAPI(ctx, deps, tenantID, productKey)
	})
	amazonListingModule, err := b.buildAmazonListing(amazonlistinghttpapi.RuntimeBuildInput{
		Logger:                       logger,
		Config:                       deps.shared.cfg,
		ProductSnapshotReader:        productSnapshots,
		ApprovedAssetInventoryReader: approvedAssets,
	})
	if err != nil {
		return nil, err
	}
	deps.attachAmazonListingModule(amazonListingModule)
	return amazonListingModule, nil
}
