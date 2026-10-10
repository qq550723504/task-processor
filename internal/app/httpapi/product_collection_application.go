package httpapi

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/product/collection"
	collectionhttp "task-processor/internal/product/collection/httpapi"
	"task-processor/internal/product/sourcing"
)

func buildProductCollectionService(ctx context.Context, db *gorm.DB, dependencies routeAuthDependencies, permissions *authz.ListingKitAuthorizer, supply ...bool) (*collection.Service, error) {
	if dependencies.organizationResolver == nil || permissions == nil {
		return nil, collection.ErrUnavailable
	}
	if len(supply) > 2 {
		return nil, collection.ErrUnavailable
	}
	if err := acquisitionstore.VerifyRuntimePermissions(ctx, db, acquisitionstore.RuntimeCapabilities{Collections: true, SupplyChain: len(supply) >= 1 && supply[0], ImageSets: len(supply) == 2 && supply[1]}); err != nil {
		return nil, err
	}
	live := &productReviewLiveOrganizationAccess{resolver: dependencies.organizationResolver, now: time.Now}
	authority, err := collection.NewContextAuthorizer(live, permissions)
	if err != nil {
		return nil, err
	}
	repository, err := collectionstore.NewRepository(ctx, db, func(tx *gorm.DB) (collectionstore.OwnPublisher, error) {
		return productsourcing.NewOwnProductWriter(tx, authority)
	})
	if err != nil {
		return nil, err
	}
	sources, err := productsourcing.NewPublishedAcquisitionReader(ctx, db, live, permissions)
	if err != nil {
		return nil, err
	}
	snapshots, err := catalogstore.NewBoundedSnapshotReader(db, sourcing.MaxEncodedSnapshotBytes)
	if err != nil {
		return nil, err
	}
	service, err := collection.NewService(repository, authority, sources)
	if err != nil {
		return nil, err
	}
	return service.WithSnapshots(snapshots), nil
}
func buildProductCollectionModule(ctx context.Context, db *gorm.DB, dependencies routeAuthDependencies, permissions *authz.ListingKitAuthorizer, supply ...bool) (kernelmodule.Module, error) {
	service, err := buildProductCollectionService(ctx, db, dependencies, permissions, supply...)
	if err != nil {
		return nil, err
	}
	binder := productReviewCapabilityBinder{now: time.Now}
	return productCollectionModule{routes: collectionhttp.Routes(service, binder.Bind)}, nil
}

type productCollectionModule struct{ routes []httproute.Descriptor }

func (productCollectionModule) Name() string { return "product-collection" }
func (productCollectionModule) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Workbench.Enabled
}
func (m productCollectionModule) Register(reg *kernelmodule.Registry) error {
	reg.AddRoutes(m.routes...)
	return nil
}
func validateCollectionDescriptor(route httproute.Descriptor) error {
	for _, expected := range collectionhttp.Routes(nil, nil) {
		if expected.Method == route.Method && expected.Path == route.Path {
			if route.Module != expected.Module || route.Permission != expected.Permission || route.AuthPolicy != expected.AuthPolicy || route.OrganizationAccessPolicy != expected.OrganizationAccessPolicy || route.OrganizationTargetResolver != nil || route.RequestTimeout != expected.RequestTimeout || route.Handler == nil {
				return errors.New("collection route loses live actor-private permission boundary")
			}
			return nil
		}
	}
	return errors.New("collection route is not admitted")
}
