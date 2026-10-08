package productsourcing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/authz"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
)

func TestCollectionPublicationUsesOneActualSourceCatalogTransaction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("supply605"), tcpostgres.WithUsername("supply_owner"), tcpostgres.WithPassword("isolated-supply-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, InstallAcquisitionSchema(db))
	require.NoError(t, collectionstore.InstallSchema(db))
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	live := liveRolesFunc(func(context.Context, string, string) ([]string, error) { return []string{"listingkit_admin"}, nil })
	authority, err := collection.NewContextAuthorizer(live, permissions)
	require.NoError(t, err)
	repository, err := collectionstore.NewRepository(ctx, db, func(tx *gorm.DB) (collectionstore.OwnPublisher, error) { return NewOwnProductWriter(tx, authority) })
	require.NoError(t, err)
	snapshots, err := catalogstore.NewBoundedSnapshotReader(db, sourcing.MaxEncodedSnapshotBytes)
	require.NoError(t, err)
	captures, err := NewBrowserAcquisition(ctx, db, live, permissions, WithCollectionPublication())
	require.NoError(t, err)
	sources, err := NewPublishedAcquisitionReader(ctx, db, live, permissions)
	require.NoError(t, err)
	service, err := collection.NewService(repository, authority, sources)
	require.NoError(t, err)
	service.WithSnapshots(snapshots)
	actor := acquisitionIdentity("supply-org", "supply-actor")
	ownKey := uuid.NewString()
	own, err := service.Mutate(actor, ownKey, collection.Mutation{Action: "create_product", Product: &collection.OwnProduct{Title: "原始自有商品", Images: []string{"https://images.example.org/original.png"}}})
	require.NoError(t, err)
	detail, err := service.ReadItem(actor, own.ItemID)
	require.NoError(t, err)
	require.Equal(t, "原始自有商品", detail.Product.Title)
	var evidenceCount, versionCount int64
	require.NoError(t, db.Table("product_source_publications").Count(&evidenceCount).Error)
	require.NoError(t, db.Table("product_snapshot_versions").Count(&versionCount).Error)
	require.EqualValues(t, 1, evidenceCount)
	require.Equal(t, evidenceCount, versionCount)
	replay, err := service.Mutate(actor, ownKey, collection.Mutation{Action: "create_product", Product: &collection.OwnProduct{Title: "原始自有商品", Images: []string{"https://images.example.org/original.png"}}})
	require.NoError(t, err)
	require.Equal(t, own.ItemID, replay.ItemID)

	body, _, _ := browserApplicationFixture(t)
	capture, err := captures.Capture(actor, uuid.NewString(), body)
	require.NoError(t, err)
	scope, err := authority.Authorize(actor, collection.PermissionRead)
	require.NoError(t, err)
	batches, err := repository.ListBatches(actor, scope, collection.Query{Limit: 100})
	require.NoError(t, err)
	require.EqualValues(t, 2, batches.Total)
	var references int64
	require.NoError(t, db.Table("product_collection_items").Where("publication_id=?", capture.Publication.Receipt.PublicationID).Count(&references).Error)
	require.EqualValues(t, 1, references)

	// Fail the new reference write. Publication and Catalog must roll back with it.
	require.NoError(t, db.Exec("CREATE FUNCTION reject_collection() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture reference failure'; END $$").Error)
	require.NoError(t, db.Exec("CREATE TRIGGER reject_collection BEFORE INSERT ON product_collection_items FOR EACH ROW EXECUTE FUNCTION reject_collection()").Error)
	failingKey := uuid.NewString()
	failed, err := captures.Capture(actor, failingKey, body)
	require.Error(t, err)
	require.Nil(t, failed.Publication)
	op, err := captures.core.operations.ByKey(actor, sourcing.PublicationScope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID}, failingKey)
	require.NoError(t, err)
	var after int64
	require.NoError(t, db.Table("product_source_publications").Count(&after).Error)
	require.EqualValues(t, 2, after)
	require.NoError(t, db.Exec("DROP TRIGGER reject_collection ON product_collection_items").Error)
	_, err = captures.Verify(actor, failingKey, body)
	require.ErrorIs(t, err, sourcing.ErrAcquisitionUnknown, "verification remains read-only after a rolled-back write")
	verified, err := captures.ByKey(actor, failingKey)
	require.NoError(t, err)
	require.Equal(t, op.ID, verified.Operation.ID)
	require.NotNil(t, verified.Publication)
	require.NoError(t, db.Table("product_collection_items").Where("publication_id=?", verified.Publication.Receipt.PublicationID).Count(&references).Error)
	require.EqualValues(t, 1, references)
	_, err = service.ReadItem(acquisitionIdentity(scope.OrganizationID, "other-actor"), own.ItemID)
	require.ErrorIs(t, err, collection.ErrNotFound)
	// Source writer never installs a schema or repairs omitted collection tables.
	require.NoError(t, db.Exec("DROP TABLE product_collection_operations,product_collection_items,product_collection_batches").Error)
	_, err = captures.Capture(actor, uuid.NewString(), body)
	require.Error(t, err)
	require.False(t, errors.Is(err, catalog.ErrSnapshotNotReady))
}
