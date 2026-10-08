package recordpersistence

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/authidentity"
	preparationstore "task-processor/internal/integration/persistence/listing/preparation"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/listing/record"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/storecenter"
)

type fixedAuth struct{ scope collection.Scope }

func (a fixedAuth) Authorize(context.Context, string) (collection.Scope, error) { return a.scope, nil }

type unusedSources struct {
	sourcing.PublishedAcquisitionReader
}
type missingAssets struct{}

type executionAuth struct{ scope collection.Scope }

func (a executionAuth) AuthorizeExecution(_ context.Context, scope collection.Scope, permission string) error {
	if scope != a.scope || permission != collection.PermissionRead && permission != preparation.PermissionRead && permission != preparation.PermissionManage {
		return collection.ErrForbidden
	}
	return nil
}

func (missingAssets) GetApprovedInventory(context.Context, asset.InventoryScope) (asset.ApprovedAssetInventory, error) {
	return asset.ApprovedAssetInventory{}, asset.ErrApprovedAssetsNotReady
}

type currentRules struct{}

func (currentRules) ReadTargetRules(_ context.Context, scope collection.Scope, storeID string, _ goods.OfficialDraftInput) (storecenter.ProductMerchantBinding, goods.OfficialRuleSnapshot, error) {
	return storecenter.ProductMerchantBinding{OrganizationID: scope.OrganizationID, StoreID: storeID, Site: "shein-us", StoreVersion: 1, ConnectionRevision: 1, ApplicationRevision: "app-v1", SupplierIdentityHash: collection.Digest("merchant-a"), ServiceExpiresAt: time.Now().Add(time.Hour)}, goods.OfficialRuleSnapshot{}, nil
}

func TestPostgresTargetImmutableRevisionsCommandReplayAndConcurrentCAS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("target605"), tcpostgres.WithUsername("target_owner"), tcpostgres.WithPassword("isolated-target-test"), tcpostgres.BasicWaitStrategies())
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
	require.NoError(t, preparationstore.InstallSchema(db))
	_, err = NewRepository(ctx, db)
	require.ErrorIs(t, err, record.ErrUnavailable, "serving construction never installs missing schema")
	require.NoError(t, InstallSchema(db))
	repository, err := NewRepository(ctx, db)
	require.NoError(t, err)
	scope := collection.Scope{OrganizationID: "org-a", ActorID: "actor-a", MemberID: "member-a"}
	authority := fixedAuth{scope}
	collectionsRepo, err := collectionstore.NewRepository(ctx, db, func(*gorm.DB) (collectionstore.OwnPublisher, error) { return nil, collection.ErrUnavailable })
	require.NoError(t, err)
	collections, err := collection.NewService(collectionsRepo, authority, unusedSources{})
	require.NoError(t, err)
	prepRepo, err := preparationstore.NewRepository(ctx, db)
	require.NoError(t, err)
	preparations, err := preparation.NewService(prepRepo, collections, authority)
	require.NoError(t, err)
	batchID, itemID, productKey, publication := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	snapshot := catalog.ProductSnapshot{Title: "Original"}
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NoError(t, db.Create(&catalogstore.SnapshotVersionRecord{TenantID: scope.OrganizationID, ProductKey: productKey, Version: 1, PublicationID: publication, PayloadHash: collection.Digest(snapshot), SnapshotJSON: raw}).Error)
	require.NoError(t, db.Exec("INSERT INTO product_collection_batches(organization_id,actor_id,member_id,id,name,kind,revision,created_at) VALUES(?,?,?,?,?,'manual',1,now())", scope.OrganizationID, scope.ActorID, scope.MemberID, batchID, "原始资料").Error)
	require.NoError(t, db.Exec("INSERT INTO product_collection_items(organization_id,actor_id,member_id,id,batch_id,product_key,publication_id,original_version,source_kind,source_operation_id,revision,created_at) VALUES(?,?,?,?,?,?,?,1,'own','',1,now())", scope.OrganizationID, scope.ActorID, scope.MemberID, itemID, batchID, productKey, publication).Error)
	actor := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID, TokenExpiresAt: time.Now().Add(time.Minute)})
	transfer, err := preparations.Transfer(actor, uuid.NewString(), preparation.TransferInput{BatchID: batchID, ExpectedRevision: 1})
	require.NoError(t, err)
	page, err := preparations.ListSources(actor, transfer.Preparation.ID, collection.Query{Limit: 100})
	require.NoError(t, err)
	snapshots, err := catalogstore.NewBoundedSnapshotReader(db, record.MaxPayloadBytes)
	require.NoError(t, err)
	selector, err := preparation.NewSourceSelector(preparations, collections, prepRepo, snapshots)
	require.NoError(t, err)
	execution := executionAuth{scope}
	selector, err = selector.WithExecution(execution, collection.ExecutionOwnerAuthority{Authorization: execution})
	require.NoError(t, err)
	service, err := record.NewTargetService(record.TargetDependencies{Sources: selector, ExecutionSources: selector, ExecutionAuthorization: execution, Products: record.OriginalTargetProduct{}, Assets: missingAssets{}, Rules: currentRules{}, Records: repository, Authorizer: authority})
	require.NoError(t, err)
	key := uuid.NewString()
	input := record.TargetInput{SourceID: page.Items[0].ID, StoreID: uuid.NewString(), EffectiveVersion: 1, Draft: goods.OfficialDraftInput{Product: model.PublishProduct{Names: []model.LanguageContent{{Language: "en", Name: "Manual title"}}}}}
	first, err := service.Create(actor, key, input)
	require.NoError(t, err)
	require.EqualValues(t, 1, first.Record.Revision)
	require.NotEmpty(t, first.Record.Result.Issues)
	replay, err := service.Create(actor, key, input)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, first.Record, replay.Record, "jsonb ordering and timestamp precision preserve the immutable receipt")
	changed := input
	changed.ExpectedRevision = 1
	_, err = service.Create(actor, key, changed)
	require.ErrorIs(t, err, record.ErrConflict)
	_, err = service.Create(actor, uuid.NewString(), input)
	require.ErrorIs(t, err, record.ErrConflict, "new command cannot overwrite revision 1 using expected revision 0")
	var outcomes [2]error
	var candidates [2]record.TargetReceipt
	var wait sync.WaitGroup
	for i := range outcomes {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			candidates[i], outcomes[i] = service.Create(actor, uuid.NewString(), changed)
		}(i)
	}
	wait.Wait()
	winner := 0
	if outcomes[0] != nil {
		winner = 1
	}
	require.NoError(t, outcomes[winner])
	require.ErrorIs(t, outcomes[1-winner], record.ErrConflict)
	require.EqualValues(t, 2, candidates[winner].Record.Revision)
	head, err := repository.ReadTargetHead(actor, scope, first.Record.TargetID)
	require.NoError(t, err)
	require.Equal(t, candidates[winner].Record, head)
	original, err := repository.ReadTargetCommand(actor, scope, key)
	require.NoError(t, err)
	require.Equal(t, first.Record, original.Record, "new target revision never mutates the original record")
	for _, foreign := range []collection.Scope{{"org-b", scope.ActorID, scope.MemberID}, {scope.OrganizationID, "actor-b", scope.MemberID}, {scope.OrganizationID, scope.ActorID, "rejoined-member"}} {
		_, err = repository.ReadTargetHead(actor, foreign, first.Record.TargetID)
		require.ErrorIs(t, err, record.ErrNotFound)
		_, err = repository.ReadTargetCommand(actor, foreign, key)
		require.ErrorIs(t, err, record.ErrNotFound)
	}
	_, err = repository.SaveTarget(actor, record.TargetPrepared{})
	require.ErrorIs(t, err, record.ErrForbidden)
	var count int64
	require.NoError(t, db.Table("listing_target_records").Count(&count).Error)
	require.EqualValues(t, 2, count, "losing CAS rolls back its record and command together")
	changed.ExpectedRevision = 2
	worker, err := service.CreateForExecution(ctx, scope, uuid.NewString(), changed)
	require.NoError(t, err, "worker commits use live execution proof without a fabricated authenticated identity")
	require.EqualValues(t, 3, worker.Record.Revision)
	_, authenticated := authidentity.AuthenticatedIdentityFromContext(ctx)
	require.False(t, authenticated)
	_, err = service.CreateForExecution(ctx, collection.Scope{scope.OrganizationID, scope.ActorID, "replacement-member"}, uuid.NewString(), changed)
	require.ErrorIs(t, err, record.ErrForbidden)
}
