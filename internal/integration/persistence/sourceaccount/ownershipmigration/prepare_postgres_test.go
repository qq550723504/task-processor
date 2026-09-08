//go:build integration

package ownershipmigration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPreparedOwnershipPostgresTransactionKernel(t *testing.T) {
	fixture := newPreparePostgres(t)

	t.Run("preserves accounts and replays one durable result", func(t *testing.T) {
		request := fixture.resetAndSeed(t)
		before := fixture.tableJSON(t, "public.source_account", "id")
		preparer, err := NewPreparer(fixture.db)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err = InstallPreparedSchema(ctx, fixture.db); err != nil {
			t.Fatalf("repeat InstallPreparedSchema() error = %v", err)
		}
		result, replayed, err := preparer.Prepare(ctx, request)
		if err != nil {
			t.Fatalf("Prepare() error = %v", err)
		}
		if replayed {
			t.Fatal("first Prepare() reported replay")
		}
		if result.Stage != PreparedStage || result.AccountCount != 3 || len(result.Accounts) != 3 {
			t.Fatalf("result = %#v", result)
		}
		if result.Accounts[0].OrganizationID != "org-a" || result.Accounts[0].ProfileRef != "profile-a" || result.Accounts[0].ProfileDirectory != prepareTestProfileDirectory("101", "1") {
			t.Fatalf("enabled target = %#v", result.Accounts[0])
		}
		if result.Accounts[0].Label == nil || *result.Accounts[0].Label != "enabled" || result.Accounts[0].ProxyRef == nil || *result.Accounts[0].ProxyRef != "proxy-a" || result.Accounts[0].LoginURL == nil || result.Accounts[0].LastVerifiedAt == nil {
			t.Fatalf("enabled metadata was not preserved = %#v", result.Accounts[0])
		}
		if result.Accounts[1].Status != 1 || result.Accounts[1].Deleted != 0 {
			t.Fatalf("disabled target = %#v", result.Accounts[1])
		}
		if result.Accounts[1].Label != nil || result.Accounts[1].ProxyRef != nil || result.Accounts[1].LoginURL != nil {
			t.Fatalf("SQL NULL metadata was not preserved = %#v", result.Accounts[1])
		}
		if result.Accounts[2].Status != 0 || result.Accounts[2].Deleted != 1 {
			t.Fatalf("deleted target = %#v", result.Accounts[2])
		}
		if result.Accounts[2].ProxyRef == nil || *result.Accounts[2].ProxyRef != "" || result.Accounts[2].LoginURL == nil || *result.Accounts[2].LoginURL != "" {
			t.Fatalf("empty non-NULL metadata was not preserved = %#v", result.Accounts[2])
		}
		if after := fixture.tableJSON(t, "public.source_account", "id"); before != after {
			t.Fatalf("legacy source changed:\n before=%s\n after=%s", before, after)
		}
		fixture.assertCount(t, "public.organization_source_accounts", 3)
		fixture.assertCount(t, "public.source_account_ownership_migration_receipts", 1)
		fixture.assertTargetHasNoLegacyTenant(t)
		fixture.assertPreparedSchema(t)
		versions := fixture.rowVersions(t)

		replayedResult, wasReplay, err := preparer.Prepare(ctx, request)
		if err != nil {
			t.Fatalf("replay Prepare() error = %v", err)
		}
		if !wasReplay || !preparedReceiptsEqual(result, replayedResult) {
			t.Fatalf("replay = %#v, %v; want same receipt", replayedResult, wasReplay)
		}
		if got := fixture.rowVersions(t); versions != got {
			t.Fatalf("replay rewrote rows:\n before=%s\n after=%s", versions, got)
		}
		fresh, err := NewPreparer(fixture.db)
		if err != nil {
			t.Fatal(err)
		}
		read, found, err := fresh.ReadPreparedReceipt(ctx, request)
		if err != nil || !found || !preparedReceiptsEqual(result, read) {
			t.Fatalf("ReadPreparedReceipt() = %#v, %v, %v", read, found, err)
		}
	})

	t.Run("rejects idempotency target and drift conflicts", func(t *testing.T) {
		request := fixture.resetAndSeed(t)
		preparer, err := NewPreparer(fixture.db)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		committed, _, err := preparer.Prepare(ctx, request)
		if err != nil {
			t.Fatal(err)
		}

		changedPayload := clonePrepareRequest(t, request)
		changedPayload.Preflight.Accounts[0].ProfileDirectory += "-different"
		changedPayload.Preflight.Digest = receiptDigest(changedPayload.Preflight)
		if _, _, err = preparer.Prepare(ctx, changedPayload); !errors.Is(err, ErrIdempotencyConflict) {
			t.Fatalf("same key/different payload error = %v", err)
		}
		changedCapturedField := clonePrepareRequest(t, request)
		changedCapturedField.Preflight.Accounts[0].Previous.Status = 1
		changedCapturedField.Preflight.Digest = receiptDigest(changedCapturedField.Preflight)
		if _, _, err = preparer.Prepare(ctx, changedCapturedField); !errors.Is(err, ErrIdempotencyConflict) {
			t.Fatalf("same key/different captured source field error = %v", err)
		}
		differentKey := clonePrepareRequest(t, request)
		differentKey.IdempotencyKey = "migration-2"
		if _, _, err = preparer.Prepare(ctx, differentKey); !errors.Is(err, ErrTargetConflict) {
			t.Fatalf("different key/existing target error = %v", err)
		}
		fixture.assertCount(t, "public.organization_source_accounts", 3)
		fixture.assertCount(t, "public.source_account_ownership_migration_receipts", 1)

		if _, err = fixture.db.ExecContext(ctx, "UPDATE public.source_account SET label = 'source-drift' WHERE id = 1"); err != nil {
			t.Fatal(err)
		}
		if _, _, err = preparer.ReadPreparedReceipt(ctx, request); !errors.Is(err, ErrSourceDrift) {
			t.Fatalf("source drift read error = %v", err)
		}
		if _, err = fixture.db.ExecContext(ctx, "UPDATE public.source_account SET label = 'enabled' WHERE id = 1"); err != nil {
			t.Fatal(err)
		}
		if _, err = fixture.db.ExecContext(ctx, "UPDATE public.organization_source_accounts SET label = 'target-drift' WHERE id = 1"); err != nil {
			t.Fatal(err)
		}
		if _, _, err = preparer.ReadPreparedReceipt(ctx, request); !errors.Is(err, ErrTargetDrift) {
			t.Fatalf("target drift read error = %v", err)
		}
		if committed.TargetSHA256 == "" || committed.SourceSHA256 == "" {
			t.Fatal("committed receipt omitted digests")
		}
	})

	t.Run("rejects source drift before first write", func(t *testing.T) {
		request := fixture.resetAndSeed(t)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if _, err := fixture.db.ExecContext(ctx, "UPDATE public.source_account SET status = 1 WHERE id = 1"); err != nil {
			t.Fatal(err)
		}
		preparer, err := NewPreparer(fixture.db)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err = preparer.Prepare(ctx, request); !errors.Is(err, ErrSourceDrift) {
			t.Fatalf("Prepare() drift error = %v", err)
		}
		fixture.assertCount(t, "public.organization_source_accounts", 0)
		fixture.assertCount(t, "public.source_account_ownership_migration_receipts", 0)
	})

	t.Run("rejects source text outside byte budgets before writes", func(t *testing.T) {
		tests := []struct {
			name   string
			limits prepareSourceResourceLimits
		}{
			{
				name: "one field exceeds its byte limit",
				limits: prepareSourceResourceLimits{
					maxFieldBytes:    8,
					maxSnapshotBytes: maxPrepareSourceSnapshotBytes,
				},
			},
			{
				name: "aggregate text exceeds the snapshot limit",
				limits: prepareSourceResourceLimits{
					maxFieldBytes:    maxPrepareTextFieldBytes,
					maxSnapshotBytes: 32,
				},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				request := fixture.resetAndSeed(t)
				preparer, err := NewPreparer(fixture.db)
				if err != nil {
					t.Fatal(err)
				}
				preparer.sourceResourceLimits = test.limits
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				if _, _, err = preparer.Prepare(ctx, request); !errors.Is(err, ErrResourceLimit) {
					t.Fatalf("Prepare() resource error = %v", err)
				}
				fixture.assertCount(t, "public.organization_source_accounts", 0)
				fixture.assertCount(t, "public.source_account_ownership_migration_receipts", 0)
			})
		}
	})

	t.Run("rejects a different selected database before mutation", func(t *testing.T) {
		request := fixture.resetAndSeed(t)
		request.Preflight.AccountObservation.Database = "different_database"
		request.Preflight.Digest = receiptDigest(request.Preflight)
		preparer, err := NewPreparer(fixture.db)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if _, _, err = preparer.Prepare(ctx, request); err == nil {
			t.Fatal("Prepare() database mismatch error = nil")
		}
		fixture.assertCount(t, "public.organization_source_accounts", 0)
		fixture.assertCount(t, "public.source_account_ownership_migration_receipts", 0)
	})

	t.Run("rolls back failures and cancellation", func(t *testing.T) {
		tests := []struct {
			name string
			set  func(*Preparer, context.CancelFunc)
		}{
			{name: "failure after target insert", set: func(p *Preparer, _ context.CancelFunc) {
				p.afterTargetInsert = func(context.Context, *sql.Tx, int) error { return errors.New("injected target failure") }
			}},
			{name: "database error after target insert", set: func(p *Preparer, _ context.CancelFunc) {
				p.afterTargetInsert = func(ctx context.Context, tx *sql.Tx, inserted int) error {
					if inserted != 1 {
						return nil
					}
					_, err := tx.ExecContext(ctx, "INSERT INTO public.organization_source_accounts SELECT * FROM public.organization_source_accounts WHERE id = 1")
					return err
				}
			}},
			{name: "cancellation after locks", set: func(p *Preparer, cancel context.CancelFunc) {
				p.afterLocks = func() error { cancel(); return nil }
			}},
			{name: "cancellation before commit", set: func(p *Preparer, cancel context.CancelFunc) {
				p.beforeCommit = func() error { cancel(); return nil }
			}},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				request := fixture.resetAndSeed(t)
				before := fixture.tableJSON(t, "public.source_account", "id")
				preparer, err := NewPreparer(fixture.db)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				test.set(preparer, cancel)
				if _, _, err = preparer.Prepare(ctx, request); err == nil {
					t.Fatal("Prepare() error = nil")
				}
				if after := fixture.tableJSON(t, "public.source_account", "id"); before != after {
					t.Fatalf("legacy source changed:\n before=%s\n after=%s", before, after)
				}
				fixture.assertCount(t, "public.organization_source_accounts", 0)
				fixture.assertCount(t, "public.source_account_ownership_migration_receipts", 0)
			})
		}
	})

	t.Run("resolves post commit response loss from durable receipt", func(t *testing.T) {
		request := fixture.resetAndSeed(t)
		preparer, err := NewPreparer(fixture.db)
		if err != nil {
			t.Fatal(err)
		}
		responseLost := errors.New("injected response loss")
		preparer.afterCommit = func() error { return responseLost }
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if _, _, err = preparer.Prepare(ctx, request); !errors.Is(err, responseLost) {
			t.Fatalf("Prepare() error = %v, want response loss", err)
		}
		fixture.assertCount(t, "public.organization_source_accounts", 3)
		fixture.assertCount(t, "public.source_account_ownership_migration_receipts", 1)

		fresh, err := NewPreparer(fixture.db)
		if err != nil {
			t.Fatal(err)
		}
		readCtx, readCancel := context.WithTimeout(context.Background(), time.Minute)
		defer readCancel()
		read, found, err := fresh.ReadPreparedReceipt(readCtx, request)
		if err != nil || !found || read.Stage != PreparedStage || read.AccountCount != 3 {
			t.Fatalf("ReadPreparedReceipt() = %#v, %v, %v", read, found, err)
		}
	})

	t.Run("serializes concurrent identical preparation", func(t *testing.T) {
		request := fixture.resetAndSeed(t)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		const callers = 8
		results := make(chan PreparedReceipt, callers)
		replays := make(chan bool, callers)
		errs := make(chan error, callers)
		var wg sync.WaitGroup
		for index := 0; index < callers; index++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				preparer, err := NewPreparer(fixture.db)
				if err != nil {
					errs <- err
					return
				}
				result, replayed, err := preparer.Prepare(ctx, request)
				results <- result
				replays <- replayed
				errs <- err
			}()
		}
		wg.Wait()
		close(results)
		close(replays)
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("concurrent Prepare(): %v", err)
			}
		}
		fresh := 0
		for replayed := range replays {
			if !replayed {
				fresh++
			}
		}
		var first *PreparedReceipt
		for result := range results {
			if first == nil {
				copy := result
				first = &copy
				continue
			}
			if !preparedReceiptsEqual(*first, result) {
				t.Fatalf("concurrent receipts differ:\nfirst=%#v\nother=%#v", *first, result)
			}
		}
		if fresh != 1 {
			t.Fatalf("fresh preparations = %d, want 1", fresh)
		}
		fixture.assertCount(t, "public.organization_source_accounts", 3)
		fixture.assertCount(t, "public.source_account_ownership_migration_receipts", 1)
	})

	t.Run("waits for an earlier legacy writer then fails closed", func(t *testing.T) {
		operations := []struct {
			name string
			sql  string
		}{
			{name: "insert", sql: `INSERT INTO public.source_account (id, tenant_id, platform, label, profile_ref, status, deleted, created_at, updated_at) VALUES (9, 909, '1688', 'late', 'profile-late', 0, 0, '2026-09-08T00:00:00Z', '2026-09-08T00:00:00Z')`},
			{name: "update", sql: `UPDATE public.source_account SET status = 1 WHERE id = 1`},
			{name: "delete", sql: `DELETE FROM public.source_account WHERE id = 3`},
		}
		for _, operation := range operations {
			t.Run(operation.name, func(t *testing.T) {
				request := fixture.resetAndSeed(t)
				writer, err := fixture.db.BeginTx(context.Background(), nil)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = writer.Exec(operation.sql); err != nil {
					_ = writer.Rollback()
					t.Fatal(err)
				}
				preparer, err := NewPreparer(fixture.db)
				if err != nil {
					_ = writer.Rollback()
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				done := make(chan error, 1)
				go func() {
					_, _, prepareErr := preparer.Prepare(ctx, request)
					done <- prepareErr
				}()
				select {
				case err = <-done:
					_ = writer.Rollback()
					t.Fatalf("Prepare() did not wait for writer: %v", err)
				case <-time.After(150 * time.Millisecond):
				}
				if err = writer.Commit(); err != nil {
					t.Fatal(err)
				}
				if err = <-done; !errors.Is(err, ErrSourceDrift) {
					t.Fatalf("Prepare() error = %v, want source drift", err)
				}
				fixture.assertCount(t, "public.organization_source_accounts", 0)
				fixture.assertCount(t, "public.source_account_ownership_migration_receipts", 0)
			})
		}
	})

	t.Run("blocks a later legacy writer until preparation finishes", func(t *testing.T) {
		operations := []struct {
			name string
			sql  string
		}{
			{name: "insert", sql: `INSERT INTO public.source_account (id, tenant_id, platform, label, profile_ref, status, deleted, created_at, updated_at) VALUES (9, 909, '1688', 'late', 'profile-late', 0, 0, '2026-09-08T00:00:00Z', '2026-09-08T00:00:00Z')`},
			{name: "update", sql: `UPDATE public.source_account SET label = 'writer-drift' WHERE id = 1`},
			{name: "delete", sql: `DELETE FROM public.source_account WHERE id = 3`},
		}
		for _, operation := range operations {
			t.Run(operation.name, func(t *testing.T) {
				request := fixture.resetAndSeed(t)
				preparer, err := NewPreparer(fixture.db)
				if err != nil {
					t.Fatal(err)
				}
				locked := make(chan struct{})
				release := make(chan struct{})
				preparer.afterLocks = func() error {
					close(locked)
					<-release
					return nil
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				prepared := make(chan error, 1)
				go func() {
					_, _, prepareErr := preparer.Prepare(ctx, request)
					prepared <- prepareErr
				}()
				<-locked
				written := make(chan error, 1)
				go func() {
					_, writeErr := fixture.db.ExecContext(ctx, operation.sql)
					written <- writeErr
				}()
				select {
				case err = <-written:
					close(release)
					t.Fatalf("legacy writer was not blocked: %v", err)
				case <-time.After(150 * time.Millisecond):
				}
				close(release)
				if err = <-prepared; err != nil {
					t.Fatalf("Prepare() error = %v", err)
				}
				if err = <-written; err != nil {
					t.Fatalf("legacy writer error = %v", err)
				}
				fixture.assertCount(t, "public.organization_source_accounts", 3)
				fixture.assertCount(t, "public.source_account_ownership_migration_receipts", 1)
			})
		}
	})

	t.Run("rejects schema drift without writes", func(t *testing.T) {
		cases := []struct {
			name       string
			statements []string
		}{
			{name: "legacy numeric owner column", statements: []string{
				"ALTER TABLE public.organization_source_accounts ADD COLUMN tenant_id BIGINT",
			}},
			{name: "changed named constraint", statements: []string{
				"ALTER TABLE public.organization_source_accounts DROP CONSTRAINT organization_source_accounts_platform_1688",
				"ALTER TABLE public.organization_source_accounts ADD CONSTRAINT organization_source_accounts_platform_1688 CHECK (platform = 'not-1688') NOT VALID",
			}},
			{name: "changed primary key", statements: []string{
				"ALTER TABLE public.organization_source_accounts DROP CONSTRAINT organization_source_accounts_pkey",
				"ALTER TABLE public.organization_source_accounts ADD CONSTRAINT organization_source_accounts_pkey UNIQUE (id)",
			}},
			{name: "extra unique constraint", statements: []string{
				"ALTER TABLE public.organization_source_accounts ADD CONSTRAINT unexpected_platform_unique UNIQUE (platform)",
			}},
			{name: "changed reader index", statements: []string{
				"DROP INDEX public.idx_organization_source_accounts_reader",
				"CREATE INDEX idx_organization_source_accounts_reader ON public.organization_source_accounts (organization_id, id, status)",
			}},
			{name: "changed receipt constraint", statements: []string{
				"ALTER TABLE public.source_account_ownership_migration_receipts DROP CONSTRAINT source_account_ownership_receipts_stage_prepared",
				"ALTER TABLE public.source_account_ownership_migration_receipts ADD CONSTRAINT source_account_ownership_receipts_stage_prepared CHECK (stage = 'not-prepared') NOT VALID",
			}},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				request := fixture.resetAndSeed(t)
				before := fixture.tableJSON(t, "public.source_account", "id")
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				for _, statement := range test.statements {
					if _, err := fixture.db.ExecContext(ctx, statement); err != nil {
						t.Fatal(err)
					}
				}
				if err := InstallPreparedSchema(ctx, fixture.db); err == nil {
					t.Fatal("InstallPreparedSchema() accepted schema drift")
				}
				preparer, err := NewPreparer(fixture.db)
				if err != nil {
					t.Fatal(err)
				}
				if _, _, err = preparer.Prepare(ctx, request); err == nil {
					t.Fatal("Prepare() accepted schema drift")
				}
				if after := fixture.tableJSON(t, "public.source_account", "id"); before != after {
					t.Fatalf("legacy source changed:\n before=%s\n after=%s", before, after)
				}
				fixture.assertCount(t, "public.organization_source_accounts", 0)
				fixture.assertCount(t, "public.source_account_ownership_migration_receipts", 0)
			})
		}
	})

	t.Run("enforces Organization only target schema and protects other platforms", func(t *testing.T) {
		request := fixture.resetAndSeed(t)
		before := fixture.tableJSON(t, "public.source_account", "id")
		preparer, err := NewPreparer(fixture.db)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if _, _, err = preparer.Prepare(ctx, request); err != nil {
			t.Fatal(err)
		}
		if after := fixture.tableJSON(t, "public.source_account", "id"); before != after {
			t.Fatalf("source table changed:\n before=%s\n after=%s", before, after)
		}
		fixture.assertTargetHasNoLegacyTenant(t)
		var otherPlatformTargets int
		if err = fixture.db.QueryRowContext(ctx, `SELECT count(*) FROM public.organization_source_accounts WHERE id = 4`).Scan(&otherPlatformTargets); err != nil {
			t.Fatal(err)
		}
		if otherPlatformTargets != 0 {
			t.Fatalf("non-1688 targets = %d", otherPlatformTargets)
		}
		for name, statement := range map[string]string{
			"empty Organization": `INSERT INTO public.organization_source_accounts (id, organization_id, platform, profile_ref, profile_directory, status, deleted, created_at, updated_at) VALUES (20, '', '1688', 'p', '/p', 0, 0, now(), now())`,
			"wrong platform":     `INSERT INTO public.organization_source_accounts (id, organization_id, platform, profile_ref, profile_directory, status, deleted, created_at, updated_at) VALUES (21, 'org-x', 'amazon', 'p', '/p', 0, 0, now(), now())`,
			"empty profile ref":  `INSERT INTO public.organization_source_accounts (id, organization_id, platform, profile_ref, profile_directory, status, deleted, created_at, updated_at) VALUES (22, 'org-x', '1688', '', '/p', 0, 0, now(), now())`,
		} {
			t.Run(name, func(t *testing.T) {
				if _, err := fixture.db.ExecContext(ctx, statement); err == nil {
					t.Fatal("constraint error = nil")
				}
			})
		}
	})
}

type preparePostgresFixture struct {
	db        *sql.DB
	database  string
	container *tcpostgres.PostgresContainer
}

func newPreparePostgres(t *testing.T) *preparePostgresFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("issue362"),
		tcpostgres.WithUsername("issue362"),
		tcpostgres.WithPassword("issue362"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(20)
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &preparePostgresFixture{db: db, database: "issue362", container: container}
}

func (f *preparePostgresFixture) resetAndSeed(t *testing.T) PrepareRequest {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	statements := []string{
		`DROP TABLE IF EXISTS public.source_account_ownership_migration_receipts`,
		`DROP TABLE IF EXISTS public.organization_source_accounts`,
		`DROP TABLE IF EXISTS public.source_account`,
		`CREATE TABLE public.source_account (
			id BIGINT PRIMARY KEY,
			tenant_id BIGINT NOT NULL,
			platform VARCHAR(32) NOT NULL,
			label VARCHAR(128),
			profile_ref VARCHAR(256) NOT NULL,
			proxy_ref VARCHAR(256),
			login_url TEXT,
			status SMALLINT NOT NULL,
			deleted SMALLINT NOT NULL,
			last_verified_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`INSERT INTO public.source_account (id, tenant_id, platform, label, profile_ref, proxy_ref, login_url, status, deleted, last_verified_at, created_at, updated_at) VALUES
			(1, 101, '1688', 'enabled', 'profile-a', 'proxy-a', 'https://login/a', 0, 0, '2026-09-08T00:10:00Z', '2026-09-01T00:00:00Z', '2026-09-08T00:00:00Z'),
			(2, 202, '1688', NULL, 'profile-b', NULL, NULL, 1, 0, NULL, '2026-09-02T00:00:00Z', '2026-09-08T00:00:00Z'),
			(3, 303, '1688', 'deleted', 'profile-c', '', '', 0, 1, NULL, '2026-09-03T00:00:00Z', '2026-09-08T00:00:00Z'),
			(4, 404, 'amazon', 'protected', 'profile-amazon', NULL, NULL, 0, 0, NULL, '2026-09-04T00:00:00Z', '2026-09-08T00:00:00Z')`,
	}
	for _, statement := range statements {
		if _, err := f.db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := InstallPreparedSchema(ctx, f.db); err != nil {
		t.Fatal(err)
	}
	request := validPrepareTestRequest("migration-1")
	request.Preflight.AccountObservation.Database = f.database
	request.Preflight.Accounts = append(request.Preflight.Accounts, AccountEvidence{
		OrganizationID:   "org-c",
		ProfileDirectory: prepareTestProfileDirectory("303", "3"),
		Previous:         LegacyAccount{ID: 3, TenantID: 303, Platform: "1688", ProfileRef: "profile-c", Status: 0, Deleted: 1},
	})
	request.Preflight.Metadata = append(request.Preflight.Metadata, OrganizationMetadata{OrganizationID: "org-c", Value: []byte("303"), Sequence: 3})
	request.Preflight.Digest = receiptDigest(request.Preflight)
	return request
}

func (f *preparePostgresFixture) tableJSON(t *testing.T, table, order string) string {
	t.Helper()
	query := fmt.Sprintf(`SELECT COALESCE(json_agg(row_to_json(rows) ORDER BY %s)::text, '[]') FROM (SELECT * FROM %s) AS rows`, order, table)
	var value string
	if err := f.db.QueryRow(query).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func (f *preparePostgresFixture) assertCount(t *testing.T, table string, want int) {
	t.Helper()
	var got int
	if err := f.db.QueryRow(fmt.Sprintf("SELECT count(*) FROM %s", table)).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}

func (f *preparePostgresFixture) assertTargetHasNoLegacyTenant(t *testing.T) {
	t.Helper()
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'organization_source_accounts' AND column_name = 'tenant_id'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("organization target tenant_id columns = %d, want 0", count)
	}
}

func (f *preparePostgresFixture) assertPreparedSchema(t *testing.T) {
	t.Helper()
	targetWant := []string{
		"id|bigint||NO",
		"organization_id|character varying|128|NO",
		"platform|character varying|32|NO",
		"label|character varying|128|YES",
		"profile_ref|character varying|256|NO",
		"profile_directory|character varying|1024|NO",
		"proxy_ref|character varying|256|YES",
		"login_url|text||YES",
		"status|smallint||NO",
		"deleted|smallint||NO",
		"last_verified_at|timestamp with time zone||YES",
		"created_at|timestamp with time zone||NO",
		"updated_at|timestamp with time zone||NO",
	}
	receiptWant := []string{
		"contract_version|smallint||NO",
		"idempotency_key|character varying|128|NO",
		"stage|character varying|32|NO",
		"request_sha256|character|64|NO",
		"preflight_sha256|character|64|NO",
		"source_id|character varying|256|NO",
		"source_database|character varying|128|NO",
		"source_schema|character varying|63|NO",
		"source_sha256|character|64|NO",
		"target_sha256|character|64|NO",
		"account_count|integer||NO",
		"result_json|jsonb||NO",
		"prepared_at|timestamp with time zone||NO",
	}
	f.assertColumns(t, "organization_source_accounts", targetWant)
	f.assertColumns(t, "source_account_ownership_migration_receipts", receiptWant)
	var readerIndex int
	if err := f.db.QueryRow("SELECT count(*) FROM pg_indexes WHERE schemaname = 'public' AND tablename = 'organization_source_accounts' AND indexdef LIKE '%(organization_id, id, status, deleted)%'").Scan(&readerIndex); err != nil {
		t.Fatal(err)
	}
	if readerIndex != 1 {
		t.Fatalf("Organization target reader indexes = %d, want 1", readerIndex)
	}
}

func (f *preparePostgresFixture) assertColumns(t *testing.T, table string, want []string) {
	t.Helper()
	rows, err := f.db.Query("SELECT column_name, data_type, COALESCE(character_maximum_length::text, ''), is_nullable FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1 ORDER BY ordinal_position", table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := make([]string, 0, len(want))
	for rows.Next() {
		var name, dataType, maximumLength, nullable string
		if err = rows.Scan(&name, &dataType, &maximumLength, &nullable); err != nil {
			t.Fatal(err)
		}
		got = append(got, strings.Join([]string{name, dataType, maximumLength, nullable}, "|"))
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%s columns:\n got: %v\nwant: %v", table, got, want)
	}
}

func (f *preparePostgresFixture) rowVersions(t *testing.T) string {
	t.Helper()
	var targets, receipts string
	if err := f.db.QueryRow(`SELECT COALESCE(string_agg(id::text || ':' || xmin::text, ',' ORDER BY id), '') FROM public.organization_source_accounts`).Scan(&targets); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(`SELECT COALESCE(string_agg(contract_version::text || ':' || idempotency_key || ':' || xmin::text, ',' ORDER BY contract_version, idempotency_key), '') FROM public.source_account_ownership_migration_receipts`).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	return targets + "|" + receipts
}

func init() {
	if runtime.GOOS == "windows" && os.Getenv("DOCKER_HOST") == "" {
		_ = os.Setenv("DOCKER_HOST", "npipe:////./pipe/dockerDesktopLinuxEngine")
	}
}
