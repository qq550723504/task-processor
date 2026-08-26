package store

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/imageagent"
)

type gormConcurrencyContextKey string

const (
	saveSlotBarrierKey   gormConcurrencyContextKey = "save-slot-barrier"
	appendPlanBarrierKey gormConcurrencyContextKey = "append-plan-barrier"
)

func TestGormSaveSlotResultAndAppendPlanSerialize(t *testing.T) {
	t.Run("slot write loses revision race without mutating inactive revision", func(t *testing.T) {
		db := newConcurrentSQLite(t)
		repo := NewGormRepository(db)
		ctx := context.Background()
		scope := imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}
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
		repo := NewGormRepository(db)
		ctx := context.Background()
		scope := imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}
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

func TestGormAppendAttemptConcurrentIdenticalRetries(t *testing.T) {
	db := newConcurrentSQLite(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	repo := NewGormRepository(db)
	ctx := context.Background()
	attempt := imageagent.StepAttempt{TenantID: "tenant-a", RunID: "run-1", SlotID: "slot-1", Node: "generate", IdempotencyKey: "attempt-1", Attempt: 1, Outcome: "succeeded"}
	require.NoError(t, repo.CreateRun(ctx, manualRun(attempt.RunID, attempt.TenantID)))
	require.NoError(t, repo.AppendAttempt(ctx, attempt))

	const callers = 8
	start := make(chan struct{})
	errs := make(chan error, callers)
	var workers sync.WaitGroup
	for range callers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			errs <- repo.AppendAttempt(ctx, attempt)
		}()
	}
	close(start)
	workers.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	var count int64
	require.NoError(t, db.Model(&attemptRecord{}).Where("tenant_id = ? AND run_id = ? AND slot_id = ? AND attempt = ? AND idempotency_key = ?", attempt.TenantID, attempt.RunID, attempt.SlotID, attempt.Attempt, attempt.IdempotencyKey).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func newConcurrentSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=5000", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, AutoMigrate(db))
	return db
}
