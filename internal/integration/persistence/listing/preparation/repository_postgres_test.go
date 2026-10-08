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
	"task-processor/internal/product/sourcing"
	"testing"
	"time"
)

type fixedAuth struct{ scope collection.Scope }

func (a fixedAuth) Authorize(context.Context, string) (collection.Scope, error) { return a.scope, nil }

type unusedSources struct {
	sourcing.PublishedAcquisitionReader
}

func TestPostgresTransferCapturesAllPagesAndReplaysOriginalMembership(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("preparation605"), tcpostgres.WithUsername("preparation_owner"), tcpostgres.WithPassword("isolated-preparation-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, catalogstore.AutoMigrate(db))
	require.NoError(t, collectionstore.InstallSchema(db))
	require.NoError(t, InstallSchema(db))
	collectionRepo, err := collectionstore.NewRepository(ctx, db, func(*gorm.DB) (collectionstore.OwnPublisher, error) { return nil, collection.ErrUnavailable })
	require.NoError(t, err)
	scope := collection.Scope{"org-a", "actor-a", "member-a"}
	authority := fixedAuth{scope}
	collections, err := collection.NewService(collectionRepo, authority, unusedSources{})
	require.NoError(t, err)
	actor := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID, TokenExpiresAt: time.Now().Add(time.Minute)})
	batchID := uuid.NewString()
	require.NoError(t, db.Exec("INSERT INTO product_collection_batches(organization_id,actor_id,member_id,id,name,kind,revision,created_at) VALUES(?,?,?,?,?,'manual',1,now())", scope.OrganizationID, scope.ActorID, scope.MemberID, batchID, "待适配的完整批次").Error)
	var chosen []string
	// 205 immutable source references: a UI page of 100 must not truncate transfer.
	for index := 0; index < 205; index++ {
		itemID, publication, product := uuid.NewString(), uuid.NewString(), uuid.NewString()
		chosen = append(chosen, itemID)
		require.NoError(t, db.Create(&catalogstore.SnapshotVersionRecord{TenantID: scope.OrganizationID, ProductKey: product, Version: 1, PublicationID: publication, PayloadHash: collection.Digest(product), SnapshotJSON: []byte(`{"title":"商品"}`)}).Error)
		require.NoError(t, db.Exec("INSERT INTO product_collection_items(organization_id,actor_id,member_id,id,batch_id,product_key,publication_id,original_version,source_kind,source_operation_id,revision,created_at) VALUES(?,?,?,?,?,?,?,1,'own','',1,now())", scope.OrganizationID, scope.ActorID, scope.MemberID, itemID, batchID, product, publication).Error)
	}
	repository, err := NewRepository(ctx, db)
	require.NoError(t, err)
	service, err := preparation.NewService(repository, collections, authority)
	require.NoError(t, err)
	key := uuid.NewString()
	input := preparation.TransferInput{BatchID: batchID, ExpectedRevision: 1}
	receipt, err := service.Transfer(actor, key, input)
	require.NoError(t, err)
	require.EqualValues(t, 205, receipt.Preparation.Count)
	page, err := service.ListSources(actor, receipt.Preparation.ID, collection.Query{Limit: 100})
	require.NoError(t, err)
	require.Len(t, page.Items, 100)
	require.EqualValues(t, 205, page.Total)
	require.NotEmpty(t, page.NextCursor)
	second, err := service.ListSources(actor, receipt.Preparation.ID, collection.Query{Limit: 100, After: page.NextCursor})
	require.NoError(t, err)
	require.Len(t, second.Items, 100)
	third, err := service.ListSources(actor, receipt.Preparation.ID, collection.Query{Limit: 100, After: second.NextCursor})
	require.NoError(t, err)
	require.Len(t, third.Items, 5)
	subsetKey := uuid.NewString()
	subset := preparation.TransferInput{BatchID: batchID, ExpectedRevision: 1, ItemIDs: []string{chosen[0], chosen[204]}}
	subsetReceipt, err := service.Transfer(actor, subsetKey, subset)
	require.NoError(t, err)
	require.EqualValues(t, 2, subsetReceipt.Preparation.Count)
	subset.ItemIDs[0], subset.ItemIDs[1] = subset.ItemIDs[1], subset.ItemIDs[0]
	subsetReplay, err := service.Transfer(actor, subsetKey, subset)
	require.NoError(t, err)
	require.True(t, subsetReplay.Replayed, "item order is canonical and jsonb key ordering does not corrupt the command digest")
	missingKey := uuid.NewString()
	_, err = service.Transfer(actor, missingKey, preparation.TransferInput{BatchID: batchID, ExpectedRevision: 1, ItemIDs: []string{chosen[0], uuid.NewString()}})
	require.ErrorIs(t, err, collection.ErrConflict)
	_, err = service.ReadByKey(actor, missingKey)
	require.ErrorIs(t, err, preparation.ErrNotFound, "partial selection and its operation must roll back together")
	_, err = repository.Transfer(actor, preparation.TransferCommit{})
	require.ErrorIs(t, err, preparation.ErrForbidden)
	require.NoError(t, db.Exec("UPDATE product_collection_items SET archived_at=now(),revision=revision+1 WHERE organization_id=? AND actor_id=? AND id=?", scope.OrganizationID, scope.ActorID, chosen[0]).Error)
	require.NoError(t, db.Exec("UPDATE product_collection_batches SET name='后续改名',archived_at=now(),revision=revision+1 WHERE organization_id=? AND actor_id=? AND id=?", scope.OrganizationID, scope.ActorID, batchID).Error)
	replay, err := service.Transfer(actor, key, input)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, receipt.Preparation, replay.Preparation)
	changed := input
	changed.ExpectedRevision++
	_, err = service.Transfer(actor, key, changed)
	require.ErrorIs(t, err, preparation.ErrConflict)
	foreign := collection.Scope{scope.OrganizationID, "other-actor", "other-member"}
	_, err = repository.ReadByKey(actor, foreign, key)
	require.ErrorIs(t, err, preparation.ErrNotFound)
	_, err = repository.ListSources(actor, foreign, receipt.Preparation.ID, collection.Query{Limit: 100})
	require.ErrorIs(t, err, preparation.ErrNotFound)
	// Constructor is read-only: it does not repair an omitted required table.
	require.NoError(t, db.Exec("DROP TABLE listing_preparation_sources").Error)
	_, err = NewRepository(ctx, db)
	require.ErrorIs(t, err, preparation.ErrUnavailable)
}
