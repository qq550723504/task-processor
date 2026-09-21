package accountallocation

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
	platformmigration "task-processor/internal/platform/database/migration"
)

const versionTableRelation = "public.goose_account_member_token_allocation_version"
const baselineVersion = int64(2026092002)

func Migrations() []*goose.Migration {
	return []*goose.Migration{goose.NewGoMigration(baselineVersion, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
		return installSchemaTx(ctx, tx)
	}}, nil)}
}

func Migrate(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return fmt.Errorf("account allocation schema requires postgres")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get account allocation database: %w", err)
	}
	runner, err := platformmigration.NewWithVersionTable(goose.DialectPostgres, sqlDB, versionTableRelation, Migrations()...)
	if err != nil {
		return fmt.Errorf("create account allocation migration runner: %w", err)
	}
	if _, err := runner.Up(ctx); err != nil {
		return fmt.Errorf("initialize account allocation schema: %w", err)
	}
	return verifySchema(ctx, db)
}

func installSchemaTx(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return fmt.Errorf("account allocation schema transaction is nil")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS public.account_member_token_locks (
    organization_id VARCHAR(128) NOT NULL PRIMARY KEY,
    updated_at TIMESTAMPTZ NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS public.account_member_token_allocations (
    organization_id VARCHAR(128) NOT NULL,
    member_id VARCHAR(128) NOT NULL,
    metric VARCHAR(32) NOT NULL DEFAULT 'token',
    allocated BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT FALSE,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT account_member_token_allocations_pkey PRIMARY KEY (organization_id, member_id, metric),
    CONSTRAINT account_member_token_allocations_metric_check CHECK (metric = 'token'),
    CONSTRAINT account_member_token_allocations_ids_check CHECK (octet_length(organization_id) BETWEEN 1 AND 128 AND organization_id = btrim(organization_id) AND octet_length(member_id) BETWEEN 1 AND 128 AND member_id = btrim(member_id)),
    CONSTRAINT account_member_token_allocations_values_check CHECK (allocated >= 0 AND version >= 0 AND window_end > window_start)
)`,
		`CREATE TABLE IF NOT EXISTS public.account_member_token_operations (
    idempotency_key VARCHAR(128) NOT NULL PRIMARY KEY,
    organization_id VARCHAR(128) NOT NULL,
    member_id VARCHAR(128) NOT NULL,
    fingerprint CHAR(64) NOT NULL,
    target BIGINT NOT NULL,
    version BIGINT NOT NULL,
    allocated BIGINT NOT NULL,
    consumed BIGINT NOT NULL,
    active BOOLEAN NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT account_member_token_operations_values_check CHECK (target >= 0 AND version > 0 AND allocated >= 0 AND consumed >= 0 AND window_end > window_start)
)`,
		`CREATE TABLE IF NOT EXISTS public.account_member_token_audit_events (
    id BIGSERIAL PRIMARY KEY,
    organization_id VARCHAR(128) NOT NULL,
    actor_id VARCHAR(128) NOT NULL,
    member_id VARCHAR(128) NOT NULL,
    operation VARCHAR(32) NOT NULL,
    target BIGINT NOT NULL,
    allocated BIGINT NOT NULL,
    consumed BIGINT NOT NULL,
    version BIGINT NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT account_member_token_audit_values_check CHECK (target >= 0 AND allocated >= 0 AND consumed >= 0 AND version > 0)
)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("install account allocation schema: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'commercial_runtime') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE ON TABLE public.account_member_token_locks, public.account_member_token_allocations, public.account_member_token_operations, public.account_member_token_audit_events TO commercial_runtime';
        EXECUTE 'GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO commercial_runtime';
    END IF;
END
$$`); err != nil {
		return fmt.Errorf("grant account allocation runtime privileges: %w", err)
	}
	return nil
}

func verifySchema(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('account_member_token_locks','account_member_token_allocations','account_member_token_operations','account_member_token_audit_events')`).Scan(&count).Error; err != nil {
		return fmt.Errorf("verify account allocation schema: %w", err)
	}
	if count != 4 {
		return fmt.Errorf("verify account allocation schema: expected 4 tables, got %d", count)
	}
	return nil
}
