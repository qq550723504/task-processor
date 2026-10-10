package dataacquisitionpersistence

import (
	"context"
	"errors"
	"sync"
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

type testResultReader struct{}

func (testResultReader) Verify(context.Context, collection.Scope, collection.Source, dataacquisition.Evidence) error {
	return nil
}
func newTestResultReader(*gorm.DB) (dataacquisition.CapturedResultReader, error) {
	return testResultReader{}, nil
}

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
	repo, err := NewRepository(ctx, db, access, publish, newTestResultReader)
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
	// A command binds the effective key permission set, independently of mutable quotas.
	onePermission := key.Input
	onePermission.Permissions = []string{dataservice.PermissionAcquire}
	changedKey, err := keys.Change(ctx, scope, key.ID, uuid.NewString(), collection.Digest(onePermission), 1, dataservice.KeyPatch{State: "ACTIVE", Limits: &onePermission})
	require.NoError(t, err)
	_, err = repo.Admit(ctx, dataacquisition.Principal{Scope: scope, CredentialID: key.ID, CredentialRevision: changedKey.Revision}, command, q, orgresource.FundingEnterprise)
	require.ErrorIs(t, err, dataacquisition.ErrConflict, "permission changes cannot replay a different admission contract")
	_, err = keys.Change(ctx, scope, key.ID, uuid.NewString(), collection.Digest(key.Input), changedKey.Revision, dataservice.KeyPatch{State: "ACTIVE", Limits: &key.Input})
	require.NoError(t, err)
	replay, err = repo.Admit(ctx, principal, command, q, orgresource.FundingEnterprise)
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
	restarted, err := NewRepository(ctx, db, access, publish, newTestResultReader)
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
	quotas, err := repo.KeyQuotas(ctx, scope)
	require.NoError(t, err)
	require.Len(t, quotas, 1)
	require.Equal(t, int64(1), quotas[0].DayConsumedRows)
	require.Zero(t, quotas[0].DayReservedRows)
	require.Equal(t, int64(5), quotas[0].MonthConsumedFen)
	stats, err := repo.Usage(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.DayRows)
	require.Equal(t, int64(5), stats.MonthConfirmedFen)
	foreignStats, err := repo.Usage(ctx, other)
	require.NoError(t, err)
	require.Zero(t, foreignStats.DayRows)
	t.Run("concurrent admissions cannot over-reserve quota", func(t *testing.T) {
		access.denied = false
		concurrentScope := collection.Scope{OrganizationID: "quota-org", ActorID: "creator", MemberID: "member"}
		concurrentKey := key
		concurrentKey.ID = uuid.NewString()
		concurrentKey.Scope = concurrentScope
		concurrentKey.Input.DailyRows = 3
		concurrentKey.Input.MonthlyCostFen = 15
		_, _, err := keys.Create(ctx, concurrentKey, uuid.NewString(), collection.Digest(concurrentKey.Input))
		require.NoError(t, err)
		p := dataacquisition.Principal{Scope: concurrentScope, CredentialID: concurrentKey.ID, CredentialRevision: 1}
		one := changed
		one.Limit = 1
		var wg sync.WaitGroup
		out := make(chan error, 8)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, e := repo.Admit(ctx, p, uuid.NewString(), one, orgresource.FundingMember)
				out <- e
			}()
		}
		wg.Wait()
		close(out)
		admitted := 0
		for e := range out {
			if e == nil {
				admitted++
			} else {
				require.ErrorIs(t, e, dataacquisition.ErrConflict)
			}
		}
		require.Equal(t, 3, admitted)
		quota, err := repo.KeyQuotas(ctx, concurrentScope)
		require.NoError(t, err)
		require.Len(t, quota, 1)
		require.Equal(t, int64(3), quota[0].DayReservedRows)
		require.Equal(t, int64(15), quota[0].MonthReservedFen)
	})
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
	for _, reason := range []string{"provider_rejected", "provider_challenged", "provider_unsupported"} {
		t.Run("terminal provider reason requires current claim "+reason, func(t *testing.T) {
			access.denied = false
			original, err := repo.Admit(ctx, dataacquisition.Principal{Scope: scope}, uuid.NewString(), changed, orgresource.FundingMember)
			require.NoError(t, err)
			original, err = repo.Discover(ctx, original, []string{"B000123456"})
			require.NoError(t, err)
			rows, err := repo.Items(ctx, original)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			intent, err := repo.ChargeIntent(ctx, orgresource.ConsumerChargeIdentity{OrganizationID: scope.OrganizationID, Consumer: orgresource.ConsumerAmazonData, OperationID: rows[0].ID})
			require.NoError(t, err)
			reservation := orgresource.ConsumerChargeReceipt{Intent: intent, ReservationID: uuid.NewString(), State: orgresource.ReservationReserved, CreatedAt: time.Now().UTC()}
			_, err = repo.BindReservation(ctx, original, rows[0].ID, reservation)
			require.NoError(t, err)
			oldClaim, err := repo.Claim(ctx, original, rows[0].ID)
			require.NoError(t, err)
			require.NoError(t, db.Exec("UPDATE data_acquisition_items SET lease_until=now()-interval '1 second' WHERE id=?", rows[0].ID).Error)
			currentClaim, err := repo.Claim(ctx, original, rows[0].ID)
			require.NoError(t, err)
			_, err = repo.Fence(ctx, original, oldClaim, reason)
			require.ErrorIs(t, err, dataacquisition.ErrConflict, "a late response cannot fence the replacement claim")
			fenced, err := repo.Fence(ctx, original, currentClaim, reason)
			require.NoError(t, err)
			require.Equal(t, reason, fenced.Reason)
			require.Equal(t, "FAILED", fenced.State)
			lateEvidence := evidence
			lateEvidence.ASIN = oldClaim.ASIN
			_, err = repo.PrepareEvidence(ctx, original, oldClaim, lateEvidence)
			require.ErrorIs(t, err, dataacquisition.ErrConflict, "terminal provider failure cannot become a late publication")
			proof, err := repo.ChargeProof(ctx, reservation)
			require.NoError(t, err)
			require.Equal(t, orgresource.ConsumerEffectFailed, proof.State)
		})
	}
	t.Run("cancel after UTC rollover releases only original persisted windows", func(t *testing.T) {
		access.denied = false
		s := collection.Scope{OrganizationID: "rollover-org", ActorID: "creator", MemberID: "member"}
		k := key
		k.ID = uuid.NewString()
		k.Scope = s
		k.Input.DailyRows = 3
		k.Input.MonthlyCostFen = 15
		_, _, err := keys.Create(ctx, k, uuid.NewString(), collection.Digest(k.Input))
		require.NoError(t, err)
		p := dataacquisition.Principal{Scope: s, CredentialID: k.ID, CredentialRevision: 1}
		old, err := repo.Admit(ctx, p, uuid.NewString(), changed, orgresource.FundingMember)
		require.NoError(t, err)
		require.NoError(t, db.Exec("UPDATE data_service_quota SET window_start=CASE WHEN window_kind='day' THEN (date_trunc('day',now() AT TIME ZONE 'UTC')-interval '1 day')::date ELSE (date_trunc('month',now() AT TIME ZONE 'UTC')-interval '1 month')::date END WHERE key_id=?", k.ID).Error)
		require.NoError(t, db.Exec("UPDATE data_acquisition_jobs SET day_window=(date_trunc('day',now() AT TIME ZONE 'UTC')-interval '1 day')::date,month_window=(date_trunc('month',now() AT TIME ZONE 'UTC')-interval '1 month')::date,created_at=now()-interval '1 day',deadline=now()-interval '23 hours 30 minutes' WHERE id=?", old.ID).Error)
		fresh, err := repo.Admit(ctx, p, uuid.NewString(), changed, orgresource.FundingMember)
		require.NoError(t, err)
		require.NotEqual(t, old.ID, fresh.ID)
		_, err = repo.Cancel(ctx, s, old.ID, uuid.NewString())
		require.NoError(t, err)
		current, err := repo.KeyQuotas(ctx, s)
		require.NoError(t, err)
		require.Equal(t, int64(1), current[0].DayReservedRows)
		require.Equal(t, int64(5), current[0].MonthReservedFen)
		var remaining int64
		require.NoError(t, db.Raw("SELECT sum(reserved_rows) FROM data_service_quota WHERE key_id=? AND window_start<date_trunc(window_kind,now() AT TIME ZONE 'UTC')::date", k.ID).Scan(&remaining).Error)
		require.Zero(t, remaining)
	})
	t.Run("cancellation commands bind one target atomically", func(t *testing.T) {
		access.denied = false
		for _, concurrent := range []bool{false, true} {
			t.Run(map[bool]string{false: "sequential", true: "concurrent"}[concurrent], func(t *testing.T) {
				s := collection.Scope{OrganizationID: "cancel-" + uuid.NewString(), ActorID: "creator", MemberID: "original-grant"}
				k := key
				k.ID, k.Scope = uuid.NewString(), s
				k.Input.DailyRows, k.Input.MonthlyCostFen = 3, 15
				_, _, err := keys.Create(ctx, k, uuid.NewString(), collection.Digest(k.Input))
				require.NoError(t, err)
				p := dataacquisition.Principal{Scope: s, CredentialID: k.ID, CredentialRevision: 1}
				jobs := make([]dataacquisition.Job, 2)
				for index := range jobs {
					jobs[index], err = repo.Admit(ctx, p, uuid.NewString(), changed, orgresource.FundingEnterprise)
					require.NoError(t, err)
					jobs[index], err = repo.Discover(ctx, jobs[index], []string{"B000123456"})
					require.NoError(t, err)
				}
				command := uuid.NewString()
				outcomes := make([]error, 2)
				if concurrent {
					var wg sync.WaitGroup
					start := make(chan struct{})
					for index := range jobs {
						wg.Add(1)
						go func(index int) {
							defer wg.Done()
							<-start
							_, outcomes[index] = repo.Cancel(ctx, s, jobs[index].ID, command)
						}(index)
					}
					close(start)
					wg.Wait()
				} else {
					_, outcomes[0] = repo.Cancel(ctx, s, jobs[0].ID, command)
					_, outcomes[1] = repo.Cancel(ctx, s, jobs[1].ID, command)
				}
				winner := 0
				if outcomes[0] != nil {
					winner = 1
				}
				require.NoError(t, outcomes[winner])
				require.ErrorIs(t, outcomes[1-winner], dataacquisition.ErrConflict, "one cancellation command cannot fence another job")
				loser, err := repo.Read(ctx, s, jobs[1-winner].ID)
				require.NoError(t, err)
				require.Equal(t, "RUNNING", loser.State)
				items, err := repo.Items(ctx, loser)
				require.NoError(t, err)
				require.Len(t, items, 1)
				require.Equal(t, "PREPARED", items[0].State)
				quotas, err := repo.KeyQuotas(ctx, s)
				require.NoError(t, err)
				require.Len(t, quotas, 1)
				require.Equal(t, int64(1), quotas[0].DayReservedRows)
				require.Equal(t, int64(5), quotas[0].MonthReservedFen)
				pool.SetMaxOpenConns(1)
				t.Cleanup(func() { pool.SetMaxOpenConns(0) })
				restarted, err := NewRepository(ctx, db, access, publish, newTestResultReader)
				require.NoError(t, err)
				replayed, err := restarted.Cancel(ctx, s, jobs[winner].ID, command)
				require.NoError(t, err)
				require.Equal(t, "CANCELED", replayed.State)
				failedCommand := uuid.NewString()
				_, err = restarted.Cancel(ctx, s, uuid.NewString(), failedCommand)
				require.ErrorIs(t, err, dataacquisition.ErrNotFound)
				var count int64
				require.NoError(t, db.Raw("SELECT count(*) FROM data_service_commands WHERE organization_id=? AND actor_id=? AND command_key=?", s.OrganizationID, s.ActorID, failedCommand).Scan(&count).Error)
				require.Zero(t, count)
				// Inject only this command's receipt write failure in the disposable
				// Product database, after its cancellation callback has run.
				require.NoError(t, db.Exec("CREATE FUNCTION reject_cancel_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.command_key='"+failedCommand+"'::uuid THEN RAISE EXCEPTION 'injected cancellation receipt failure'; END IF; RETURN NEW; END $$").Error)
				require.NoError(t, db.Exec("CREATE TRIGGER reject_cancel_receipt BEFORE INSERT ON data_service_commands FOR EACH ROW EXECUTE FUNCTION reject_cancel_receipt()").Error)
				removeInjection := func() {
					require.NoError(t, db.Exec("DROP TRIGGER IF EXISTS reject_cancel_receipt ON data_service_commands").Error)
					require.NoError(t, db.Exec("DROP FUNCTION IF EXISTS reject_cancel_receipt()").Error)
				}
				t.Cleanup(removeInjection)
				_, err = restarted.Cancel(ctx, s, loser.ID, failedCommand)
				require.Error(t, err)
				require.NotErrorIs(t, err, dataacquisition.ErrUnknown, "known rollback is not a committed UNKNOWN")
				unchanged, err := repo.Read(ctx, s, loser.ID)
				require.NoError(t, err)
				require.Equal(t, "RUNNING", unchanged.State)
				items, err = repo.Items(ctx, unchanged)
				require.NoError(t, err)
				require.Equal(t, "PREPARED", items[0].State)
				quotas, err = repo.KeyQuotas(ctx, s)
				require.NoError(t, err)
				require.Equal(t, int64(1), quotas[0].DayReservedRows)
				require.Equal(t, int64(5), quotas[0].MonthReservedFen)
				require.NoError(t, db.Raw("SELECT count(*) FROM data_service_commands WHERE organization_id=? AND actor_id=? AND command_key=?", s.OrganizationID, s.ActorID, failedCommand).Scan(&count).Error)
				require.Zero(t, count)
				removeInjection()
				_, err = restarted.Cancel(ctx, s, loser.ID, failedCommand)
				require.NoError(t, err, "a rejected command must remain unbound")
				quotas, err = repo.KeyQuotas(ctx, s)
				require.NoError(t, err)
				require.Zero(t, quotas[0].DayReservedRows)
				require.Zero(t, quotas[0].MonthReservedFen)
				pool.SetMaxOpenConns(0)
			})
		}
	})
	t.Run("result read guard orders credential changes and releases failed reads", func(t *testing.T) {
		access.denied = false
		newKey := func() dataservice.Credential {
			k := key
			k.ID = uuid.NewString()
			k.Scope.ActorID = "guard-" + uuid.NewString()
			k.State, k.Revision = "ACTIVE", 1
			k.Input.ExpiresAt = time.Now().UTC().Truncate(time.Microsecond).Add(time.Hour)
			k.Input.Permissions = []string{dataservice.PermissionAcquire, dataservice.PermissionResult}
			stored, _, err := keys.Create(ctx, k, uuid.NewString(), collection.Digest(k.Input))
			require.NoError(t, err)
			return stored
		}
		k := newKey()
		p := dataacquisition.Principal{Scope: k.Scope, CredentialID: k.ID, CredentialRevision: 1}
		original, err := repo.Admit(ctx, p, uuid.NewString(), q, orgresource.FundingMember)
		require.NoError(t, err)
		entered, release := make(chan struct{}), make(chan struct{})
		var releaseOnce sync.Once
		defer releaseOnce.Do(func() { close(release) })
		readDone := make(chan error, 1)
		go func() {
			readDone <- repo.WithResultRead(ctx, p, func(ctx context.Context, reader dataacquisition.ResultReadRepository, _ dataacquisition.CapturedResultReader) error {
				job, err := reader.Read(ctx, k.Scope, original.ID)
				if err != nil || job.ID != original.ID {
					return dataacquisition.ErrUnavailable
				}
				close(entered)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-release:
					return nil
				}
			})
		}()
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		changeDone := make(chan error, 1)
		go func() {
			_, err := keys.Change(ctx, k.Scope, k.ID, uuid.NewString(), collection.Digest("revoke read guard"), 1, dataservice.KeyPatch{State: "REVOKED"})
			changeDone <- err
		}()
		require.Eventually(t, func() bool {
			var waiting int64
			result := db.Raw("SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%data_service_credentials%' AND query LIKE '%FOR UPDATE%'").Scan(&waiting)
			return result.Error == nil && waiting > 0
		}, time.Second, 10*time.Millisecond, "revoke must wait on the result read's real PostgreSQL row lock")
		select {
		case err := <-changeDone:
			t.Fatalf("credential change completed before result read: %v", err)
		default:
		}
		releaseOnce.Do(func() { close(release) })
		require.NoError(t, <-readDone)
		require.NoError(t, <-changeDone)
		called := false
		err = repo.WithResultRead(ctx, p, func(context.Context, dataacquisition.ResultReadRepository, dataacquisition.CapturedResultReader) error {
			called = true
			return nil
		})
		require.ErrorIs(t, err, dataacquisition.ErrForbidden)
		require.False(t, called, "revocation completed before this read")

		for _, scenario := range []string{"disabled", "stale revision", "result removed", "foreign member"} {
			t.Run(scenario, func(t *testing.T) {
				k := newKey()
				p := dataacquisition.Principal{Scope: k.Scope, CredentialID: k.ID, CredentialRevision: 1}
				patch := dataservice.KeyPatch{State: "ACTIVE"}
				if scenario == "disabled" {
					patch.State = "DISABLED"
				}
				if scenario == "result removed" {
					limits := k.Input
					limits.Permissions = []string{dataservice.PermissionAcquire}
					patch.Limits = &limits
				}
				if scenario == "foreign member" {
					p.Scope.MemberID = "different-grant"
				} else {
					changed, err := keys.Change(ctx, k.Scope, k.ID, uuid.NewString(), collection.Digest(patch), 1, patch)
					require.NoError(t, err)
					if scenario != "stale revision" {
						p.CredentialRevision = changed.Revision
					}
				}
				called := false
				err := repo.WithResultRead(ctx, p, func(context.Context, dataacquisition.ResultReadRepository, dataacquisition.CapturedResultReader) error {
					called = true
					return nil
				})
				require.ErrorIs(t, err, dataacquisition.ErrForbidden)
				require.False(t, called)
			})
		}
		t.Run("expiry during result materialization rejects output", func(t *testing.T) {
			k := newKey()
			limits := k.Input
			limits.ExpiresAt = time.Now().UTC().Truncate(time.Microsecond).Add(250 * time.Millisecond)
			changed, err := keys.Change(ctx, k.Scope, k.ID, uuid.NewString(), collection.Digest(limits), 1, dataservice.KeyPatch{State: "ACTIVE", Limits: &limits})
			require.NoError(t, err)
			p := dataacquisition.Principal{Scope: k.Scope, CredentialID: k.ID, CredentialRevision: changed.Revision}
			err = repo.WithResultRead(ctx, p, func(ctx context.Context, _ dataacquisition.ResultReadRepository, _ dataacquisition.CapturedResultReader) error {
				timer := time.NewTimer(time.Until(limits.ExpiresAt.Add(time.Millisecond)))
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-timer.C:
					return nil
				}
			})
			require.ErrorIs(t, err, dataacquisition.ErrForbidden)
		})
		t.Run("canceled read releases its credential lock", func(t *testing.T) {
			k := newKey()
			p := dataacquisition.Principal{Scope: k.Scope, CredentialID: k.ID, CredentialRevision: 1}
			readCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			err := repo.WithResultRead(readCtx, p, func(ctx context.Context, _ dataacquisition.ResultReadRepository, _ dataacquisition.CapturedResultReader) error {
				cancel()
				<-ctx.Done()
				return ctx.Err()
			})
			require.ErrorIs(t, err, context.Canceled)
			_, err = keys.Change(ctx, k.Scope, k.ID, uuid.NewString(), collection.Digest("after canceled read"), 1, dataservice.KeyPatch{State: "REVOKED"})
			require.NoError(t, err, "no surviving row lock after the read fails")
		})
	})
}
