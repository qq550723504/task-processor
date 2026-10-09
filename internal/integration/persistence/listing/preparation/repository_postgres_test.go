package preparationpersistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sync"
	"task-processor/internal/authidentity"
	officialstore "task-processor/internal/integration/persistence/listing/official"
	recordstore "task-processor/internal/integration/persistence/listing/record"
	submissionstore "task-processor/internal/integration/persistence/listing/submission"
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

type operationExecutionAuth struct{ denied bool }

func (a *operationExecutionAuth) AuthorizeExecution(context.Context, collection.Scope, string) error {
	if a.denied {
		return collection.ErrForbidden
	}
	return nil
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
		snapshotJSON := []byte(`{"title":"商品","variants":[{"sku":"SKU-SECOND-PAGE"}]}`)
		hash := sha256.Sum256(snapshotJSON)
		require.NoError(t, db.Create(&catalogstore.SnapshotVersionRecord{TenantID: scope.OrganizationID, ProductKey: product, Version: 1, PublicationID: publication, PayloadHash: hex.EncodeToString(hash[:]), SnapshotJSON: snapshotJSON}).Error)
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
	// A whole-batch action persists all fixed source references before Temporal.
	_, err = NewOperationRepository(ctx, db)
	require.ErrorIs(t, err, preparation.ErrUnavailable, "serving never installs omitted operation schema")
	require.NoError(t, InstallOperationSchema(db))
	operations, err := NewOperationRepository(ctx, db)
	require.NoError(t, err)
	require.NoError(t, recordstore.InstallSchema(db))
	require.NoError(t, submissionstore.InstallSchema(db))
	require.NoError(t, officialstore.InstallOfficialSchema(db))
	projected := []preparation.SourceStageFacts{}
	projectionAfter := ""
	for {
		facts, err := repository.ListStageFacts(ctx, scope, receipt.Preparation.ID, uuid.NewString(), collection.Digest("supplier"), projectionAfter)
		require.NoError(t, err)
		projected = append(projected, facts...)
		if len(facts) < 100 {
			break
		}
		projectionAfter = facts[len(facts)-1].SourceID
	}
	require.Len(t, projected, 205, "projection must traverse the complete batch")
	for _, f := range projected {
		require.Equal(t, "own", f.SourceKind)
		require.Equal(t, "SKU-SECOND-PAGE", f.SKUs)
	}
	opService, err := preparation.NewOperationService(service, collections, operations)
	require.NoError(t, err)
	opInput := preparation.OperationInput{PreparationID: receipt.Preparation.ID, ExpectedRevision: 1, StoreID: uuid.NewString(), Action: preparation.OperationAdapt}
	opKey := uuid.NewString()
	op, err := opService.Create(actor, opKey, opInput)
	require.NoError(t, err)
	require.EqualValues(t, 205, op.Operation.Count)
	retained, err := service.Read(actor, receipt.Preparation.ID)
	require.NoError(t, err)
	require.Equal(t, receipt.Preparation, retained)
	_, err = repository.Read(actor, collection.Scope{scope.OrganizationID, scope.ActorID, "rejoined"}, retained.ID)
	require.ErrorIs(t, err, preparation.ErrNotFound)
	secondOperation, err := opService.Create(actor, uuid.NewString(), opInput)
	require.NoError(t, err)
	otherStore := opInput
	otherStore.StoreID = uuid.NewString()
	otherOperation, err := opService.Create(actor, uuid.NewString(), otherStore)
	require.NoError(t, err)
	history, err := opService.List(actor, retained.ID, opInput.StoreID, collection.Query{Limit: 1})
	require.NoError(t, err)
	require.EqualValues(t, 2, history.Total)
	require.Len(t, history.Items, 1)
	require.Equal(t, secondOperation.Operation.ID, history.Items[0].ID)
	require.Equal(t, secondOperation.Operation.ID, history.NextCursor)
	older, err := opService.List(actor, retained.ID, opInput.StoreID, collection.Query{Limit: 1, After: history.NextCursor})
	require.NoError(t, err)
	require.Len(t, older.Items, 1)
	require.Equal(t, op.Operation.ID, older.Items[0].ID)
	require.Empty(t, older.NextCursor)
	_, err = opService.List(actor, retained.ID, opInput.StoreID, collection.Query{Limit: 1, After: otherOperation.Operation.ID})
	require.ErrorIs(t, err, preparation.ErrNotFound, "cursor cannot cross store boundaries")
	_, err = operations.ListOperations(actor, collection.Scope{scope.OrganizationID, "other-actor", "other-member"}, retained.ID, opInput.StoreID, collection.Query{Limit: 1})
	require.ErrorIs(t, err, preparation.ErrNotFound)
	opPage, err := opService.ListItems(actor, op.Operation.ID, collection.Query{Limit: 100})
	require.NoError(t, err)
	require.Len(t, opPage.Items, 100)
	require.EqualValues(t, 205, opPage.Total)
	opInput.SourceIDs = []string{opPage.Items[0].SourceID, uuid.NewString()}
	_, err = opService.Create(actor, uuid.NewString(), opInput)
	require.ErrorIs(t, err, preparation.ErrConflict)
	opInput.SourceIDs = nil
	replayedOperation, err := opService.Create(actor, opKey, opInput)
	require.NoError(t, err)
	require.True(t, replayedOperation.Replayed)
	_, err = operations.ReadOperation(actor, collection.Scope{scope.OrganizationID, scope.ActorID, "rejoined"}, op.Operation.ID)
	require.ErrorIs(t, err, preparation.ErrNotFound)
	access, err := opService.RequestAccess(actor, op.Operation.ID)
	require.NoError(t, err)
	claimed, err := operations.BeginOperationItem(actor, access, opPage.Items[0].SourceID)
	require.NoError(t, err)
	require.Equal(t, preparation.ItemRunning, claimed.Status)
	cancelled, err := opService.Cancel(actor, op.Operation.ID)
	require.NoError(t, err)
	require.Equal(t, preparation.OperationCancelled, cancelled.Status)
	stopped, err := operations.BeginOperationItem(actor, access, opPage.Items[1].SourceID)
	require.NoError(t, err, "cancel between list and claim returns the durable terminal result")
	require.Equal(t, preparation.ItemCancelled, stopped.Status)
	claimed.Status, claimed.Note = preparation.ItemSucceeded, "适配资料已保存"
	require.NoError(t, operations.FinishOperationItem(actor, access, claimed))
	afterCancel, err := opService.Read(actor, op.Operation.ID)
	require.NoError(t, err)
	require.EqualValues(t, 205, afterCancel.Completed)
	// Revocation terminalizes only execution progress. Existing UNKNOWN and
	// successful results, references and provider facts remain untouched.
	executionAuth := &operationExecutionAuth{}
	_, err = opService.WithExecution(executionAuth)
	require.NoError(t, err)
	workerProof, err := opService.AuthorizeExecution(ctx, scope.OrganizationID, secondOperation.Operation.ID)
	require.NoError(t, err)
	priorUnknown, err := operations.BeginOperationItem(ctx, workerProof, opPage.Items[0].SourceID)
	require.NoError(t, err)
	priorUnknown.Status, priorUnknown.ResultReference = preparation.ItemUnknown, uuid.NewString()
	require.NoError(t, operations.FinishOperationItem(ctx, workerProof, priorUnknown))
	running, err := operations.BeginOperationItem(ctx, workerProof, opPage.Items[1].SourceID)
	require.NoError(t, err)
	priorSuccess, err := operations.BeginOperationItem(ctx, workerProof, opPage.Items[3].SourceID)
	require.NoError(t, err)
	priorSuccess.Status, priorSuccess.ResultReference = preparation.ItemSucceeded, uuid.NewString()
	require.NoError(t, operations.FinishOperationItem(ctx, workerProof, priorSuccess))
	racingClaim, err := opService.RequestAccess(actor, secondOperation.Operation.ID)
	require.NoError(t, err)
	executionAuth.denied = true
	_, err = opService.StopExecution(actor, scope.OrganizationID, secondOperation.Operation.ID, "")
	require.ErrorIs(t, err, preparation.ErrForbidden)
	var concurrent sync.WaitGroup
	concurrentErrors := make(chan error, 2)
	concurrent.Add(2)
	go func() {
		defer concurrent.Done()
		_, e := operations.BeginOperationItem(actor, racingClaim, opPage.Items[4].SourceID)
		concurrentErrors <- e
	}()
	go func() {
		defer concurrent.Done()
		_, e := opService.StopExecution(ctx, scope.OrganizationID, secondOperation.Operation.ID, "")
		concurrentErrors <- e
	}()
	concurrent.Wait()
	close(concurrentErrors)
	for e := range concurrentErrors {
		require.NoError(t, e)
	}
	for range 2 {
		stoppedItem, stopErr := opService.StopExecution(ctx, scope.OrganizationID, secondOperation.Operation.ID, running.SourceID)
		require.NoError(t, stopErr)
		require.Equal(t, preparation.ItemUnknown, stoppedItem.Status)
		require.Empty(t, stoppedItem.ResultReference, "an operation command key is not a provider attempt ID")
	}
	deniedOperation, err := operations.ReadExecutionOperation(ctx, scope.OrganizationID, secondOperation.Operation.ID)
	require.NoError(t, err)
	require.Equal(t, preparation.OperationCompleted, deniedOperation.Status)
	require.EqualValues(t, 205, deniedOperation.Completed, "replayed stop never increments progress twice")
	deniedPage, err := operations.ListOperationItems(ctx, scope, secondOperation.Operation.ID, collection.Query{Limit: 100})
	require.NoError(t, err)
	require.Equal(t, priorUnknown, deniedPage.Items[0])
	require.Equal(t, preparation.ItemUnknown, deniedPage.Items[1].Status)
	require.Equal(t, preparation.ItemDenied, deniedPage.Items[2].Status)
	require.Equal(t, priorSuccess, deniedPage.Items[3])
	require.Contains(t, []string{preparation.ItemDenied, preparation.ItemUnknown}, deniedPage.Items[4].Status, "stop and claim serialize without a surviving runnable item")
	executionAuth.denied = false
	workerProof, err = opService.AuthorizeExecution(ctx, scope.OrganizationID, secondOperation.Operation.ID)
	require.NoError(t, err)
	lateClaim, err := operations.BeginOperationItem(ctx, workerProof, deniedPage.Items[2].SourceID)
	require.NoError(t, err)
	require.Equal(t, preparation.ItemDenied, lateClaim.Status, "restored grants do not restart the stopped batch")
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
	snapshots, err := catalogstore.NewBoundedSnapshotReader(db, 2<<20)
	require.NoError(t, err)
	selector, err := preparation.NewSourceSelector(service, collections, repository, snapshots)
	require.NoError(t, err)
	var archivedSource preparation.SourceItem
	for _, source := range append(append(page.Items, second.Items...), third.Items...) {
		if source.CollectionItemID == chosen[0] {
			archivedSource = source
		}
	}
	require.NotEmpty(t, archivedSource.ID)
	selected, err := selector.Select(actor, archivedSource.ID)
	require.NoError(t, err, "fixed source must survive later collection archive")
	selectedScope, source, snapshot, err := selected.Read(actor)
	require.NoError(t, err)
	require.Equal(t, scope, selectedScope)
	require.Equal(t, archivedSource, source)
	require.Equal(t, "商品", snapshot.Snapshot.Title)
	_, _, _, err = (preparation.AuthorizedSource{}).Read(actor)
	require.ErrorIs(t, err, preparation.ErrForbidden)
	_, err = repository.ReadRetainedSource(actor, collection.Scope{scope.OrganizationID, scope.ActorID, "rejoined-member"}, archivedSource.ID)
	require.ErrorIs(t, err, preparation.ErrNotFound)
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
	require.NoError(t, db.Exec("DROP TABLE listing_submission_official_receipts; DROP TABLE listing_submission_official_intents; DROP TABLE listing_target_record_commands; DROP TABLE listing_preparation_targets; DROP TABLE listing_target_records").Error)
	require.NoError(t, db.Exec("DROP TABLE listing_preparation_operation_items; DROP TABLE listing_preparation_operations").Error)
	require.NoError(t, db.Exec("DROP TABLE listing_preparation_sources").Error)
	_, err = NewRepository(ctx, db)
	require.ErrorIs(t, err, preparation.ErrUnavailable)
}
