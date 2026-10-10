package collectionpersistence

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/collection"
)

func TestPostgresDataPublicationBatchAtomicityReplayAndActorIsolation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("data_collection621"), tcpostgres.WithUsername("test_owner"), tcpostgres.WithPassword("isolated-data621"), tcpostgres.BasicWaitStrategies())
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
	require.NoError(t, InstallSchema(db))
	scope := collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "grant"}
	job := uuid.NewString()
	batch, err := collection.NewPublicationBatch(scope, job, "amazon_data", "Amazon · us")
	require.NoError(t, err)
	for _, title := range []string{"fixture one", "fixture two"} {
		op := uuid.NewString()
		envelope, err := collection.OwnEnvelope(op, collection.OwnProduct{Title: title})
		require.NoError(t, err)
		require.NoError(t, db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			source, err := (testOwnPublisher{tx}).PublishOwn(ctx, scope, op, envelope)
			if err != nil {
				return err
			}
			source.Kind = "amazon_data"
			source.OperationID = op
			first, err := AppendDataPublication(ctx, tx, scope, batch, source, time.Now())
			if err != nil {
				return err
			}
			replay, err := AppendDataPublication(ctx, tx, scope, batch, source, time.Now())
			require.Equal(t, first, replay)
			return err
		}))
	}
	r, err := NewRepository(ctx, db, func(tx *gorm.DB) (OwnPublisher, error) { return testOwnPublisher{tx}, nil })
	require.NoError(t, err)
	items, err := r.ListItems(ctx, scope, batch.ID, collection.Query{Limit: 100})
	require.NoError(t, err)
	require.Len(t, items.Items, 2)
	for _, i := range items.Items {
		require.Equal(t, "amazon_data", i.Source.Kind)
	}
	foreign, err := r.ListItems(ctx, collection.Scope{OrganizationID: "org", ActorID: "other", MemberID: "other"}, "", collection.Query{Limit: 100})
	require.NoError(t, err)
	require.Empty(t, foreign.Items)
	failedOp := uuid.NewString()
	envelope, err := collection.OwnEnvelope(failedOp, collection.OwnProduct{Title: "rollback fixture"})
	require.NoError(t, err)
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		source, err := (testOwnPublisher{tx}).PublishOwn(ctx, scope, failedOp, envelope)
		if err != nil {
			return err
		}
		source.Kind = "amazon_data"
		source.OperationID = failedOp
		if _, err = AppendDataPublication(ctx, tx, scope, batch, source, time.Now()); err != nil {
			return err
		}
		return errors.New("injected Product transaction failure")
	})
	require.Error(t, err)
	var persisted int64
	require.NoError(t, db.Raw("SELECT count(*) FROM product_snapshot_versions WHERE tenant_id=? AND publication_id=?", scope.OrganizationID, failedOp).Scan(&persisted).Error)
	require.Zero(t, persisted)
	items, err = r.ListItems(ctx, scope, batch.ID, collection.Query{Limit: 100})
	require.NoError(t, err)
	require.Len(t, items.Items, 2)
	// An old kind constraint must fail readiness instead of enabling a route
	// that cannot save Amazon/custom source references. This is isolated test DDL.
	t.Run("supply eligibility follows active sources after moving and archiving", func(t *testing.T) {
		own, err := r.Execute(ctx, testCommand(scope, uuid.NewString(), collection.Mutation{Action: "create_product", Product: &collection.OwnProduct{Title: "own fixture"}}))
		require.NoError(t, err)
		check := func(id string, want bool) {
			t.Helper()
			page, err := r.ListBatches(ctx, scope, collection.Query{Limit: 100})
			require.NoError(t, err)
			found := false
			for _, b := range page.Items {
				if b.ID == id {
					found = true
					raw, err := json.Marshal(b)
					require.NoError(t, err)
					var fields map[string]any
					require.NoError(t, json.Unmarshal(raw, &fields))
					require.Equal(t, want, fields["supplyTransferSupported"])
				}
			}
			require.True(t, found)
			b, err := r.ReadBatch(ctx, scope, id)
			require.NoError(t, err)
			raw, err := json.Marshal(b)
			require.NoError(t, err)
			var fields map[string]any
			require.NoError(t, json.Unmarshal(raw, &fields))
			require.Equal(t, want, fields["supplyTransferSupported"])
		}
		check(own.BatchID, true)
		check(batch.ID, false)
		moved, err := r.Execute(ctx, testCommand(scope, uuid.NewString(), collection.Mutation{Action: "move_item", ItemID: items.Items[0].ID, TargetBatchID: own.BatchID, ExpectedRevision: items.Items[0].Revision}))
		require.NoError(t, err)
		check(own.BatchID, false)
		_, err = r.Execute(ctx, testCommand(scope, uuid.NewString(), collection.Mutation{Action: "archive_item", ItemID: items.Items[0].ID, ExpectedRevision: moved.Revision}))
		require.NoError(t, err)
		check(own.BatchID, true)
		check(batch.ID, false)
	})
	t.Run("original producer appends remain saved while archived batch stays hidden", func(t *testing.T) {
		owner := collection.Scope{OrganizationID: "org", ActorID: "archive-creator", MemberID: "archive-grant"}
		publication, err := collection.NewPublicationBatch(owner, uuid.NewString(), "amazon_data", "Amazon · us")
		require.NoError(t, err)
		appendProduct := func(op string) string {
			t.Helper()
			var itemID string
			err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				envelope, err := collection.OwnEnvelope(op, collection.OwnProduct{Title: "archive fixture"})
				if err != nil {
					return err
				}
				source, err := (testOwnPublisher{tx}).PublishOwn(ctx, owner, op, envelope)
				if err != nil {
					return err
				}
				source.Kind, source.OperationID = "amazon_data", op
				itemID, err = AppendDataPublication(ctx, tx, owner, publication, source, time.Now())
				return err
			})
			require.NoError(t, err)
			return itemID
		}
		appendProduct(uuid.NewString())
		visible, err := r.ReadBatch(ctx, owner, publication.ID)
		require.NoError(t, err)
		_, err = r.Execute(ctx, testCommand(owner, uuid.NewString(), collection.Mutation{Action: "archive_batch", BatchID: visible.ID, ExpectedRevision: visible.Revision}))
		require.NoError(t, err)
		var archivedAt time.Time
		require.NoError(t, db.Raw("SELECT archived_at FROM product_collection_batches WHERE organization_id=? AND actor_id=? AND id=?", owner.OrganizationID, owner.ActorID, publication.ID).Scan(&archivedAt).Error)
		second := uuid.NewString()
		require.Equal(t, appendProduct(second), appendProduct(second), "replay retains original item identity")
		var row struct {
			Count      int64
			ArchivedAt time.Time
		}
		require.NoError(t, db.Raw("SELECT b.archived_at,count(i.id) FROM product_collection_batches b JOIN product_collection_items i ON i.organization_id=b.organization_id AND i.actor_id=b.actor_id AND i.batch_id=b.id WHERE b.organization_id=? AND b.actor_id=? AND b.id=? GROUP BY b.archived_at", owner.OrganizationID, owner.ActorID, publication.ID).Scan(&row).Error)
		require.Equal(t, int64(2), row.Count)
		require.Equal(t, archivedAt, row.ArchivedAt, "producer must not restore an archived batch")
		_, err = r.ReadBatch(ctx, owner, publication.ID)
		require.ErrorIs(t, err, collection.ErrNotFound)
		hidden, err := r.ListItems(ctx, owner, "", collection.Query{Limit: 100})
		require.NoError(t, err)
		require.Empty(t, hidden.Items)
		_, err = r.Execute(ctx, testCommand(owner, uuid.NewString(), collection.Mutation{Action: "rename_batch", BatchID: publication.ID, ExpectedRevision: visible.Revision + 1, Name: "must stay archived"}))
		require.ErrorIs(t, err, collection.ErrNotFound, "new user mutations still reject archived batches")
	})
	require.NoError(t, db.Exec("ALTER TABLE product_collection_items DROP CONSTRAINT product_collection_items_source_kind_check; ALTER TABLE product_collection_items ADD CONSTRAINT product_collection_items_source_kind_check CHECK(source_kind IN ('acquisition','own')) NOT VALID").Error)
	require.ErrorIs(t, VerifySchema(ctx, db), collection.ErrUnavailable)
}
