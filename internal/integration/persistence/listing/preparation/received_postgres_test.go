package preparationpersistence

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/authidentity"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/collection"
	"testing"
	"time"
)

func TestPostgresTransferReceivedProductsRetainsKindsAndRejectsBlankTemplates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("received_supply"), tcpostgres.WithUsername("owner"), tcpostgres.WithPassword("isolated-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Terminate(context.Background())) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, catalogstore.AutoMigrate(db))
	require.NoError(t, collectionstore.InstallSchema(db))
	require.NoError(t, InstallSchema(db))
	scope := collection.Scope{"org", "actor", "member"}
	auth := fixedAuth{scope}
	collectionsRepo, err := collectionstore.NewRepository(ctx, db, func(*gorm.DB) (collectionstore.OwnPublisher, error) { return nil, collection.ErrUnavailable })
	require.NoError(t, err)
	collections, err := collection.NewService(collectionsRepo, auth, unusedSources{})
	require.NoError(t, err)
	repo, err := NewRepository(ctx, db)
	require.NoError(t, err)
	service, err := preparation.NewService(repo, collections, auth)
	require.NoError(t, err)
	actor := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID, TokenExpiresAt: time.Now().Add(time.Minute)})
	// Owner-only fixture tampering proves an old admission cannot silently serve.
	require.NoError(t, db.Exec(`ALTER TABLE listing_preparation_sources DROP CONSTRAINT listing_preparation_sources_source_kind_check;
 ALTER TABLE listing_preparation_sources ADD CONSTRAINT listing_preparation_sources_source_kind_check CHECK(source_kind IN ('acquisition','own'))`).Error)
	require.ErrorIs(t, VerifySchema(ctx, db), preparation.ErrUnavailable)
	require.NoError(t, db.Exec(`ALTER TABLE listing_preparation_sources DROP CONSTRAINT listing_preparation_sources_source_kind_check;
 ALTER TABLE listing_preparation_sources ADD CONSTRAINT listing_preparation_sources_source_kind_check CHECK(source_kind IN ('acquisition','own','market','sds_finished'))`).Error)
	require.NoError(t, VerifySchema(ctx, db))
	for _, kind := range []string{"market", "sds_finished", "sds_template"} {
		t.Run(kind, func(t *testing.T) {
			batch, item, product, publication, op, key := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
			require.NoError(t, db.Exec(`INSERT INTO product_collection_batches(organization_id,actor_id,member_id,id,name,kind,revision,created_at) VALUES(?,?,?,?,?,'manual',1,now())`, scope.OrganizationID, scope.ActorID, scope.MemberID, batch, "received").Error)
			require.NoError(t, db.Create(&catalogstore.SnapshotVersionRecord{TenantID: scope.OrganizationID, ProductKey: product, Version: 1, PublicationID: publication, PayloadHash: collection.Digest("source"), SnapshotJSON: []byte(`{"title":"received"}`)}).Error)
			require.NoError(t, db.Exec(`INSERT INTO product_collection_items(organization_id,actor_id,member_id,id,batch_id,product_key,publication_id,original_version,source_kind,source_operation_id,revision,created_at) VALUES(?,?,?,?,?,?,?,1,?,?,1,now())`, scope.OrganizationID, scope.ActorID, scope.MemberID, item, batch, product, publication, kind, op).Error)
			input := preparation.TransferInput{BatchID: batch, ExpectedRevision: 1}
			r, e := service.Transfer(actor, key, input)
			if kind == "sds_template" {
				require.ErrorIs(t, e, preparation.ErrInvalid)
				var n int64
				require.NoError(t, db.Table("listing_preparations").Where("command_key=?", key).Count(&n).Error)
				require.Zero(t, n)
				return
			}
			require.NoError(t, e)
			items, e := service.ListSources(actor, r.Preparation.ID, collection.Query{Limit: 10})
			require.NoError(t, e)
			require.Len(t, items.Items, 1)
			require.Equal(t, kind, items.Items[0].Source.Kind)
			require.Equal(t, op, items.Items[0].Source.OperationID)
			require.Equal(t, publication, items.Items[0].Source.PublicationID)
			replay, e := service.Transfer(actor, key, input)
			require.NoError(t, e)
			require.True(t, replay.Replayed)
			require.Equal(t, r.Preparation, replay.Preparation)
		})
	}
}
