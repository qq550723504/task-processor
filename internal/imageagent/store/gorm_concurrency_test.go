package store

import (
	"context"
	"encoding/json"
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
	projectionCallerKey  gormConcurrencyContextKey = "projection-caller"
)

func TestGormProjectionCommitConcurrentExactCommitIDReturnsWinnerReceipt(t *testing.T) {
	const callers = 6
	db := newConcurrentSQLite(t)
	repository := NewGormRepository(db)
	ctx := context.Background()
	run := manualRun("run-projection-concurrent", "tenant-a")
	run.Version = 1
	plan := planRevision(1)
	scope := imageagent.ScopeForRun(*run)
	initial, err := repository.InitializeRun(ctx, imageagent.ProjectionInitialization{
		Scope: scope, Run: *run, Plan: plan,
		Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"},
			{ID: "style-1", Type: imageagent.AuthorizedAssetStyle},
		}},
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: "start:projection-concurrent",
		EventType: "run.initialized", EventPayload: []byte(`{}`),
	})
	require.NoError(t, err)
	next := initial
	next.Run.Status = imageagent.RunStatusExecuting
	next.Run.CurrentNode = "execute_slots"
	next.Run.Version++
	commit := imageagent.ProjectionCommit{
		Scope: scope, CommitID: "run:projection-concurrent", ExpectedProjectionVersion: initial.ProjectionVersion,
		ExpectedRunVersion: initial.Run.Version, Snapshot: next, EventType: "run.updated", EventPayload: []byte(`{}`),
		RunMutation: &imageagent.RunMutation{Status: next.Run.Status, CurrentNode: next.Run.CurrentNode, ActivePlanRevision: 1},
	}

	start := make(chan struct{})
	type outcome struct {
		projection imageagent.RunProjection
		err        error
	}
	outcomes := make(chan outcome, callers)
	for index := range callers {
		go func(caller int) {
			<-start
			projection, commitErr := repository.CommitProjection(ctx, commit)
			outcomes <- outcome{projection: projection, err: commitErr}
		}(index)
	}
	close(start)
	for range callers {
		outcome := <-outcomes
		require.NoError(t, outcome.err)
		require.EqualValues(t, 2, outcome.projection.ProjectionVersion)
		require.Equal(t, imageagent.RunStatusExecuting, outcome.projection.Run.Status)
	}

	conflict := commit
	conflict.Snapshot.Run.CurrentNode = "conflicting-node"
	_, err = repository.CommitProjection(ctx, conflict)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
}

func TestGormPostgresProjectionCommitRechecksReceiptAfterConcurrentLockAcquisition(t *testing.T) {
	const callers = 2
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
	repository := NewGormRepository(db)
	run := manualRun("run-postgres-projection-concurrent", "tenant-a")
	run.Version = 1
	run.ActivePlanRevision = 1
	plan := planRevision(1)
	scope := imageagent.ScopeForRun(*run)
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{
		Manifest: imageagent.CatalogManifest{Version: 1, CreatedAt: time.Unix(123, 0).UTC()},
		Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"},
			{ID: "style-1", Type: imageagent.AuthorizedAssetStyle},
		},
	})
	require.NoError(t, err)
	current := imageagent.RunProjection{
		Run: *run, Plan: plan, Slots: initialSlotProjections(plan), AssetCatalog: catalog,
		ProjectionVersion: 1, LastEventID: 1, Actions: imageagent.AllowedActions(*run),
	}
	next := cloneProjection(current)
	next.Run.Status = imageagent.RunStatusExecuting
	next.Run.CurrentNode = "execute_slots"
	next.Run.Version = 2
	commit := imageagent.ProjectionCommit{
		Scope: scope, CommitID: "run:postgres-concurrent", ExpectedProjectionVersion: 1, ExpectedRunVersion: 1,
		Snapshot: next, EventType: "run.updated", EventPayload: []byte(`{}`),
		RunMutation: &imageagent.RunMutation{Status: imageagent.RunStatusExecuting, CurrentNode: "execute_slots", ActivePlanRevision: 1},
	}
	fingerprint, err := projectionCommitFingerprint(commit)
	require.NoError(t, err)
	next.LastEventID, next.ProjectionVersion = 2, 2
	next.Actions = imageagent.AllowedActions(next.Run)
	currentJSON, err := json.Marshal(current)
	require.NoError(t, err)
	nextJSON, err := json.Marshal(next)
	require.NoError(t, err)

	for range callers {
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT .*image_agent_v2_projection_commits.*commit_id.*`).
			WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "owner_user_id", "run_id", "commit_id", "fingerprint", "version", "snapshot_json", "created_at"}))
		mock.ExpectQuery(`SELECT .*image_agent_v2_runs.*FOR UPDATE`).
			WillReturnRows(postgresRunRows(*run))
		mock.ExpectQuery(`SELECT .*image_agent_v2_projection_snapshots.*FOR UPDATE`).
			WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "owner_user_id", "run_id", "version", "snapshot_json", "created_at", "updated_at"}).
				AddRow(scope.TenantID, scope.OwnerUserID, scope.RunID, 1, currentJSON, time.Unix(0, 0), time.Unix(0, 0)))
	}
	// After both callers passed the initial no-receipt read, one lock owner writes
	// while the second observes that exact durable receipt on the mandatory
	// post-lock recheck. Both transport attempts must return the same snapshot.
	mock.ExpectQuery(`SELECT .*image_agent_v2_projection_commits.*commit_id.*`).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "owner_user_id", "run_id", "commit_id", "fingerprint", "version", "snapshot_json", "created_at"}))
	mock.ExpectQuery(`SELECT .*image_agent_v2_projection_commits.*commit_id.*`).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "owner_user_id", "run_id", "commit_id", "fingerprint", "version", "snapshot_json", "created_at"}).
			AddRow(scope.TenantID, scope.OwnerUserID, scope.RunID, commit.CommitID, fingerprint, 2, nextJSON, time.Unix(0, 0)))
	mock.ExpectExec(`UPDATE .*image_agent_v2_runs.*`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE .*image_agent_v2_projection_snapshots.*`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO .*image_agent_v2_events.*`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO .*image_agent_v2_projection_commits.*`).WillReturnResult(sqlmock.NewResult(1, 1))
	for range callers {
		mock.ExpectCommit()
	}

	arrived := make(chan struct{})
	release := make(chan struct{})
	seen := map[int]bool{}
	var barrier sync.Mutex
	reached := 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:round3:projection-first-receipt-barrier", func(tx *gorm.DB) {
		caller, ok := tx.Statement.Context.Value(projectionCallerKey).(int)
		if !ok || tx.Statement.Table != "image_agent_v2_projection_commits" {
			return
		}
		barrier.Lock()
		if seen[caller] {
			barrier.Unlock()
			return
		}
		seen[caller] = true
		reached++
		if reached == callers {
			close(arrived)
		}
		barrier.Unlock()
		<-release
	}))

	results := make(chan struct {
		projection imageagent.RunProjection
		err        error
	}, callers)
	for caller := range callers {
		go func(caller int) {
			projection, commitErr := repository.CommitProjection(context.WithValue(context.Background(), projectionCallerKey, caller), commit)
			results <- struct {
				projection imageagent.RunProjection
				err        error
			}{projection: projection, err: commitErr}
		}(caller)
	}
	<-arrived
	close(release)
	collected := make([]struct {
		projection imageagent.RunProjection
		err        error
	}, 0, callers)
	for range callers {
		collected = append(collected, <-results)
	}
	for _, result := range collected {
		require.NoError(t, result.err)
		require.Equal(t, next, result.projection)
	}
	conflicting := commit
	conflicting.Snapshot.Run.CurrentNode = "different-node"
	for range 2 {
		mock.ExpectQuery(`SELECT .*image_agent_v2_projection_commits.*commit_id.*`).
			WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "owner_user_id", "run_id", "commit_id", "fingerprint", "version", "snapshot_json", "created_at"}).
				AddRow(scope.TenantID, scope.OwnerUserID, scope.RunID, commit.CommitID, fingerprint, 2, nextJSON, time.Unix(0, 0)))
	}
	mock.ExpectBegin()
	mock.ExpectRollback()
	_, err = repository.CommitProjection(context.Background(), conflicting)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func postgresRunRows(run imageagent.Run) *sqlmock.Rows {
	budget, _ := json.Marshal(run.Budget)
	usage, _ := json.Marshal(run.Usage)
	block, _ := json.Marshal(run.Block)
	return sqlmock.NewRows([]string{
		"tenant_id", "owner_user_id", "id", "business_task_id", "mode", "idempotency_key", "status", "current_node", "active_plan_revision", "version", "budget_json", "usage_json", "block_json", "created_at", "updated_at",
	}).AddRow(
		run.TenantID, run.UserID, run.ID, run.BusinessTaskID, run.Mode, run.IdempotencyKey, run.Status, run.CurrentNode,
		run.ActivePlanRevision, run.Version, budget, usage, block, time.Unix(0, 0), time.Unix(0, 0),
	)
}

func TestGormInitializeRunConcurrentExactCommitIDReturnsWinnerReceipt(t *testing.T) {
	const callers = 6
	db := newConcurrentSQLite(t)
	repository := NewGormRepository(db)
	run := manualRun("run-initialize-concurrent", "tenant-a")
	plan := planRevision(1)
	input := imageagent.ProjectionInitialization{
		Scope: imageagent.ScopeForRun(*run), Run: *run, Plan: plan,
		Catalog: imageagent.AssetCatalog{Manifest: imageagent.CatalogManifest{Version: 1, CreatedAt: time.Unix(123, 0).UTC()}, Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"},
			{ID: "style-1", Type: imageagent.AuthorizedAssetStyle},
		}},
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: "start:initialize-concurrent",
		EventType: "run.initialized", EventPayload: []byte(`{}`),
	}

	start := make(chan struct{})
	type outcome struct {
		projection imageagent.RunProjection
		err        error
	}
	outcomes := make(chan outcome, callers)
	for range callers {
		go func() {
			<-start
			projection, err := repository.InitializeRun(context.Background(), input)
			outcomes <- outcome{projection: projection, err: err}
		}()
	}
	close(start)
	for range callers {
		result := <-outcomes
		require.NoError(t, result.err)
		require.EqualValues(t, 1, result.projection.ProjectionVersion)
		require.Equal(t, input.Catalog.Manifest.CreatedAt, result.projection.AssetCatalog.Manifest.CreatedAt)
	}

	conflict := input
	conflict.Run.BusinessTaskID = "different-task"
	conflict.Snapshot.Run = conflict.Run
	_, err := repository.InitializeRun(context.Background(), conflict)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
}

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
			if tx.Statement.Table != "image_agent_v2_runs" || tx.Statement.Context.Value(saveSlotBarrierKey) != true {
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
			if tx.Statement.Table != "image_agent_v2_runs" || tx.Statement.Context.Value(appendPlanBarrierKey) != true {
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
	attempt := imageagent.StepAttempt{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-1", PlanRevision: 1, SlotID: "slot-1", Node: "generate", IdempotencyKey: "attempt-1", Attempt: 1, Outcome: "succeeded"}

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
			WithArgs(attempt.TenantID, attempt.OwnerUserID, attempt.RunID, attempt.PlanRevision, attempt.SlotID, attempt.Attempt, attempt.Node, attempt.IdempotencyKey, attempt.Outcome, attempt.ErrorCategory, sqlmock.AnyArg()).
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
		if tx.Statement.Table != "image_agent_v2_attempts" || tx.Statement.Context.Value(appendAttemptKey) != true {
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
		WithArgs(conflicting.TenantID, conflicting.OwnerUserID, conflicting.RunID, conflicting.PlanRevision, conflicting.SlotID, conflicting.Attempt, conflicting.Node, conflicting.IdempotencyKey, conflicting.Outcome, conflicting.ErrorCategory, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	expectAppendAttemptEqualityLookup(mock, attempt)
	mock.ExpectRollback()
	require.ErrorIs(t, repo.AppendAttempt(ctx, conflicting), imageagent.ErrRevisionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectAppendAttemptRunLookup(mock sqlmock.Sqlmock, attempt imageagent.StepAttempt) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "image_agent_v2_runs" WHERE tenant_id = $1 AND owner_user_id = $2 AND id = $3 ORDER BY "image_agent_v2_runs"."id" LIMIT $4`)).
		WithArgs(attempt.TenantID, attempt.OwnerUserID, attempt.RunID, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"tenant_id", "id", "business_task_id", "owner_user_id", "mode", "idempotency_key", "status", "current_node", "active_plan_revision", "version", "budget_json", "usage_json", "block_json", "created_at", "updated_at",
		}).AddRow(
			attempt.TenantID, attempt.RunID, "task-1", "user-1", imageagent.RunModeManual, "run-key", imageagent.RunStatusPlanning, "", 0, 1, []byte(`{}`), []byte(`{}`), []byte(`null`), time.Unix(0, 0), time.Unix(0, 0),
		))
}

func expectAppendAttemptEqualityLookup(mock sqlmock.Sqlmock, attempt imageagent.StepAttempt) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "image_agent_v2_attempts" WHERE tenant_id = $1 AND owner_user_id = $2 AND run_id = $3 AND plan_revision = $4 AND slot_id = $5 AND (attempt = $6 OR idempotency_key = $7)`)).
		WithArgs(attempt.TenantID, attempt.OwnerUserID, attempt.RunID, attempt.PlanRevision, attempt.SlotID, attempt.Attempt, attempt.IdempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{
			"tenant_id", "owner_user_id", "run_id", "plan_revision", "slot_id", "attempt", "node", "idempotency_key", "outcome", "error_category", "created_at",
		}).AddRow(
			attempt.TenantID, attempt.OwnerUserID, attempt.RunID, attempt.PlanRevision, attempt.SlotID, attempt.Attempt, attempt.Node, attempt.IdempotencyKey, attempt.Outcome, attempt.ErrorCategory, time.Unix(0, 0),
		))
}

func postgresAttemptInsertSQL() string {
	return regexp.QuoteMeta(`INSERT INTO "image_agent_v2_attempts" ("tenant_id","owner_user_id","run_id","plan_revision","slot_id","attempt","node","idempotency_key","outcome","error_category","created_at") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT DO NOTHING`)
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
