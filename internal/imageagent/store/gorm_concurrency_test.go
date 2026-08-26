package store

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/imageagent"
)

type gormConcurrencyContextKey string

const (
	saveSlotBarrierKey   gormConcurrencyContextKey = "save-slot-barrier"
	appendPlanBarrierKey gormConcurrencyContextKey = "append-plan-barrier"
	appendAttemptKey     gormConcurrencyContextKey = "append-attempt-barrier"
)

func TestGormSaveSlotResultAndAppendPlanSerialize(t *testing.T) {
	t.Run("slot write loses revision race without mutating inactive revision", func(t *testing.T) {
		db := newConcurrentSQLite(t)
		repo := NewGormRepository(db).(repositoryContract)
		ctx := context.Background()
		scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-1"}
		require.NoError(t, repo.CreateRun(ctx, manualRun(scope.RunID, scope.TenantID)))
		require.NoError(t, repo.AppendPlan(ctx, scope, 0, planRevision(1)))

		blocked := make(chan struct{})
		release := make(chan struct{})
		require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:round2:block-save-run-read", func(tx *gorm.DB) {
			if tx.Statement.Table != "image_agent_runs" || tx.Statement.Context.Value(saveSlotBarrierKey) != true {
				return
			}
			close(blocked)
			<-release
		}))

		saveDone := make(chan error, 1)
		saveCtx := context.WithValue(ctx, saveSlotBarrierKey, true)
		go func() {
			saveDone <- repo.SaveSlotResult(saveCtx, scope, 1, imageagent.SlotResult{SlotID: "slot-1", Attempt: 1, Status: imageagent.SlotStatusAccepted, CandidateAssetIDs: []string{"asset-1"}})
		}()
		<-blocked
		require.NoError(t, repo.AppendPlan(ctx, scope, 1, planRevision(2)))
		close(release)
		require.ErrorIs(t, <-saveDone, imageagent.ErrRevisionConflict)

		var inactive slotRecord
		require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND plan_revision = ? AND id = ?", scope.TenantID, scope.RunID, 1, "slot-1").First(&inactive).Error)
		require.Equal(t, 0, inactive.Attempt)
		require.Equal(t, imageagent.SlotStatusPending, imageagent.SlotStatus(inactive.Status))
	})

	t.Run("slot write commits before revision advance", func(t *testing.T) {
		db := newConcurrentSQLite(t)
		repo := NewGormRepository(db).(repositoryContract)
		ctx := context.Background()
		scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-1"}
		require.NoError(t, repo.CreateRun(ctx, manualRun(scope.RunID, scope.TenantID)))
		require.NoError(t, repo.AppendPlan(ctx, scope, 0, planRevision(1)))

		blocked := make(chan struct{})
		release := make(chan struct{})
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:round2:block-plan-revision-update", func(tx *gorm.DB) {
			if tx.Statement.Table != "image_agent_runs" || tx.Statement.Context.Value(appendPlanBarrierKey) != true {
				return
			}
			close(blocked)
			<-release
		}))

		planDone := make(chan error, 1)
		planCtx := context.WithValue(ctx, appendPlanBarrierKey, true)
		go func() {
			planDone <- repo.AppendPlan(planCtx, scope, 1, planRevision(2))
		}()
		<-blocked
		require.NoError(t, repo.SaveSlotResult(ctx, scope, 1, imageagent.SlotResult{SlotID: "slot-1", Attempt: 1, Status: imageagent.SlotStatusAccepted, CandidateAssetIDs: []string{"asset-1"}}))
		close(release)
		require.NoError(t, <-planDone)

		var inactive slotRecord
		require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND plan_revision = ? AND id = ?", scope.TenantID, scope.RunID, 1, "slot-1").First(&inactive).Error)
		require.Equal(t, 1, inactive.Attempt)
		require.Equal(t, imageagent.SlotStatusAccepted, imageagent.SlotStatus(inactive.Status))
	})
}

func TestGormAppendAttemptPostgresConcurrentIdenticalRetries(t *testing.T) {
	const callers = 8

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(callers)
	sqlDB.SetMaxIdleConns(callers)
	mock.MatchExpectationsInOrder(false)

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, WithoutReturning: true}), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	repo := NewGormRepository(db).(repositoryContract)
	ctx := context.Background()
	attempt := imageagent.StepAttempt{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-1", SlotID: "slot-1", Node: "generate", IdempotencyKey: "attempt-1", Attempt: 1, Outcome: "succeeded"}

	for range callers {
		mock.ExpectBegin()
		expectAppendAttemptRunLookup(mock, attempt)
	}
	for caller := range callers {
		result := sqlmock.NewResult(0, 0)
		if caller == 0 {
			result = sqlmock.NewResult(0, 1)
		}
		mock.ExpectExec(postgresAttemptInsertSQL()).
			WithArgs(attempt.TenantID, attempt.OwnerUserID, attempt.RunID, attempt.SlotID, attempt.Attempt, attempt.Node, attempt.IdempotencyKey, attempt.Outcome, attempt.ErrorCategory, sqlmock.AnyArg()).
			WillReturnResult(result)
	}
	for range callers - 1 {
		expectAppendAttemptEqualityLookup(mock, attempt)
	}
	for range callers {
		mock.ExpectCommit()
	}

	arrived := make(chan struct{})
	release := make(chan struct{})
	var barrier sync.Mutex
	reached := 0
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:round3:barrier-attempt-insert", func(tx *gorm.DB) {
		if tx.Statement.Table != "image_agent_attempts" || tx.Statement.Context.Value(appendAttemptKey) != true {
			return
		}
		barrier.Lock()
		reached++
		if reached == callers {
			close(arrived)
		}
		barrier.Unlock()
		<-release
	}))

	start := make(chan struct{})
	errs := make(chan error, callers)
	var workers sync.WaitGroup
	for range callers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			errs <- repo.AppendAttempt(context.WithValue(ctx, appendAttemptKey, true), attempt)
		}()
	}
	close(start)
	workersDone := make(chan struct{})
	go func() {
		workers.Wait()
		close(workersDone)
	}()
	for {
		select {
		case <-arrived:
			goto insertReady
		case err := <-errs:
			if err != nil {
				close(release)
				workers.Wait()
				t.Fatalf("AppendAttempt returned before reaching the insert barrier: %v", err)
			}
		case <-workersDone:
			close(release)
			close(errs)
			for err := range errs {
				t.Logf("AppendAttempt returned before reaching the insert barrier: %v", err)
			}
			t.Fatal("not every caller reached the PostgreSQL INSERT path")
		}
	}

insertReady:
	close(release)
	workers.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, callers, reached, "all callers reached the PostgreSQL INSERT path before release")

	conflicting := attempt
	conflicting.Outcome = "failed"
	mock.ExpectBegin()
	expectAppendAttemptRunLookup(mock, conflicting)
	mock.ExpectExec(postgresAttemptInsertSQL()).
		WithArgs(conflicting.TenantID, conflicting.OwnerUserID, conflicting.RunID, conflicting.SlotID, conflicting.Attempt, conflicting.Node, conflicting.IdempotencyKey, conflicting.Outcome, conflicting.ErrorCategory, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	expectAppendAttemptEqualityLookup(mock, attempt)
	mock.ExpectRollback()
	require.ErrorIs(t, repo.AppendAttempt(ctx, conflicting), imageagent.ErrRevisionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectAppendAttemptRunLookup(mock sqlmock.Sqlmock, attempt imageagent.StepAttempt) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "image_agent_runs" WHERE tenant_id = $1 AND user_id = $2 AND id = $3 ORDER BY "image_agent_runs"."id" LIMIT $4`)).
		WithArgs(attempt.TenantID, attempt.OwnerUserID, attempt.RunID, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"tenant_id", "id", "business_task_id", "user_id", "mode", "idempotency_key", "status", "current_node", "active_plan_revision", "version", "budget_json", "usage_json", "block_json", "created_at", "updated_at",
		}).AddRow(
			attempt.TenantID, attempt.RunID, "task-1", "user-1", imageagent.RunModeManual, "run-key", imageagent.RunStatusPlanning, "", 0, 1, []byte(`{}`), []byte(`{}`), []byte(`null`), time.Unix(0, 0), time.Unix(0, 0),
		))
}

func expectAppendAttemptEqualityLookup(mock sqlmock.Sqlmock, attempt imageagent.StepAttempt) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "image_agent_attempts" WHERE tenant_id = $1 AND owner_user_id = $2 AND run_id = $3 AND slot_id = $4 AND (attempt = $5 OR idempotency_key = $6)`)).
		WithArgs(attempt.TenantID, attempt.OwnerUserID, attempt.RunID, attempt.SlotID, attempt.Attempt, attempt.IdempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{
			"tenant_id", "owner_user_id", "run_id", "slot_id", "attempt", "node", "idempotency_key", "outcome", "error_category", "created_at",
		}).AddRow(
			attempt.TenantID, attempt.OwnerUserID, attempt.RunID, attempt.SlotID, attempt.Attempt, attempt.Node, attempt.IdempotencyKey, attempt.Outcome, attempt.ErrorCategory, time.Unix(0, 0),
		))
}

func postgresAttemptInsertSQL() string {
	return regexp.QuoteMeta(`INSERT INTO "image_agent_attempts" ("tenant_id","owner_user_id","run_id","slot_id","attempt","node","idempotency_key","outcome","error_category","created_at") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`)
}

func newConcurrentSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared&_busy_timeout=5000", strings.ReplaceAll(t.Name(), "/", "_"), concurrentSQLiteSequence.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, AutoMigrate(db))
	return db
}

var concurrentSQLiteSequence atomic.Uint64
