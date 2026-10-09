package dataservicesapp

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
}
