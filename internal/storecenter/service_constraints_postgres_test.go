//go:build integration

package storecenter_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"task-processor/internal/storecenter"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPostgresStoreServicePhaseEConstraintsAreGuardedBoundedAndIdempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	db, sqlDB := openStoreServiceConstraintPostgres(t, ctx)
	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		t.Fatal(err)
	}
	if storeRecordStatusNotNull(t, db) {
		t.Fatal("AutoMigrateStoreRepository() initiated Phase E on a fresh schema")
	}
	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		t.Fatalf("replayed pre-Phase E AutoMigrateStoreRepository() = %v", err)
	}
	if storeRecordStatusNotNull(t, db) {
		t.Fatal("replayed AutoMigrateStoreRepository() initiated Phase E")
	}

	ids := []string{
		"00000000-0000-4000-8000-000000000801",
		"00000000-0000-4000-8000-000000000802",
		"00000000-0000-4000-8000-000000000803",
		"00000000-0000-4000-8000-000000000804",
	}
	seedLegacyStoreRow(t, db, ids[0], "org-phase-e", "active", 1, nil)
	seedLegacyStoreRow(t, db, ids[1], "org-phase-e", "disabled", 1, nil)
	seedLegacyStoreRow(t, db, ids[2], "org-phase-e", "provisioning", 1, nil)
	seedLegacyStoreRow(t, db, ids[3], "org-phase-e", "deleting", 1, nil)
	migrator := newNoHistoryMigrator(t, db, time.Date(2026, 9, 5, 1, 0, 0, 0, time.UTC))
	options := storecenter.StoreServiceConstraintOptions{LockTimeout: 500 * time.Millisecond, StatementTimeout: 15 * time.Second}

	blocked, err := migrator.ApplyConstraints(ctx, options)
	if !errors.Is(err, storecenter.ErrStoreHistoryRolloutBlocked) || blocked.PhaseD.ReadyForConstraints || countPhaseEConstraints(t, db) != 0 {
		t.Fatalf("pre-backfill ApplyConstraints() = %+v, %v; want Phase D blocker and zero DDL", blocked, err)
	}
	if report, err := migrator.BackfillBatch(ctx, 100); err != nil || report.UpdatedCount != 4 {
		t.Fatalf("BackfillBatch() = %+v, %v", report, err)
	}

	if err := db.Exec(`ALTER TABLE workbench_stores ADD CONSTRAINT workbench_stores_record_status_nn CHECK (TRUE) NOT VALID`).Error; err != nil {
		t.Fatal(err)
	}
	owned, err := migrator.ApplyConstraints(ctx, options)
	if !errors.Is(err, storecenter.ErrStoreServiceConstraintOwnership) || owned.ConstraintsApplied || countPhaseEConstraints(t, db) != 1 || storeRecordStatusNotNull(t, db) {
		t.Fatalf("foreign constraint ApplyConstraints() = %+v, %v", owned, err)
	}
	if err := db.Exec(`ALTER TABLE workbench_stores DROP CONSTRAINT workbench_stores_record_status_nn`).Error; err != nil {
		t.Fatal(err)
	}

	locker, err := sqlDB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := locker.ExecContext(ctx, `LOCK TABLE workbench_stores IN SHARE MODE`); err != nil {
		_ = locker.Rollback()
		t.Fatal(err)
	}
	started := time.Now()
	_, lockErr := migrator.ApplyConstraints(ctx, storecenter.StoreServiceConstraintOptions{LockTimeout: 75 * time.Millisecond, StatementTimeout: 2 * time.Second})
	elapsed := time.Since(started)
	if rollbackErr := locker.Rollback(); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if postgresSQLState(lockErr) != "55P03" || elapsed >= time.Second || countPhaseEConstraints(t, db) != 0 {
		t.Fatalf("locked ApplyConstraints() error/state/elapsed/constraints = %v/%q/%s/%d", lockErr, postgresSQLState(lockErr), elapsed, countPhaseEConstraints(t, db))
	}

	report, err := migrator.ApplyConstraints(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if !report.PhaseD.ReadyForConstraints || report.ConstraintsAdded != 5 || report.ConstraintsValidated != 5 || !report.RecordStatusNotNull || !report.ConstraintsApplied {
		t.Fatalf("ApplyConstraints() report = %+v", report)
	}
	assertPhaseEConstraintsValidated(t, db, 5)

	replayed, err := migrator.ApplyConstraints(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ConstraintsAdded != 0 || replayed.ConstraintsValidated != 0 || !replayed.RecordStatusNotNull || !replayed.ConstraintsApplied {
		t.Fatalf("replayed ApplyConstraints() report = %+v", replayed)
	}
	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		t.Fatalf("AutoMigrateStoreRepository() after Phase E = %v", err)
	}
	if !storeRecordStatusNotNull(t, db) {
		t.Fatal("AutoMigrateStoreRepository() reversed Phase E record_status NOT NULL")
	}

	assertStoreServiceConstraintRejects(t, db, ids[0], map[string]any{"record_status": nil})
	assertStoreServiceConstraintRejects(t, db, ids[0], map[string]any{"record_status": "unknown"})
	assertStoreServiceConstraintRejects(t, db, ids[0], map[string]any{"service_status": nil})
	assertStoreServiceConstraintRejects(t, db, ids[0], map[string]any{"service_started_at": time.Now().UTC()})
	assertStoreServiceConstraintRejects(t, db, ids[3], map[string]any{"service_status": "pending_activation"})

	startedAt := time.Date(2026, 9, 5, 2, 0, 0, 0, time.UTC)
	expiresAt := startedAt.Add(30 * 24 * time.Hour)
	if err := db.Table("workbench_stores").Where("id = ?", ids[0]).Updates(map[string]any{
		"service_status": "active", "service_started_at": startedAt, "service_expires_at": expiresAt,
	}).Error; err != nil {
		t.Fatalf("valid active period rejected: %v", err)
	}
	assertStoreServiceConstraintRejects(t, db, ids[0], map[string]any{"service_expires_at": startedAt})
	if err := db.Table("workbench_stores").Where("id = ?", ids[1]).Updates(map[string]any{
		"service_status": "suspended", "service_started_at": startedAt, "service_expires_at": expiresAt,
	}).Error; err != nil {
		t.Fatalf("valid suspended history rejected: %v", err)
	}
	assertStoreServiceConstraintRejects(t, db, ids[1], map[string]any{"service_expires_at": nil})
}

func openStoreServiceConstraintPostgres(t *testing.T, ctx context.Context) (*gorm.DB, *sql.DB) {
	t.Helper()
	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("storeconstraints"),
		tcpostgres.WithUsername("storeconstraints"),
		tcpostgres.WithPassword("storeconstraints"),
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
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, sqlDB
}

func countPhaseEConstraints(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Raw(`
SELECT COUNT(*)
FROM pg_constraint AS constraint_row
JOIN pg_class AS table_row ON table_row.oid = constraint_row.conrelid
JOIN pg_namespace AS schema_row ON schema_row.oid = table_row.relnamespace
WHERE table_row.relname = 'workbench_stores'
  AND schema_row.nspname = current_schema()
  AND constraint_row.conname = ANY (ARRAY[
      'workbench_stores_record_status_nn',
      'workbench_stores_record_status_enum',
      'workbench_stores_service_status_enum',
      'workbench_stores_record_service_shape',
      'workbench_stores_service_timestamp_shape'
  ])`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func assertPhaseEConstraintsValidated(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Raw(`
SELECT COUNT(*)
FROM pg_constraint AS constraint_row
JOIN pg_class AS table_row ON table_row.oid = constraint_row.conrelid
JOIN pg_namespace AS schema_row ON schema_row.oid = table_row.relnamespace
WHERE table_row.relname = 'workbench_stores'
  AND schema_row.nspname = current_schema()
  AND constraint_row.conname = ANY (ARRAY[
      'workbench_stores_record_status_nn',
      'workbench_stores_record_status_enum',
      'workbench_stores_service_status_enum',
      'workbench_stores_record_service_shape',
      'workbench_stores_service_timestamp_shape'
  ])
  AND constraint_row.convalidated`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want || !storeRecordStatusNotNull(t, db) {
		t.Fatalf("validated constraints/not-null = %d/%v, want %d/true", count, storeRecordStatusNotNull(t, db), want)
	}
}

func storeRecordStatusNotNull(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	var notNull bool
	if err := db.Raw(`
SELECT attribute.attnotnull
FROM pg_attribute AS attribute
JOIN pg_class AS table_row ON table_row.oid = attribute.attrelid
JOIN pg_namespace AS schema_row ON schema_row.oid = table_row.relnamespace
WHERE table_row.relname = 'workbench_stores'
  AND schema_row.nspname = current_schema()
  AND attribute.attname = 'record_status'`).Scan(&notNull).Error; err != nil {
		t.Fatal(err)
	}
	return notNull
}

func assertStoreServiceConstraintRejects(t *testing.T, db *gorm.DB, id string, updates map[string]any) {
	t.Helper()
	err := db.Table("workbench_stores").Where("id = ?", id).Updates(updates).Error
	if state := postgresSQLState(err); state != "23502" && state != "23514" {
		t.Fatalf("updates %+v error/state = %v/%q, want PostgreSQL constraint rejection", updates, err, state)
	}
}

func postgresSQLState(err error) string {
	for current := err; current != nil; current = errors.Unwrap(current) {
		if stateError, ok := current.(interface{ SQLState() string }); ok {
			return stateError.SQLState()
		}
		if strings.Contains(current.Error(), "SQLSTATE 55P03") {
			return "55P03"
		}
	}
	return ""
}
