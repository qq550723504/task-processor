package httpapi

import (
	"context"
	"gorm.io/gorm"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/authz"
	officialstore "task-processor/internal/integration/persistence/listing/official"
	prepstore "task-processor/internal/integration/persistence/listing/preparation"
	recordstore "task-processor/internal/integration/persistence/listing/record"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/workbenchcontext"
	"time"
)

type nativeDraftReadCore struct {
	app         *supplyapp.Application
	collections *collection.Service
	repository  *prepstore.Repository
}

// Both serving modes consume the same native, live-authorized read owners.
func buildNativeDraftReadCore(ctx context.Context, db *gorm.DB, deps routeAuthDependencies, permissions *authz.ListingKitAuthorizer, constrain func(preparation.Authorizer) preparation.Authorizer) (nativeDraftReadCore, error) {
	var empty nativeDraftReadCore
	resolver, ok := deps.organizationResolver.(*workbenchcontext.Resolver)
	if !ok || resolver == nil || permissions == nil || db == nil {
		return empty, preparation.ErrUnavailable
	}
	collections, err := buildProductCollectionService(ctx, db, deps, permissions, true)
	if err != nil {
		return empty, err
	}
	live := &productReviewLiveOrganizationAccess{resolver: resolver, now: time.Now}
	baseAuth, err := preparation.NewContextAuthorizer(live, permissions)
	if err != nil {
		return empty, err
	}
	var auth preparation.Authorizer = baseAuth
	if constrain != nil {
		auth = constrain(auth)
	}
	repo, err := prepstore.NewRepository(ctx, db)
	if err != nil {
		return empty, err
	}
	preps, err := preparation.NewService(repo, collections, auth)
	if err != nil {
		return empty, err
	}
	snapshots, err := catalogstore.NewBoundedSnapshotReader(db, sourcing.MaxEncodedSnapshotBytes)
	if err != nil {
		return empty, err
	}
	sources, err := preparation.NewSourceSelector(preps, collections, repo, snapshots)
	if err != nil {
		return empty, err
	}
	reviews, err := buildProductReviewCore(db, resolver, permissions)
	if err != nil {
		return empty, err
	}
	records, err := recordstore.NewRepository(ctx, db)
	if err != nil {
		return empty, err
	}
	receipts, err := officialstore.NewOfficialRepository(ctx, db)
	if err != nil {
		return empty, err
	}
	app := &supplyapp.Application{Preparations: preps, Sources: sources, Records: records, Authorization: auth, Products: supplyapp.EffectiveProductReader{Reviews: reviews.store, Snapshots: snapshots}, StageProjection: supplyapp.ReviewProjection{Facts: repo, Reviews: reviews.store}, PublicationReceipts: receipts}
	return nativeDraftReadCore{app, collections, repo}, nil
}
