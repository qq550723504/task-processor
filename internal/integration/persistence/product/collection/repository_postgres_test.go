package collectionpersistence

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
)

type testOwnPublisher struct{ tx *gorm.DB }

func (p testOwnPublisher) PublishOwn(ctx context.Context, scope collection.Scope, operation string, envelope sourcing.SourceEnvelope) (collection.Source, error) {
	writer, err := catalogstore.NewTransactionWriter(p.tx)
	if err != nil {
		return collection.Source{}, err
	}
	publisher, err := catalog.NewPublisher(writer)
	if err != nil {
		return collection.Source{}, err
	}
	snapshot, err := sourcing.ToSnapshot(envelope)
	if err != nil {
		return collection.Source{}, err
	}
	key := "own-" + operation
	zero := uint64(0)
	published, err := publisher.Publish(ctx, catalog.PublishRequest{Identity: catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: key}, PublicationID: operation, ExpectedBaseVersion: &zero, Snapshot: snapshot})
	if err != nil {
		return collection.Source{}, err
	}
	return collection.Source{ProductKey: key, PublicationID: published.PublicationID, Version: published.Version, Kind: "own"}, nil
}
func testCommand(scope collection.Scope, key string, input collection.Mutation) collection.Command {
	command := collection.Command{Scope: scope, Key: key, OperationID: collection.StableID(scope.OrganizationID, scope.ActorID, key), InputHash: collection.Digest(input), Mutation: input}
	if input.Product != nil {
		envelope, _ := collection.OwnEnvelope(command.OperationID, *input.Product)
		command.Envelope = &envelope
	}
	for i, product := range input.Products {
		envelope, _ := collection.OwnEnvelope(collection.StableID(command.OperationID, "row", strconv.Itoa(i)), product)
		command.Envelopes = append(command.Envelopes, envelope)
	}
	return command
}

func TestPostgresCollectionAtomicReplayOwnershipMoveAndCommitUnknown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("collection605"), tcpostgres.WithUsername("collection_owner"), tcpostgres.WithPassword("isolated-collection-test"), tcpostgres.BasicWaitStrategies())
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
	require.NoError(t, InstallSchema(db))
	repository, err := NewRepository(ctx, db, func(tx *gorm.DB) (OwnPublisher, error) { return testOwnPublisher{tx}, nil })
	require.NoError(t, err)
	scope := collection.Scope{OrganizationID: "org-a", ActorID: "actor-a", MemberID: "member-a"}
	command := testCommand(scope, uuid.NewString(), collection.Mutation{Action: "create_product", Product: &collection.OwnProduct{Title: "原始标题"}})
	receipt, err := repository.Execute(ctx, command)
	require.NoError(t, err)
	original, err := repository.ReadItem(ctx, scope, receipt.ItemID)
	require.NoError(t, err)
	library, err := repository.ListItems(ctx, scope, "", collection.Query{Limit: 100, Keyword: "原始标题"})
	require.NoError(t, err)
	require.Len(t, library.Items, 1, "the own-product library searches its immutable source title")
	foreignLibrary, err := repository.ListItems(ctx, collection.Scope{"org-a", "other-actor", "other-member"}, "", collection.Query{Limit: 100})
	require.NoError(t, err)
	require.Empty(t, foreignLibrary.Items)
	replay, err := repository.Execute(ctx, command)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, receipt.ItemID, replay.ItemID)
	changed := command
	changed.InputHash = collection.Digest("different payload")
	_, err = repository.Execute(ctx, changed)
	require.ErrorIs(t, err, collection.ErrConflict)
	for _, other := range []collection.Scope{{OrganizationID: "org-b", ActorID: scope.ActorID, MemberID: scope.MemberID}, {OrganizationID: scope.OrganizationID, ActorID: "actor-b", MemberID: "member-b"}} {
		_, err := repository.ReadItem(ctx, other, receipt.ItemID)
		require.ErrorIs(t, err, collection.ErrNotFound)
		_, err = repository.ReadOperation(ctx, other, receipt.OperationID)
		require.ErrorIs(t, err, collection.ErrNotFound)
	}
	destination, err := repository.Execute(ctx, testCommand(scope, uuid.NewString(), collection.Mutation{Action: "create_batch", Name: "新批次"}))
	require.NoError(t, err)
	moved, err := repository.Execute(ctx, testCommand(scope, uuid.NewString(), collection.Mutation{Action: "move_item", ItemID: original.ID, TargetBatchID: destination.BatchID, ExpectedRevision: 1}))
	require.NoError(t, err)
	require.Equal(t, int64(2), moved.Revision)
	item, err := repository.ReadItem(ctx, scope, original.ID)
	require.NoError(t, err)
	require.Equal(t, original.Source, item.Source)
	require.Equal(t, destination.BatchID, item.BatchID)
	_, err = repository.Execute(ctx, testCommand(scope, uuid.NewString(), collection.Mutation{Action: "archive_item", ItemID: original.ID, ExpectedRevision: 1}))
	require.ErrorIs(t, err, collection.ErrConflict)
	_, err = repository.Execute(ctx, testCommand(scope, uuid.NewString(), collection.Mutation{Action: "archive_item", ItemID: original.ID, ExpectedRevision: 2}))
	require.NoError(t, err)
	_, err = repository.ReadItem(ctx, scope, original.ID)
	require.ErrorIs(t, err, collection.ErrNotFound)
	snapshots, err := catalogstore.NewBoundedSnapshotReader(db, collection.MaxPayloadBytes)
	require.NoError(t, err)
	snapshot, err := snapshots.GetSnapshot(ctx, catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: original.Source.ProductKey}, original.Source.Version)
	require.NoError(t, err)
	require.Equal(t, "原始标题", snapshot.Snapshot.Title)
	newCommand := testCommand(scope, uuid.NewString(), collection.Mutation{Action: "create_batch", Name: "响应丢失"})
	repository.afterCommit = func() error { return errors.New("lost acknowledgement") }
	_, err = repository.Execute(ctx, newCommand)
	require.ErrorIs(t, err, collection.ErrUnknown)
	repository.afterCommit = nil
	verified, err := repository.ReadOperation(ctx, scope, newCommand.OperationID)
	require.NoError(t, err)
	require.True(t, verified.Replayed)
	page, err := repository.ListBatches(ctx, scope, collection.Query{Limit: 100})
	require.NoError(t, err)
	require.Equal(t, int64(3), page.Total)
	_, err = repository.Execute(ctx, testCommand(scope, uuid.NewString(), collection.Mutation{Action: "rename_batch", BatchID: verified.BatchID, ExpectedRevision: verified.Revision, Name: "改名"}))
	require.NoError(t, err)
	// An import uses the same physical transaction for every publication and source.
	importCmd := testCommand(scope, uuid.NewString(), collection.Mutation{Action: "import_products", Name: "Excel完整批次", Products: []collection.OwnProduct{{Title: "导入一"}, {Title: "导入二"}}})
	failCalls := 0
	repository.own = func(tx *gorm.DB) (OwnPublisher, error) {
		return testFailOwnPublisher{base: testOwnPublisher{tx}, calls: &failCalls}, nil
	}
	_, err = repository.Execute(ctx, importCmd)
	require.Error(t, err)
	var persisted int64
	require.NoError(t, db.Raw("SELECT count(*) FROM product_snapshot_versions WHERE tenant_id=? AND publication_id IN (?,?)", scope.OrganizationID, importCmd.Envelopes[0].Identity.SourceID, importCmd.Envelopes[1].Identity.SourceID).Scan(&persisted).Error)
	require.Zero(t, persisted, "first-row publication must roll back with second-row failure")
	_, err = repository.ReadOperation(ctx, scope, importCmd.OperationID)
	require.ErrorIs(t, err, collection.ErrNotFound)
	repository.own = func(tx *gorm.DB) (OwnPublisher, error) { return testOwnPublisher{tx}, nil }
	repository.afterCommit = func() error { return errors.New("lost import response") }
	_, err = repository.Execute(ctx, importCmd)
	require.ErrorIs(t, err, collection.ErrUnknown)
	repository.afterCommit = nil
	imported, err := repository.Execute(ctx, importCmd)
	require.NoError(t, err)
	require.True(t, imported.Replayed)
	importedPage, err := repository.ListItems(ctx, scope, imported.BatchID, collection.Query{Limit: 100})
	require.NoError(t, err)
	require.Equal(t, int64(2), importedPage.Total)
	require.NotEqual(t, importedPage.Items[0].Source.PublicationID, importedPage.Items[1].Source.PublicationID)
	importCmd.InputHash = collection.Digest("changed spreadsheet")
	_, err = repository.Execute(ctx, importCmd)
	require.ErrorIs(t, err, collection.ErrConflict)
	t.Run("archive default library preserves old sources and allows new products", func(t *testing.T) {
		owner := collection.Scope{"org-a", "archive-actor", "archive-member"}
		create := func(title string) collection.Command {
			return testCommand(owner, uuid.NewString(), collection.Mutation{Action: "create_product", Product: &collection.OwnProduct{Title: title}})
		}
		firstCommand := create("归档前商品")
		first, err := repository.Execute(ctx, firstCommand)
		require.NoError(t, err)
		archive := func(id string) {
			batch, err := repository.ReadBatch(ctx, owner, id)
			require.NoError(t, err)
			_, err = repository.Execute(ctx, testCommand(owner, uuid.NewString(), collection.Mutation{Action: "archive_batch", BatchID: id, ExpectedRevision: batch.Revision}))
			require.NoError(t, err)
		}
		archive(first.BatchID)
		second, err := repository.Execute(ctx, create("归档后商品"))
		require.NoError(t, err)
		require.NotEqual(t, first.BatchID, second.BatchID)
		replay, err := repository.Execute(ctx, firstCommand)
		require.NoError(t, err)
		require.True(t, replay.Replayed)
		require.Equal(t, first.BatchID, replay.BatchID)
		_, err = repository.ReadBatch(ctx, owner, first.BatchID)
		require.ErrorIs(t, err, collection.ErrNotFound)
		batch, err := repository.ReadBatch(ctx, owner, second.BatchID)
		require.NoError(t, err)
		_, err = repository.Execute(ctx, testCommand(owner, uuid.NewString(), collection.Mutation{Action: "rename_batch", BatchID: batch.ID, ExpectedRevision: batch.Revision, Name: "保留更名"}))
		require.NoError(t, err)
		third, err := repository.Execute(ctx, create("仍在更名库"))
		require.NoError(t, err)
		require.Equal(t, second.BatchID, third.BatchID)
		archive(second.BatchID)
		imported, err := repository.Execute(ctx, testCommand(owner, uuid.NewString(), collection.Mutation{Action: "import_products", Name: "独立Excel批次", Products: []collection.OwnProduct{{Title: "Excel商品"}}}))
		require.NoError(t, err)
		type result struct {
			receipt collection.Receipt
			err     error
		}
		results := make(chan result, 4)
		for range 4 {
			command := create("并发新商品")
			go func() { receipt, err := repository.Execute(ctx, command); results <- result{receipt, err} }()
		}
		current := ""
		for range 4 {
			result := <-results
			require.NoError(t, result.err)
			require.NotEqual(t, imported.BatchID, result.receipt.BatchID)
			if current == "" {
				current = result.receipt.BatchID
			}
			require.Equal(t, current, result.receipt.BatchID)
		}
		own, err := repository.ListItems(ctx, owner, "", collection.Query{Limit: 100})
		require.NoError(t, err)
		require.Equal(t, int64(5), own.Total, "only the new default and independent Excel products remain visible")
		var old int64
		require.NoError(t, db.Raw("SELECT count(*) FROM product_collection_items WHERE organization_id=? AND actor_id=? AND batch_id IN (?,?)", owner.OrganizationID, owner.ActorID, first.BatchID, second.BatchID).Scan(&old).Error)
		require.Equal(t, int64(3), old, "archived sources are retained without restoration")
		_, err = repository.Execute(ctx, testCommand(owner, uuid.NewString(), collection.Mutation{Action: "create_product", BatchID: first.BatchID, Product: &collection.OwnProduct{Title: "明确选归档库"}}))
		require.ErrorIs(t, err, collection.ErrNotFound)
	})
	// A same-count FK aimed at the wrong ownership relation is not readiness.
	require.NoError(t, db.Exec("ALTER TABLE product_collection_items DROP CONSTRAINT product_collection_items_organization_id_actor_id_batch_id_fkey").Error)
	require.NoError(t, db.Exec("DELETE FROM product_collection_items").Error)
	require.NoError(t, db.Exec("ALTER TABLE product_collection_items ADD FOREIGN KEY(organization_id,actor_id,id) REFERENCES product_collection_batches(organization_id,actor_id,id)").Error)
	require.Error(t, VerifySchema(ctx, db))
}

type testFailOwnPublisher struct {
	base  testOwnPublisher
	calls *int
}

func (p testFailOwnPublisher) PublishOwn(ctx context.Context, scope collection.Scope, operation string, envelope sourcing.SourceEnvelope) (collection.Source, error) {
	*p.calls++
	if *p.calls == 2 {
		return collection.Source{}, errors.New("second row failed")
	}
	return p.base.PublishOwn(ctx, scope, operation, envelope)
}
