package httpapi

import (
	"context"
	"gorm.io/gorm"
	imageapp "task-processor/internal/app/imageagent"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	collectionhttp "task-processor/internal/product/collection/httpapi"
	"time"
)

func buildProductCollectionModuleWithSourceMedia(ctx context.Context, db *gorm.DB, dependencies routeAuthDependencies, permissions *authz.ListingKitAuthorizer, cfg *config.Config, supply bool, media *collectionSourceMediaDependencies, images ...bool) (kernelmodule.Module, error) {
	if len(images) > 1 {
		return nil, collection.ErrUnavailable
	}
	service, err := buildProductCollectionService(ctx, db, dependencies, permissions, supply, len(images) == 1 && images[0])
	if err != nil {
		return nil, err
	}
	if cfg != nil && cfg.ProductCollectionSourceMedia.Enabled {
		if media == nil || media.Storage == nil {
			return nil, collection.ErrUnavailable
		}
		authority, err := collection.NewContextAuthorizer(&productReviewLiveOrganizationAccess{resolver: dependencies.organizationResolver, now: time.Now}, permissions)
		if err != nil {
			return nil, err
		}
		service.WithMedia(productsourcing.SourceMedia{Storage: media.Storage, Authorization: authority})
	}
	binder := productReviewCapabilityBinder{now: time.Now}
	return productCollectionModule{routes: collectionhttp.Routes(service, binder.Bind)}, nil
}

// Runtime constructs the current storage adapter; HTTP only consumes its port.
type collectionSourceMediaDependencies struct {
	Storage productsourcing.SourceMediaStorage
}

func WithCollectionSourceMedia(storage productsourcing.SourceMediaStorage) CurrentApplicationOption {
	return func(o *currentApplicationOptions) {
		o.collectionSourceMedias++
		o.collectionSourceMedia = &collectionSourceMediaDependencies{Storage: storage}
	}
}

func newImageSetManualMedia(media *collectionSourceMediaDependencies, authority imageMediaScopeAuthority) asset.ManualImageReader {
	return imageapp.ImageSetManualMedia{Media: productsourcing.SourceMedia{Storage: media.Storage, Authorization: authority}}
}
