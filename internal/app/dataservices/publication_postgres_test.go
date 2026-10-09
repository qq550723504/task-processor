package dataservicesapp

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/dataservice"
	keystore "task-processor/internal/integration/persistence/dataservice"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	jobstore "task-processor/internal/integration/persistence/product/dataacquisition"
	sourcestore "task-processor/internal/integration/persistence/product/sourcing"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
	"task-processor/internal/product/sourcing"
	"testing"
	"time"
)

type authorizedFixture struct{}

func (authorizedFixture) Resolve(context.Context, string) (collection.Scope, error) {
	return collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "member"}, nil
}
func (authorizedFixture) Check(context.Context, collection.Scope, string) error { return nil }
func (authorizedFixture) Specialist(context.Context) (dataservice.Operator, error) {
	return dataservice.Operator{ID: "platform-specialist"}, nil
}
func (authorizedFixture) CheckApplicant(context.Context, collection.Scope) error { return nil }

func (authorizedFixture) CheckExecution(context.Context, dataacquisition.Principal, orgresource.ResourceFunding) error {
	return nil
}
func (authorizedFixture) CheckRead(context.Context, dataacquisition.Principal) error { return nil }
func TestAmazonPublicationUsesCurrentSourceCatalogCollectionTransaction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("publication621"), tcpostgres.WithUsername("test_owner"), tcpostgres.WithPassword("isolated-data621"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Terminate(context.Background())) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, keystore.InstallSchema(db))
	require.NoError(t, catalogstore.AutoMigrate(db))
	require.NoError(t, sourcestore.InstallSchema(db))
	require.NoError(t, collectionstore.InstallSchema(db))
	require.NoError(t, jobstore.InstallSchema(db))
	live := authorizedFixture{}
	publisher, err := NewProductPublisher(live)
	require.NoError(t, err)
	repo, err := jobstore.NewRepository(ctx, db, live, publisher)
	require.NoError(t, err)
	scope := collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "member"}
	q := dataacquisition.Query{Site: "jp", Mode: "asin", ASINs: []string{"B000123456"}, Limit: 1}
	job, err := repo.Admit(ctx, dataacquisition.Principal{Scope: scope}, uuid.NewString(), q, orgresource.FundingMember)
	require.NoError(t, err)
	job, err = repo.Discover(ctx, job, q.ASINs)
	require.NoError(t, err)
	items, err := repo.Items(ctx, job)
	require.NoError(t, err)
	require.Len(t, items, 1)
	item := items[0]
	intent, err := repo.ChargeIntent(ctx, orgresource.ConsumerChargeIdentity{OrganizationID: scope.OrganizationID, Consumer: orgresource.ConsumerAmazonData, OperationID: item.ID})
	require.NoError(t, err)
	item, err = repo.BindReservation(ctx, job, item.ID, orgresource.ConsumerChargeReceipt{Intent: intent, ReservationID: uuid.NewString(), State: orgresource.ReservationReserved})
	require.NoError(t, err)
	claimed, err := repo.Claim(ctx, job, item.ID)
	require.NoError(t, err)
	evidence := dataacquisition.Evidence{Site: "jp", ASIN: item.ASIN, Title: "実データではない制御fixture", MainImage: "https://m.media-amazon.com/images/I/fixture.jpg", Availability: "available", Price: 1000, Currency: "JPY", CapturedAt: time.Now().UTC().Format(time.RFC3339Nano), ParserVersion: "amazon-v1", Missing: []string{"rating"}}
	prepared, err := repo.PrepareEvidence(ctx, job, claimed, evidence)
	require.NoError(t, err)
	saved, err := repo.Publish(ctx, job, prepared)
	require.NoError(t, err)
	sourceReader, err := sourcestore.NewRepository(db, newCatalogBridge)
	require.NoError(t, err)
	persisted, err := sourceReader.Read(ctx, scope.OrganizationID, item.ID)
	require.NoError(t, err)
	require.Equal(t, saved.Source.Version, persisted.Receipt.CatalogVersion)
	require.Equal(t, "amazon_data", persisted.Receipt.Producer.Kind)
	require.Equal(t, "public_web", persisted.Envelope.Identity.SourceType)
	require.Equal(t, "amazon", persisted.Envelope.Identity.SourcePlatform)
	require.Equal(t, scope.ActorID, persisted.Receipt.ActorID)
	require.Equal(t, "rating", persisted.Envelope.Warnings[0].Field)
	require.NotEmpty(t, persisted.Receipt.EnvelopeHash)
	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM product_collection_items WHERE organization_id=? AND actor_id=? AND publication_id=? AND original_version=?", scope.OrganizationID, scope.ActorID, item.ID, saved.Source.Version).Scan(&count).Error)
	require.Equal(t, int64(1), count)
	// Verify the authoritative reader agrees with normalized captured facts.
	normalized, err := sourcing.ToSnapshot(persisted.Envelope)
	require.NoError(t, err)
	require.Equal(t, normalized, persisted.Snapshot)
	again, err := repo.Publish(ctx, job, prepared)
	require.NoError(t, err)
	require.Equal(t, saved.Source, again.Source)
	t.Run("specialist declaration preserves original applicant and spec", func(t *testing.T) {
		require.NoError(t, keystore.InstallCustomSchema(db))
		customPublisher, err := NewCustomProductPublisher()
		require.NoError(t, err)
		customRepo, err := keystore.NewCustomRepository(ctx, db, live, customPublisher)
		require.NoError(t, err)
		service, err := dataservice.NewCustomService(customRepo, live, live)
		require.NoError(t, err)
		request, err := service.Submit(ctx, uuid.NewString(), dataservice.CustomInput{Name: "定制fixture", Query: q, Purpose: "fixture only", Format: "json"})
		require.NoError(t, err)
		request, err = service.Change(ctx, request.ID, uuid.NewString(), request.Revision, dataservice.CustomPatch{State: "EVALUATING", Note: "fixture"})
		require.NoError(t, err)
		spec := dataservice.CustomSpec{Description: "declared row", QuoteNote: "线下", ConfirmationNote: "线下确认", MaximumRows: 1, Format: "json"}
		request, err = service.Change(ctx, request.ID, uuid.NewString(), request.Revision, dataservice.CustomPatch{State: "SPEC_CONFIRMED", Note: "confirmed", Spec: &spec})
		require.NoError(t, err)
		request, err = service.Change(ctx, request.ID, uuid.NewString(), request.Revision, dataservice.CustomPatch{State: "PREPARING", Note: "preparing"})
		require.NoError(t, err)
		_, err = service.Deliver(ctx, request.ID, uuid.NewString(), request.Revision, request.SpecRevision, []collection.OwnProduct{{Title: "declared fixture", Attributes: map[string]string{"asin": "B000123456"}}})
		require.NoError(t, err)
		var publication string
		require.NoError(t, db.Raw("SELECT publication_id FROM product_source_publications WHERE producer_kind='custom_dataset'").Scan(&publication).Error)
		declared, err := sourceReader.Read(ctx, scope.OrganizationID, publication)
		require.NoError(t, err)
		require.Equal(t, "user_input", declared.Envelope.Identity.SourceType)
		require.Equal(t, "custom_dataset", declared.Envelope.Identity.SourcePlatform)
		require.Equal(t, "specialist_declared", declared.Envelope.RawReference.ReferenceType)
		require.Equal(t, "creator", declared.Receipt.ActorID)
		require.Equal(t, "platform-specialist", declared.Envelope.ProductCandidate.Attributes["data.operator"])
		require.Equal(t, request.ID, declared.Envelope.ProductCandidate.Attributes["data.request"])
	})
}
