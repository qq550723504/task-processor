package httpapi

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/amazonlisting"
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/core/config"
	"task-processor/internal/product/catalog"
)

type amazonListingProductSnapshotReader func(context.Context, string, string, uint64) (catalog.PublishedSnapshot, error)

func (r amazonListingProductSnapshotReader) GetProductSnapshot(ctx context.Context, query amazonlisting.ProductSnapshotQuery) (catalog.ProductSnapshot, error) {
	published, err := r(ctx, query.TenantID, query.ProductKey, query.Version)
	if isProductSnapshotNotReadyForHTTPAPI(err) {
		return catalog.ProductSnapshot{}, amazonlisting.ErrProductSnapshotNotReady
	}
	if err != nil {
		return catalog.ProductSnapshot{}, err
	}
	return published.Snapshot, nil
}

func (r amazonListingProductSnapshotReader) GetPublishedProductSnapshot(ctx context.Context, query amazonlisting.ProductSnapshotQuery) (catalog.PublishedSnapshot, error) {
	published, err := r(ctx, query.TenantID, query.ProductKey, query.Version)
	if isProductSnapshotNotReadyForHTTPAPI(err) {
		return catalog.PublishedSnapshot{}, amazonlisting.ErrProductSnapshotNotReady
	}
	return published, err
}

type amazonListingFeatureBuilder struct {
	buildAmazonListing amazonListingModuleBuilder
	buildRepository    amazonListingRepositoryBuilder
}

func (b amazonListingFeatureBuilder) build(logger *logrus.Logger, deps *runtimeDeps) (*amazonlistinghttpapi.Module, error) {
	if deps == nil || deps.features == nil || deps.features.productSnapshotReader == nil {
		return nil, nil
	}
	approvedAssets := ensureApprovedAssetReader(logger, deps)
	if approvedAssets == nil {
		return nil, nil
	}
	productSnapshots := amazonListingProductSnapshotReader(func(ctx context.Context, tenantID, productKey string, version uint64) (catalog.PublishedSnapshot, error) {
		return readPublishedProductSnapshotForHTTPAPI(ctx, deps, tenantID, productKey, version)
	})
	if b.buildRepository == nil {
		return nil, fmt.Errorf("build amazon listing: task repository builder is required")
	}
	var database *config.DatabaseConfig
	if deps.shared != nil && deps.shared.cfg != nil {
		database = deps.shared.cfg.Database
	}
	repository, closer, err := b.buildRepository(database, logger)
	if err != nil {
		return nil, fmt.Errorf("build amazon listing task repository: %w", err)
	}
	amazonListingModule, err := b.buildAmazonListing(amazonlistinghttpapi.RuntimeBuildInput{
		Logger:                       logger,
		Config:                       deps.shared.cfg,
		ProductSnapshotReader:        productSnapshots,
		ApprovedAssetInventoryReader: approvedAssets,
		Repositories: amazonlistinghttpapi.RepositoryDependencies{
			Task: repository,
		},
	})
	if err != nil {
		if closer != nil {
			_ = closer()
		}
		return nil, err
	}
	if closer != nil {
		amazonListingModule.Closers = append(amazonListingModule.Closers, closer)
	}
	deps.attachAmazonListingModule(amazonListingModule)
	return amazonListingModule, nil
}
