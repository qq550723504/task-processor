package dataacquisitionpersistence

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
	"task-processor/internal/dataservice"
	keystore "task-processor/internal/integration/persistence/dataservice"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
	"task-processor/internal/product/sourcing"
)

type testAccess struct{ denied bool }

func (a *testAccess) CheckExecution(context.Context, dataacquisition.Principal, orgresource.ResourceFunding) error {
	if a.denied {
		return dataacquisition.ErrForbidden
	}
	return nil
}
func (a *testAccess) CheckRead(context.Context, dataacquisition.Principal) error { return nil }

func TestPostgresJobQuotaFencingPublicationAndOriginalChargeProof(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("jobs621"), tcpostgres.WithUsername("test_owner"), tcpostgres.WithPassword("isolated-data621"), tcpostgres.BasicWaitStrategies())
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
	require.NoError(t, collectionstore.InstallSchema(db))
	require.NoError(t, InstallSchema(db))
	access := &testAccess{}
	failPublication := false
	publish := func(ctx context.Context, tx *gorm.DB, job dataacquisition.Job, item dataacquisition.Item) (collection.Source, error) {
		envelope, err := item.Evidence.Envelope(item.ID)
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
		p, err := catalog.NewPublisher(writer)
		if err != nil {
			return collection.Source{}, err
		}
		zero := uint64(0)
		published, err := p.Publish(ctx, catalog.PublishRequest{Identity: catalog.SnapshotIdentity{TenantID: job.Scope.OrganizationID, ProductKey: "amazon-" + item.ID}, PublicationID: item.ID, ExpectedBaseVersion: &zero, Snapshot: snapshot})
		if err != nil {
			return collection.Source{}, err
		}
		if failPublication {
			return collection.Source{}, errors.New("injected after Catalog write")
		}
		return collection.Source{ProductKey: published.Identity.ProductKey, PublicationID: item.ID, Version: published.Version, OperationID: item.ID, Kind: "amazon_data"}, nil
	}
	repo, err := NewRepository(ctx, db, access, publish)
	require.NoError(t, err)
	scope := collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "original-grant"}
	keys, err := keystore.NewCredentialRepository(ctx, db)
	require.NoError(t, err)
	key := dataservice.Credential{ID: uuid.NewString(), Scope: scope, Input: dataservice.KeyInput{Name: "quota", ExpiresAt: time.Now().Add(time.Hour), DailyRows: 2, MonthlyCostFen: 10, Permissions: []string{dataservice.PermissionAcquire, dataservice.PermissionResult}}, Digest: collection.Digest("test-secret"), Suffix: "test", State: "ACTIVE", Revision: 1, CreatedAt: time.Now().UTC()}
	_, _, err = keys.Create(ctx, key, uuid.NewString(), collection.Digest(key.Input))
	require.NoError(t, err)
	principal := dataacquisition.Principal{Scope: scope, CredentialID: key.ID, CredentialRevision: 1}
	q, err := dataacquisition.NormalizeQuery(dataacquisition.Query{Site: "us", Mode: "asin", ASINs: []string{"B000123456", "B000654321"}, Limit: 2})
	require.NoError(t, err)
	command := uuid.NewString()
	job, err := repo.Admit(ctx, principal, command, q, orgresource.FundingEnterprise)
	require.NoError(t, err)
	replay, err := repo.Admit(ctx, principal, command, q, orgresource.FundingEnterprise)
	require.NoError(t, err)
	require.Equal(t, job.ID, replay.ID)
	changed := q
	changed.Limit = 1
	_, err = repo.Admit(ctx, principal, command, changed, orgresource.FundingEnterprise)
	require.ErrorIs(t, err, dataacquisition.ErrConflict)
	_, err = repo.Admit(ctx, principal, uuid.NewString(), changed, orgresource.FundingEnterprise)
	require.ErrorIs(t, err, dataacquisition.ErrConflict, "reserved rows prevent concurrent over-admission")
	other := scope
	other.ActorID = "other"
	_, err = repo.Read(ctx, other, job.ID)
	require.ErrorIs(t, err, dataacquisition.ErrNotFound)
	job, err = repo.Discover(ctx, job, []string{"B000123456", "B000654321", "B000123456"})
	require.NoError(t, err)
	items, err := repo.Items(ctx, job)
	require.NoError(t, err)
	require.Len(t, items, 2)
	item := items[0]
	intent, err := repo.ChargeIntent(ctx, orgresource.ConsumerChargeIdentity{OrganizationID: scope.OrganizationID, Consumer: orgresource.ConsumerAmazonData, OperationID: item.ID})
	require.NoError(t, err)
	charge := orgresource.ConsumerChargeReceipt{Intent: intent, ReservationID: uuid.NewString(), State: orgresource.ReservationReserved, CreatedAt: time.Now().UTC()}
	item, err = repo.BindReservation(ctx, job, item.ID, charge)
	require.NoError(t, err)
	claim, err := repo.Claim(ctx, job, item.ID)
	require.NoError(t, err)
	evidence := dataacquisition.Evidence{Site: "us", ASIN: item.ASIN, Title: "controlled fixture", MainImage: "https://m.media-amazon.com/images/I/fixture.jpg", Availability: "available", Price: 10, Currency: "USD", CapturedAt: time.Now().UTC().Format(time.RFC3339Nano), ParserVersion: "amazon-v1"}
	item, err = repo.PrepareEvidence(ctx, job, claim, evidence)
	require.NoError(t, err)
	failPublication = true
	_, err = repo.Publish(ctx, job, item)
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM product_snapshot_versions WHERE publication_id=?", item.ID).Scan(&count).Error)
	require.Zero(t, count, "Catalog/item/quota must roll back together")
	failPublication = false
	saved, err := repo.Publish(ctx, job, item)
	require.NoError(t, err)
	require.Equal(t, "SAVED", saved.State)
	require.NotNil(t, saved.Source)
	savedAgain, err := repo.Publish(ctx, job, item)
	require.NoError(t, err)
	require.Equal(t, *saved.Source, *savedAgain.Source)
	proof, err := repo.ChargeProof(ctx, charge)
	require.NoError(t, err)
	require.Equal(t, orgresource.ConsumerEffectSucceeded, proof.State)
	charge.State = orgresource.ReservationCommitted
	charge.OwnerEvidenceID = proof.EvidenceID
	require.NoError(t, repo.RecordCharge(ctx, job, item.ID, charge))
	second := items[1]
	claim2Intent, err := repo.ChargeIntent(ctx, orgresource.ConsumerChargeIdentity{OrganizationID: scope.OrganizationID, Consumer: orgresource.ConsumerAmazonData, OperationID: second.ID})
	require.NoError(t, err)
	secondCharge := orgresource.ConsumerChargeReceipt{Intent: claim2Intent, ReservationID: uuid.NewString(), State: orgresource.ReservationReserved, CreatedAt: time.Now().UTC()}
	_, err = repo.BindReservation(ctx, job, second.ID, secondCharge)
	require.NoError(t, err)
	stale, err := repo.Claim(ctx, job, second.ID)
	require.NoError(t, err)
	_, err = repo.Cancel(ctx, scope, job.ID, uuid.NewString())
	require.NoError(t, err)
	evidence.ASIN = second.ASIN
	_, err = repo.PrepareEvidence(ctx, job, stale, evidence)
	require.Error(t, err, "late evidence cannot become a publication")
	_, err = repo.Publish(ctx, job, stale)
	require.Error(t, err)
	secondProof, err := repo.ChargeProof(ctx, secondCharge)
	require.NoError(t, err)
	require.Equal(t, orgresource.ConsumerEffectFailed, secondProof.State)
	access.denied = true
	_, err = repo.ChargeIntent(ctx, secondCharge.Intent.Identity)
	require.ErrorIs(t, err, dataacquisition.ErrForbidden)
	// Terminal proof remains available for the original reservation after revocation.
	_, err = repo.ChargeProof(ctx, charge)
	require.NoError(t, err)
	restarted, err := NewRepository(ctx, db, access, publish)
	require.NoError(t, err)
	read, err := restarted.Read(ctx, scope, job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, read.Saved)
	require.Equal(t, 1, read.Failed)
	require.Equal(t, int64(5), read.ConfirmedFen)
	require.NotEmpty(t, read.BatchID)
	var usage struct{ ConsumedRows, ReservedRows int64 }
	require.NoError(t, db.Raw("SELECT consumed_rows,reserved_rows FROM data_service_quota WHERE key_id=? AND window_kind='day'", key.ID).Scan(&usage).Error)
	require.Equal(t, int64(1), usage.ConsumedRows)
	require.Zero(t, usage.ReservedRows)
	t.Run("stopped discovery is a failure rather than an empty success", func(t *testing.T) {
		access.denied = false
		console := dataacquisition.Principal{Scope: scope}
		empty, err := repo.Admit(ctx, console, uuid.NewString(), changed, orgresource.FundingMember)
		require.NoError(t, err)
		stopped, err := repo.FailDiscovery(ctx, empty, "provider_rejected")
		require.NoError(t, err)
		stopped, err = repo.Finish(ctx, stopped)
		require.NoError(t, err)
		require.Equal(t, "FAILED", stopped.State)
		require.Equal(t, "provider_rejected", stopped.Reason)
		require.Zero(t, stopped.Saved)
	})
}
