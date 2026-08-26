package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

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
			testStalePlanRevisionIsRejected(t, tt.new(t))
			testCrossTenantRunLookupIsHidden(t, tt.new(t))
			testRunMutationAppendsDeterministicProjectionEvent(t, tt.new(t))
			testSlotResultRequiresCurrentPlanRevision(t, tt.new(t))
			testAppendedEventIsVisibleInCursorOrder(t, tt.new(t))
			testDuplicateAttemptIsIdempotent(t, tt.new(t))
		})
	}
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
}

func testAppendedEventIsVisibleInCursorOrder(t *testing.T, repo imageagent.Repository) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}
	require.NoError(t, repo.AppendEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, RunID: scope.RunID, Type: "run.created", Cursor: 2, ProjectionVersion: 0, Payload: json.RawMessage(`{"run_id":"run-1"}`)}))
	require.NoError(t, repo.AppendEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, RunID: scope.RunID, Type: "plan.requested", Cursor: 3, ProjectionVersion: 0, Payload: json.RawMessage(`{"revision":1}`)}))

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

func testDuplicateAttemptIsIdempotent(t *testing.T, repo imageagent.Repository) {
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
