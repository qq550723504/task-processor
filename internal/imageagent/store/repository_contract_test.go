package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/imageagent"
)

func TestRepositoryContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		new  func(t *testing.T) imageagent.Repository
	}{
		{
			name: "memory",
			new: func(t *testing.T) imageagent.Repository {
				t.Helper()
				return NewMemoryRepository()
			},
		},
		{
			name: "gorm sqlite",
			new: func(t *testing.T) imageagent.Repository {
				t.Helper()
				db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
				require.NoError(t, err)
				require.NoError(t, AutoMigrate(db))
				return NewGormRepository(db)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testOnlyManualRunsAreAccepted(t, tt.new(t))
			testRunCreateRetriesAreIdempotent(t, tt.new(t))
			testRunJSONRoundTrips(t, tt.new(t))
			testStalePlanRevisionIsRejected(t, tt.new(t))
			testPlanAppendRetriesAreIdempotent(t, tt.new(t))
			testCrossTenantRunLookupIsHidden(t, tt.new(t))
			testRunMutationAppendsDeterministicProjectionEvent(t, tt.new(t))
			testSlotResultRequiresCurrentPlanRevision(t, tt.new(t))
			testSlotResultRetryAndAttemptOrdering(t, tt.new(t))
			testAppendedEventIsVisibleInCursorOrder(t, tt.new(t))
			testProjectionEventsAllocateOneDurableCursorPerChange(t, tt.new(t))
			testAuthorizedAssetCatalogRoundTripsAndCannotBeReplaced(t, tt.new(t))
			testAttemptIdentitiesAreIdempotentAndNonAliasing(t, tt.new(t))
		})
	}
}

func testProjectionEventsAllocateOneDurableCursorPerChange(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-cursor", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", RunID: "run-cursor"}
	first, err := repo.AppendProjectionEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, RunID: scope.RunID, Type: "slot.result.persisted", Payload: json.RawMessage(`{}`)})
	require.NoError(t, err)
	second, err := repo.AppendProjectionEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, RunID: scope.RunID, Type: "slot.result.persisted", Payload: json.RawMessage(`{}`)})
	require.NoError(t, err)
	require.EqualValues(t, 1, first.Cursor)
	require.Equal(t, first.Cursor, first.ProjectionVersion)
	require.EqualValues(t, 2, second.Cursor)
	require.Equal(t, second.Cursor, second.ProjectionVersion)
	require.NoError(t, repo.UpdateRun(ctx, scope, 0, imageagent.RunMutation{Status: imageagent.RunStatusExecuting, CurrentNode: "execute"}))
	events, err := repo.ListEvents(ctx, scope, 0, 10)
	require.NoError(t, err)
	require.Len(t, events, 3)
	require.EqualValues(t, 3, events[2].Cursor)
	require.Equal(t, events[2].Cursor, events[2].ProjectionVersion)
}

func testAuthorizedAssetCatalogRoundTripsAndCannotBeReplaced(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-catalog", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", RunID: "run-catalog"}
	catalog := imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
		{ID: "source-1", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example/source.png", Width: 1200, Height: 900},
		{ID: "style-1", Type: imageagent.AuthorizedAssetStyle, Label: "Style"},
	}}
	require.NoError(t, repo.SaveAssetCatalog(ctx, scope, catalog))
	require.NoError(t, repo.SaveAssetCatalog(ctx, scope, catalog))
	got, err := repo.GetAssetCatalog(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, catalog, got)
	changed := catalog
	changed.Assets = append([]imageagent.AuthorizedAsset(nil), catalog.Assets...)
	changed.Assets[0].DisplayURL = "https://attacker.example/injected.png"
	require.ErrorIs(t, repo.SaveAssetCatalog(ctx, scope, changed), imageagent.ErrRevisionConflict)
}

func testOnlyManualRunsAreAccepted(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	for _, mode := range []imageagent.RunMode{"", imageagent.RunModeAssisted, imageagent.RunModeAutomatic} {
		run := manualRun("run-"+string(mode), "tenant-a")
		run.Mode = mode
		require.Error(t, repo.CreateRun(ctx, run))
	}
}

func testRunCreateRetriesAreIdempotent(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	run := manualRun("run-1", "tenant-a")
	require.NoError(t, repo.CreateRun(ctx, run))
	require.NoError(t, repo.CreateRun(ctx, run))

	conflictingRun := *run
	conflictingRun.Status = imageagent.RunStatusExecuting
	require.ErrorIs(t, repo.CreateRun(ctx, &conflictingRun), imageagent.ErrRevisionConflict)

	conflictingKey := manualRun("run-2", "tenant-a")
	conflictingKey.IdempotencyKey = run.IdempotencyKey
	require.ErrorIs(t, repo.CreateRun(ctx, conflictingKey), imageagent.ErrRevisionConflict)
}

func testRunJSONRoundTrips(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	run := manualRun("run-1", "tenant-a")
	run.Budget = imageagent.Budget{MaxImages: 7, MaxAgentSteps: 11, MaxModelCalls: 13, MaxRepairAttemptsPerSlot: 2, MaxCostMicros: 17, MaxElapsed: 19 * time.Minute}
	run.Usage = imageagent.BudgetUsage{Images: 3, AgentSteps: 5, ModelCalls: 7, EstimatedCostMicros: 9, Elapsed: 11 * time.Second}
	run.Block = &imageagent.Block{Code: "awaiting_input", Message: "need source", SlotID: "slot-1"}
	require.NoError(t, repo.CreateRun(ctx, run))

	stored, err := repo.GetRun(ctx, imageagent.RunScope{TenantID: run.TenantID, RunID: run.ID})
	require.NoError(t, err)
	require.Equal(t, run.Budget, stored.Budget)
	require.Equal(t, run.Usage, stored.Usage)
	require.Equal(t, run.Block, stored.Block)
}

func testSlotResultRequiresCurrentPlanRevision(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}
	require.NoError(t, repo.AppendPlan(ctx, scope, 0, planRevision(1)))

	result := imageagent.SlotResult{SlotID: "slot-1", Attempt: 1, Status: imageagent.SlotStatusAccepted, CandidateAssetIDs: []string{"asset-1"}}
	require.ErrorIs(t, repo.SaveSlotResult(ctx, scope, 0, result), imageagent.ErrRevisionConflict)
	require.NoError(t, repo.SaveSlotResult(ctx, scope, 1, result))
	require.NoError(t, repo.AppendPlan(ctx, scope, 1, planRevision(2)))
	require.ErrorIs(t, repo.SaveSlotResult(ctx, scope, 1, result), imageagent.ErrRevisionConflict)
}

func testSlotResultRetryAndAttemptOrdering(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}
	require.NoError(t, repo.AppendPlan(ctx, scope, 0, planRevision(1)))

	first := imageagent.SlotResult{SlotID: "slot-1", Attempt: 1, Status: imageagent.SlotStatusAccepted, CandidateAssetIDs: []string{"asset-1"}}
	require.NoError(t, repo.SaveSlotResult(ctx, scope, 1, first))
	require.NoError(t, repo.SaveSlotResult(ctx, scope, 1, first))
	conflicting := first
	conflicting.ErrorCode = "different"
	require.ErrorIs(t, repo.SaveSlotResult(ctx, scope, 1, conflicting), imageagent.ErrRevisionConflict)

	higher := first
	higher.Attempt = 2
	higher.CandidateAssetIDs = []string{"asset-2"}
	require.NoError(t, repo.SaveSlotResult(ctx, scope, 1, higher))
	require.ErrorIs(t, repo.SaveSlotResult(ctx, scope, 1, first), imageagent.ErrRevisionConflict)
}

func testAppendedEventIsVisibleInCursorOrder(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}
	require.NoError(t, repo.AppendEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, RunID: scope.RunID, Type: "run.created", Cursor: 2, ProjectionVersion: 0, Payload: json.RawMessage(`{"run_id":"run-1"}`)}))
	require.NoError(t, repo.AppendEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, RunID: scope.RunID, Type: "plan.requested", Cursor: 3, ProjectionVersion: 0, Payload: json.RawMessage(`{"revision":1}`)}))
	require.ErrorIs(t, repo.AppendEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, RunID: scope.RunID, Type: "retrograde", Cursor: 1}), imageagent.ErrRevisionConflict)
	require.ErrorIs(t, repo.AppendEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, RunID: scope.RunID, Type: "duplicate", Cursor: 3}), imageagent.ErrRevisionConflict)

	events, err := repo.ListEvents(ctx, scope, 2, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "plan.requested", events[0].Type)
	require.EqualValues(t, 3, events[0].Cursor)
}

func testStalePlanRevisionIsRejected(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}
	require.NoError(t, repo.AppendPlan(ctx, scope, 0, planRevision(1)))
	err := repo.AppendPlan(ctx, scope, 0, planRevision(2))
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
}

func testPlanAppendRetriesAreIdempotent(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}
	plan := planRevision(1)
	require.NoError(t, repo.AppendPlan(ctx, scope, 0, plan))
	require.NoError(t, repo.AppendPlan(ctx, scope, 0, plan))

	conflicting := plan
	conflicting.CreatedBy = "another-user"
	require.ErrorIs(t, repo.AppendPlan(ctx, scope, 0, conflicting), imageagent.ErrRevisionConflict)

	conflictingStatus := plan
	conflictingStatus.Slots[0].Status = imageagent.SlotStatusAccepted
	require.ErrorIs(t, repo.AppendPlan(ctx, scope, 0, conflictingStatus), imageagent.ErrRevisionConflict)

	conflictingKey := planRevision(2)
	conflictingKey.IdempotencyKey = plan.IdempotencyKey
	require.ErrorIs(t, repo.AppendPlan(ctx, scope, 1, conflictingKey), imageagent.ErrRevisionConflict)
}

func testCrossTenantRunLookupIsHidden(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))

	_, err := repo.GetRun(ctx, imageagent.RunScope{TenantID: "tenant-b", RunID: "run-1"})
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
}

func testRunMutationAppendsDeterministicProjectionEvent(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}
	require.NoError(t, repo.AppendPlan(ctx, scope, 0, planRevision(1)))

	mutation := imageagent.RunMutation{
		Status:             imageagent.RunStatusExecuting,
		CurrentNode:        "generate",
		ActivePlanRevision: 1,
	}
	require.NoError(t, repo.UpdateRun(ctx, scope, 0, mutation))

	run, err := repo.GetRun(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, imageagent.RunStatusExecuting, run.Status)
	require.Equal(t, "generate", run.CurrentNode)
	require.EqualValues(t, 1, run.ActivePlanRevision)
	require.EqualValues(t, 1, run.Version)

	events, err := repo.ListEvents(ctx, scope, 0, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "run.updated", events[0].Type)
	require.EqualValues(t, 1, events[0].Cursor)
	require.EqualValues(t, 1, events[0].ProjectionVersion)

	wantPayload, err := json.Marshal(mutation)
	require.NoError(t, err)
	require.JSONEq(t, string(wantPayload), string(events[0].Payload))
}

func testAttemptIdentitiesAreIdempotentAndNonAliasing(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	attempt := imageagent.StepAttempt{
		TenantID:       "tenant-a",
		RunID:          "run-1",
		SlotID:         "slot-1",
		Node:           "generate",
		IdempotencyKey: "attempt-1",
		Attempt:        1,
		Outcome:        "succeeded",
	}
	require.NoError(t, repo.AppendAttempt(ctx, attempt))
	require.NoError(t, repo.AppendAttempt(ctx, attempt))

	conflictingNumber := attempt
	conflictingNumber.IdempotencyKey = "attempt-2"
	require.ErrorIs(t, repo.AppendAttempt(ctx, conflictingNumber), imageagent.ErrRevisionConflict)

	conflictingKey := attempt
	conflictingKey.Attempt = 2
	require.ErrorIs(t, repo.AppendAttempt(ctx, conflictingKey), imageagent.ErrRevisionConflict)
}

func manualRun(runID, tenantID string) *imageagent.Run {
	return &imageagent.Run{
		ID:             runID,
		TenantID:       tenantID,
		Mode:           imageagent.RunModeManual,
		IdempotencyKey: "run-key-" + runID,
		Status:         imageagent.RunStatusPlanning,
	}
}

func planRevision(revision int64) imageagent.Plan {
	return imageagent.Plan{
		Revision:          revision,
		IdempotencyKey:    fmt.Sprintf("plan-key-%d", revision),
		SourceAssetIDs:    []string{"source-1"},
		StyleReferenceIDs: []string{"style-1"},
		Slots: []imageagent.Slot{
			{
				ID:                "slot-1",
				Role:              imageagent.SlotRoleScene,
				SourceAssetIDs:    []string{"source-1"},
				StyleReferenceIDs: []string{"style-1"},
				Brief:             "front view",
				IdempotencyKey:    fmt.Sprintf("slot-key-%d", revision),
				Status:            imageagent.SlotStatusPending,
			},
		},
		CreatedBy: "user-1",
	}
}
