package dataservicepersistence

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/dataservice"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
	"task-processor/internal/product/sourcing"
	"testing"
	"time"
)

type specialistFixture struct {
	scope         collection.Scope
	op            dataservice.Operator
	denied        bool
	calls, denyAt int
}

func (a *specialistFixture) Resolve(context.Context, string) (collection.Scope, error) {
	return a.scope, nil
}
func (a *specialistFixture) Check(context.Context, collection.Scope, string) error { return nil }
func (a *specialistFixture) Specialist(context.Context) (dataservice.Operator, error) {
	a.calls++
	if a.denied || (a.denyAt > 0 && a.calls >= a.denyAt) {
		return dataservice.Operator{}, dataservice.ErrForbidden
	}
	return a.op, nil
}
func (a *specialistFixture) CheckApplicant(_ context.Context, s collection.Scope) error {
	if s != a.scope {
		return dataservice.ErrForbidden
	}
	return nil
}
func TestPostgresCustomizationSpecDeliveryAtomicityAndScope(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("custom621"), tcpostgres.WithUsername("test_owner"), tcpostgres.WithPassword("isolated-data621"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Terminate(context.Background())) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, InstallSchema(db))
	require.NoError(t, InstallCustomSchema(db))
	require.NoError(t, catalogstore.AutoMigrate(db))
	require.NoError(t, collectionstore.InstallSchema(db))
	access := &specialistFixture{scope: collection.Scope{OrganizationID: "original-org", ActorID: "creator", MemberID: "original-member"}, op: dataservice.Operator{ID: "specialist"}}
	fail := false
	publisher := func(ctx context.Context, tx *gorm.DB, a dataservice.DeliveryAuthority, operation string, p collection.OwnProduct) (collection.Source, error) {
		envelope, err := collection.OwnEnvelope(operation, p)
		if err != nil {
			return collection.Source{}, err
		}
		snapshot, err := sourcing.ToSnapshot(envelope)
		if err != nil {
			return collection.Source{}, err
		}
		writer, err := catalogstore.NewTransactionWriter(tx)
		if err != nil {
			return collection.Source{}, err
		}
		out, err := writer.PublishSnapshot(ctx, catalog.PublishRequest{Identity: catalog.SnapshotIdentity{TenantID: a.Scope().OrganizationID, ProductKey: "custom-" + operation}, PublicationID: operation, Snapshot: snapshot})
		if err != nil {
			return collection.Source{}, err
		}
		if fail {
			return collection.Source{}, errors.New("injected custom publication failure")
		}
		return collection.Source{ProductKey: out.Identity.ProductKey, PublicationID: operation, Version: out.Version, OperationID: operation, Kind: "custom_dataset"}, nil
	}
	repo, err := NewCustomRepository(ctx, db, access, publisher)
	require.NoError(t, err)
	service, err := dataservice.NewCustomService(repo, access, access)
	require.NoError(t, err)
	cmd := uuid.NewString()
	input := dataservice.CustomInput{Name: "Amazon定制", Query: dataacquisition.Query{Site: "us", Mode: "keyword", Keyword: "bottle", Limit: 2}, Purpose: "选品", Format: "json"}
	request, err := service.Submit(ctx, cmd, input)
	require.NoError(t, err)
	replay, err := service.Submit(ctx, cmd, input)
	require.NoError(t, err)
	require.Equal(t, request.ID, replay.ID)
	changed := input
	changed.Name = "other"
	_, err = service.Submit(ctx, cmd, changed)
	require.ErrorIs(t, err, dataservice.ErrConflict)
	foreign := access.scope
	foreign.ActorID = "other"
	_, err = repo.Read(ctx, foreign, request.ID)
	require.ErrorIs(t, err, dataservice.ErrNotFound)
	changeCommand := uuid.NewString()
	access.calls, access.denyAt = 0, 4 // live denial only at post-commit response read
	_, err = service.Change(ctx, request.ID, changeCommand, 1, dataservice.CustomPatch{State: "EVALUATING", Note: "正在评估"})
	require.ErrorIs(t, err, dataservice.ErrUnknown)
	access.denyAt = 0
	request, err = service.Command(ctx, changeCommand)
	require.NoError(t, err)
	require.Equal(t, "EVALUATING", request.State)
	spec := dataservice.CustomSpec{Description: "2 rows", QuoteNote: "线下单独报价", ConfirmationNote: "已线下确认规格", MaximumRows: 2, Format: "json"}
	request, err = service.Change(ctx, request.ID, uuid.NewString(), request.Revision, dataservice.CustomPatch{State: "SPEC_CONFIRMED", Note: "确认规格", Spec: &spec})
	require.NoError(t, err)
	_, err = service.Change(ctx, request.ID, uuid.NewString(), 2, dataservice.CustomPatch{State: "PREPARING", Note: "stale spec"})
	require.ErrorIs(t, err, dataservice.ErrConflict)
	request, err = service.Change(ctx, request.ID, uuid.NewString(), request.Revision, dataservice.CustomPatch{State: "PREPARING", Note: "制作中"})
	require.NoError(t, err)
	rows := []collection.OwnProduct{{Title: "fixture declared one"}, {Title: "fixture declared two"}}
	delivery := uuid.NewString()
	fail = true
	_, err = service.Deliver(ctx, request.ID, delivery, request.Revision, request.SpecRevision, rows)
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM product_snapshot_versions").Scan(&count).Error)
	require.Zero(t, count)
	still, err := service.Read(ctx, request.ID)
	require.NoError(t, err)
	require.Equal(t, "PREPARING", still.State)
	require.Empty(t, still.BatchID)
	fail = false
	access.calls, access.denyAt = 0, 5 // mutation committed, response authorization revoked
	_, err = service.Deliver(ctx, request.ID, delivery, request.Revision, request.SpecRevision, rows)
	require.ErrorIs(t, err, dataservice.ErrUnknown)
	access.denyAt = 0
	delivered, err := service.Command(ctx, delivery)
	require.NoError(t, err)
	require.Equal(t, "DELIVERED", delivered.State)
	require.Equal(t, 2, delivered.DeliveredRows)
	require.NotEmpty(t, delivered.BatchID)
	again, err := service.Deliver(ctx, request.ID, delivery, request.Revision, request.SpecRevision, rows)
	require.NoError(t, err)
	require.Equal(t, delivered.BatchID, again.BatchID)
	rows[0].Title = "changed replay"
	_, err = service.Deliver(ctx, request.ID, delivery, request.Revision, request.SpecRevision, rows)
	require.ErrorIs(t, err, dataservice.ErrConflict)
	require.NoError(t, db.Raw("SELECT count(*) FROM product_collection_items WHERE organization_id=? AND actor_id=?", access.scope.OrganizationID, access.scope.ActorID).Scan(&count).Error)
	require.Equal(t, int64(2), count)
	require.NoError(t, db.Raw("SELECT count(*) FROM data_service_quota").Scan(&count).Error)
	require.Zero(t, count, "custom delivery does not consume realtime quota or Resource")
	require.Len(t, delivered.Events, 5)
	require.Equal(t, "specialist", delivered.Events[4].OperatorID)
	access.denied = true
	_, err = service.AdminRead(ctx, request.ID)
	require.ErrorIs(t, err, dataservice.ErrForbidden)
}
