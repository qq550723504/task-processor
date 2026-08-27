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
		new  func(t *testing.T) repositoryContract
	}{
		{
			name: "memory",
			new: func(t *testing.T) repositoryContract {
				t.Helper()
				return NewMemoryRepository().(repositoryContract)
			},
		},
		{
			name: "gorm sqlite",
			new: func(t *testing.T) repositoryContract {
				t.Helper()
				db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
				require.NoError(t, err)
				require.NoError(t, AutoMigrate(db))
				return NewGormRepository(db).(repositoryContract)
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
			testMissingCatalogManifestDiffersFromLegitimateEmptyCatalog(t, tt.new(t))
			testProjectionCommitRollbackHidesEventSnapshotAndNormalizedWrites(t, tt.new(t))
			testCombinedPlanAndRunProjectionCommitUsesPreMutationRevision(t, tt.new(t))
			testProjectionCommitRejectsPlanOutsidePersistedCatalog(t, tt.new(t))
			testProjectionCommitRejectsMismatchedSlotAttemptIdentityAtomically(t, tt.new(t))
			testAttemptIdentitiesAreIdempotentAndNonAliasing(t, tt.new(t))
			testInitializationConcurrencyIdentityIncludesMaxConcurrentSlots(t, tt.new(t))
		})
	}
}

func testInitializationConcurrencyIdentityIncludesMaxConcurrentSlots(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	run := manualRun("run-concurrency-identity", "tenant-a")
	run.BusinessTaskID = "task-concurrency-identity"
	run.MaxConcurrentSlots = 2
	plan := planRevision(1)
	input := imageagent.ProjectionInitialization{
		Scope: imageagent.ScopeForRun(*run), Run: *run, Plan: plan,
		Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"},
			{ID: "style-1", Type: imageagent.AuthorizedAssetStyle},
		}},
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: "start:concurrency-identity",
		EventType: "run.initialized", EventPayload: json.RawMessage(`{}`),
	}

	winner, err := repo.InitializeRun(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 2, winner.Run.MaxConcurrentSlots)
	replayed, err := repo.InitializeRun(ctx, input)
	require.NoError(t, err)
	require.Equal(t, winner.Run.MaxConcurrentSlots, replayed.Run.MaxConcurrentSlots)

	conflict := input
	conflict.Run.MaxConcurrentSlots = 3
	conflict.Snapshot.Run = conflict.Run
	_, err = repo.InitializeRun(ctx, conflict)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	stored, err := repo.GetProjection(ctx, input.Scope)
	require.NoError(t, err)
	require.Equal(t, 2, stored.Run.MaxConcurrentSlots)
}

func testProjectionCommitRejectsMismatchedSlotAttemptIdentityAtomically(t *testing.T, repo repositoryContract) {
	t.Helper()
	tests := []struct {
		name   string
		mutate func(*imageagent.SlotProjectionMutation)
	}{
		{name: "mutation revision", mutate: func(m *imageagent.SlotProjectionMutation) { m.PlanRevision = 2 }},
		{name: "attempt revision", mutate: func(m *imageagent.SlotProjectionMutation) { m.Attempt.PlanRevision = 2 }},
		{name: "attempt tenant", mutate: func(m *imageagent.SlotProjectionMutation) { m.Attempt.TenantID = "tenant-b" }},
		{name: "attempt owner", mutate: func(m *imageagent.SlotProjectionMutation) { m.Attempt.OwnerUserID = "user-b" }},
		{name: "attempt run", mutate: func(m *imageagent.SlotProjectionMutation) { m.Attempt.RunID = "other-run" }},
		{name: "attempt slot", mutate: func(m *imageagent.SlotProjectionMutation) { m.Attempt.SlotID = "other-slot" }},
		{name: "attempt number", mutate: func(m *imageagent.SlotProjectionMutation) { m.Attempt.Attempt = 2 }},
		{name: "projection slot", mutate: func(m *imageagent.SlotProjectionMutation) { m.Projection.Slot.ID = "other-slot" }},
		{name: "projection attempt", mutate: func(m *imageagent.SlotProjectionMutation) { m.Projection.Attempt = 2 }},
	}
	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			run := manualRun(fmt.Sprintf("run-slot-identity-%d", index), "tenant-a")
			plan := planRevision(1)
			scope := imageagent.ScopeForRun(*run)
			initial, err := repo.InitializeRun(ctx, imageagent.ProjectionInitialization{
				Scope: scope, Run: *run, Plan: plan,
				Catalog:  imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source-1.png"}, {ID: "style-1", Type: imageagent.AuthorizedAssetStyle, URL: "https://style.example/style-1.png"}}},
				Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: "start", EventType: "run.initialized", EventPayload: json.RawMessage(`{}`),
			})
			require.NoError(t, err)
			updated := initial
			candidate := imageagent.AssetCandidate{AssetID: "candidate-1", URL: "https://generated.example/candidate-1.png", SourceAssetID: "source-1"}
			updated.Slots[0] = imageagent.SlotProjection{Slot: updated.Slots[0].Slot, Attempt: 1, Candidates: []imageagent.AssetCandidate{candidate}}
			updated.Slots[0].Slot.Status = imageagent.SlotStatusAccepted
			mutation := imageagent.SlotProjectionMutation{
				PlanRevision: 1,
				Result:       imageagent.SlotResult{SlotID: "slot-1", Attempt: 1, Status: imageagent.SlotStatusAccepted, CandidateAssetIDs: []string{"candidate-1"}},
				Projection:   updated.Slots[0],
				Attempt:      imageagent.StepAttempt{TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, PlanRevision: 1, SlotID: "slot-1", Attempt: 1, Node: "execute_slot", IdempotencyKey: "attempt-1", Outcome: "accepted"},
			}
			tt.mutate(&mutation)
			_, err = repo.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: "malicious", ExpectedProjectionVersion: initial.ProjectionVersion, Snapshot: updated, EventType: "slot.result.persisted", EventPayload: json.RawMessage(`{}`), SlotMutation: &mutation})
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)

			stored, err := repo.GetProjection(ctx, scope)
			require.NoError(t, err)
			require.EqualValues(t, 1, stored.ProjectionVersion)
			require.Zero(t, stored.Slots[0].Attempt)
			require.Empty(t, stored.Slots[0].Candidates)
			events, err := repo.ListEvents(ctx, scope, 0, 10)
			require.NoError(t, err)
			require.Len(t, events, 1)
		})
	}
}

func testProjectionCommitRejectsPlanOutsidePersistedCatalog(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	run := manualRun("run-malicious-plan", "tenant-a")
	plan := planRevision(1)
	scope := imageagent.ScopeForRun(*run)
	initial, err := repo.InitializeRun(ctx, imageagent.ProjectionInitialization{
		Scope: scope, Run: *run, Plan: plan,
		Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"},
			{ID: "style-1", Type: imageagent.AuthorizedAssetStyle},
		}},
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: "start:malicious-plan",
		EventType: "run.initialized", EventPayload: json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	maliciousPlan := planRevision(2)
	maliciousPlan.ParentRevision = 1
	maliciousPlan.SourceAssetIDs = []string{"source-attacker"}
	maliciousPlan.Slots[0].SourceAssetIDs = []string{"source-attacker"}
	malicious := initial
	malicious.Plan = maliciousPlan
	malicious.Run.ActivePlanRevision = 2
	malicious.Run.Version++
	malicious.Slots = []imageagent.SlotProjection{{Slot: maliciousPlan.Slots[0]}}
	_, err = repo.CommitProjection(ctx, imageagent.ProjectionCommit{
		Scope: scope, CommitID: "plan:malicious", ExpectedProjectionVersion: initial.ProjectionVersion,
		ExpectedRunVersion: initial.Run.Version, Snapshot: malicious, EventType: "plan.replaced", EventPayload: json.RawMessage(`{}`),
		RunMutation:  &imageagent.RunMutation{Status: initial.Run.Status, CurrentNode: initial.Run.CurrentNode, ActivePlanRevision: 2},
		PlanMutation: &imageagent.PlanProjectionMutation{ExpectedActiveRevision: 1, Plan: maliciousPlan},
	})
	require.Error(t, err)

	stored, getErr := repo.GetProjection(ctx, scope)
	require.NoError(t, getErr)
	require.EqualValues(t, 1, stored.ProjectionVersion)
	require.EqualValues(t, 1, stored.Plan.Revision)
	require.EqualValues(t, 1, stored.Run.ActivePlanRevision)
	events, eventsErr := repo.ListEvents(ctx, scope, 0, 10)
	require.NoError(t, eventsErr)
	require.Len(t, events, 1)
}

func testCombinedPlanAndRunProjectionCommitUsesPreMutationRevision(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	run := manualRun("run-replace-atomic", "tenant-a")
	run.Status = imageagent.RunStatusBlocked
	run.CurrentNode = "blocked"
	run.Block = &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"}
	run.Version = 1
	plan1 := planRevision(1)
	plan2 := planRevision(2)
	plan2.ParentRevision = 1
	scope := imageagent.ScopeForRun(*run)
	initial, err := repo.InitializeRun(ctx, imageagent.ProjectionInitialization{
		Scope: scope, Run: *run, Plan: plan1,
		Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"},
			{ID: "style-1", Type: imageagent.AuthorizedAssetStyle},
		}},
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan1}, CommitID: "start:replace-atomic",
		EventType: "run.initialized", EventPayload: json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	next := initial
	next.Run.Status = imageagent.RunStatusExecuting
	next.Run.CurrentNode = "execute_slots"
	next.Run.ActivePlanRevision = 2
	next.Run.Block = nil
	next.Run.Version = 2
	next.Plan = plan2
	next.Slots = []imageagent.SlotProjection{{Slot: plan2.Slots[0]}}
	commit := imageagent.ProjectionCommit{
		Scope: scope, CommitID: "plan:" + plan2.IdempotencyKey,
		ExpectedProjectionVersion: initial.ProjectionVersion,
		ExpectedRunVersion:        initial.Run.Version,
		Snapshot:                  next, EventType: "plan.replaced", EventPayload: json.RawMessage(`{"revision":2}`),
		RunMutation:  &imageagent.RunMutation{Status: imageagent.RunStatusExecuting, CurrentNode: "execute_slots", ActivePlanRevision: 2},
		PlanMutation: &imageagent.PlanProjectionMutation{ExpectedActiveRevision: 1, Plan: plan2},
	}
	stored, err := repo.CommitProjection(ctx, commit)
	require.NoError(t, err, "all preconditions must be checked against revision 1 before either mutation is applied")
	require.EqualValues(t, 2, stored.Plan.Revision)
	require.EqualValues(t, 2, stored.Run.ActivePlanRevision)
	require.EqualValues(t, 2, stored.Run.Version)
	require.EqualValues(t, 2, stored.ProjectionVersion)
	require.Equal(t, imageagent.RunStatusExecuting, stored.Run.Status)
	require.Nil(t, stored.Run.Block)

	retried, err := repo.CommitProjection(ctx, commit)
	require.NoError(t, err)
	require.Equal(t, stored, retried, "exact CommitID retry returns the stored acknowledgement")
	conflict := commit
	conflict.Snapshot.Run.CurrentNode = "different"
	_, err = repo.CommitProjection(ctx, conflict)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)

	normalizedRun, err := repo.GetRun(ctx, scope)
	require.NoError(t, err)
	require.EqualValues(t, 2, normalizedRun.ActivePlanRevision)
	require.EqualValues(t, 2, normalizedRun.Version)
	events, err := repo.ListEvents(ctx, scope, 0, 10)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Equal(t, int64(2), events[1].Cursor)
}

// repositoryContract keeps Task 1 compatibility storage methods under test
// without exposing them through the production imageagent.Repository. Public
// code can mutate run/plan/slot/event state only through CommitProjection.
type repositoryContract interface {
	imageagent.Repository
	CreateRun(context.Context, *imageagent.Run) error
	GetRun(context.Context, imageagent.RunScope) (*imageagent.Run, error)
	UpdateRun(context.Context, imageagent.RunScope, int64, imageagent.RunMutation) error
	AppendPlan(context.Context, imageagent.RunScope, int64, imageagent.Plan) error
	SaveSlotResult(context.Context, imageagent.RunScope, int64, imageagent.SlotResult) error
	AppendAttempt(context.Context, imageagent.StepAttempt) error
	AppendEvent(context.Context, imageagent.RunEvent) error
	AppendProjectionEvent(context.Context, imageagent.RunEvent) (imageagent.RunEvent, error)
	SaveAssetCatalog(context.Context, imageagent.RunScope, imageagent.AssetCatalog) error
}

func testProjectionCommitRollbackHidesEventSnapshotAndNormalizedWrites(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	run := manualRun("run-projection-rollback", "tenant-a")
	plan := planRevision(1)
	scope := imageagent.ScopeForRun(*run)
	catalog := imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
		{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source-1.png"},
		{ID: "style-1", Type: imageagent.AuthorizedAssetStyle},
	}}
	initial, err := repo.InitializeRun(ctx, imageagent.ProjectionInitialization{
		Scope: scope, Run: *run, Plan: plan, Catalog: catalog,
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan},
		CommitID: "start:rollback", EventType: "run.created", EventPayload: json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	tamperedCatalog := initial
	tamperedCatalog.AssetCatalog.Assets[0].URL = "https://attacker.example/rebind.png"
	_, err = repo.CommitProjection(ctx, imageagent.ProjectionCommit{
		Scope: scope, CommitID: "catalog:tamper", ExpectedProjectionVersion: initial.ProjectionVersion,
		Snapshot: tamperedCatalog, EventType: "catalog.tampered", EventPayload: json.RawMessage(`{}`),
	})
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict, "a projection commit cannot replace the immutable run catalog")

	bad := initial
	bad.Slots[0] = imageagent.SlotProjection{
		Slot:       bad.Slots[0].Slot,
		Attempt:    1,
		Candidates: []imageagent.AssetCandidate{{AssetID: "candidate-unsafe", URL: "javascript:alert(1)"}},
	}
	bad.Slots[0].Slot.Status = imageagent.SlotStatusAccepted
	attempt := imageagent.StepAttempt{
		TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID,
		PlanRevision: 1, SlotID: "slot-1", Node: "execute_slot", IdempotencyKey: "attempt-unsafe", Attempt: 1, Outcome: "accepted",
	}
	_, err = repo.CommitProjection(ctx, imageagent.ProjectionCommit{
		Scope: scope, CommitID: "slot:unsafe", ExpectedProjectionVersion: initial.ProjectionVersion,
		Snapshot: bad, EventType: "slot.result.persisted", EventPayload: json.RawMessage(`{}`),
		SlotMutation: &imageagent.SlotProjectionMutation{
			PlanRevision: 1,
			Result:       imageagent.SlotResult{SlotID: "slot-1", Attempt: 1, Status: imageagent.SlotStatusAccepted, CandidateAssetIDs: []string{"candidate-unsafe"}},
			Projection:   bad.Slots[0], Attempt: attempt,
		},
	})
	require.Error(t, err)

	stored, err := repo.GetProjection(ctx, scope)
	require.NoError(t, err)
	require.EqualValues(t, 1, stored.ProjectionVersion)
	require.Empty(t, stored.Slots[0].Candidates)
	events, err := repo.ListEvents(ctx, scope, 0, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)

	attempt.Outcome = "independent-retry"
	require.NoError(t, repo.AppendAttempt(ctx, attempt), "failed projection transaction must not leave an attempt behind")
}

func testMissingCatalogManifestDiffersFromLegitimateEmptyCatalog(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-catalog-missing", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-catalog-missing"}
	_, err := repo.GetAssetCatalog(ctx, scope)
	require.ErrorIs(t, err, imageagent.ErrCatalogSnapshotMissing)
	require.NoError(t, repo.SaveAssetCatalog(ctx, scope, imageagent.AssetCatalog{}))
	got, err := repo.GetAssetCatalog(ctx, scope)
	require.NoError(t, err)
	require.Empty(t, got.Assets)
}

func testProjectionEventsAllocateOneDurableCursorPerChange(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-cursor", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-cursor"}
	first, err := repo.AppendProjectionEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, Type: "slot.result.persisted", Payload: json.RawMessage(`{}`)})
	require.NoError(t, err)
	second, err := repo.AppendProjectionEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, Type: "slot.result.persisted", Payload: json.RawMessage(`{}`)})
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

func testAuthorizedAssetCatalogRoundTripsAndCannotBeReplaced(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-catalog", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-catalog"}
	catalog := imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
		{ID: "source-1", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example/source.png", Width: 1200, Height: 900},
		{ID: "style-1", Type: imageagent.AuthorizedAssetStyle, Label: "Style"},
	}}
	require.NoError(t, repo.SaveAssetCatalog(ctx, scope, catalog))
	require.NoError(t, repo.SaveAssetCatalog(ctx, scope, catalog))
	got, err := repo.GetAssetCatalog(ctx, scope)
	require.NoError(t, err)
	normalized, err := imageagent.NormalizeAssetCatalog(catalog)
	require.NoError(t, err)
	require.Equal(t, normalized.Manifest.Version, got.Manifest.Version)
	require.Equal(t, normalized.Manifest.Hash, got.Manifest.Hash)
	require.Equal(t, normalized.Assets, got.Assets)
	changed := catalog
	changed.Assets = append([]imageagent.AuthorizedAsset(nil), catalog.Assets...)
	changed.Assets[0].DisplayURL = "https://attacker.example/injected.png"
	require.ErrorIs(t, repo.SaveAssetCatalog(ctx, scope, changed), imageagent.ErrRevisionConflict)
}

func testOnlyManualRunsAreAccepted(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	for _, mode := range []imageagent.RunMode{"", imageagent.RunModeAssisted, imageagent.RunModeAutomatic} {
		run := manualRun("run-"+string(mode), "tenant-a")
		run.Mode = mode
		require.Error(t, repo.CreateRun(ctx, run))
	}
}

func testRunCreateRetriesAreIdempotent(t *testing.T, repo repositoryContract) {
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

func testRunJSONRoundTrips(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	run := manualRun("run-1", "tenant-a")
	run.Budget = imageagent.Budget{MaxImages: 7, MaxAgentSteps: 11, MaxModelCalls: 13, MaxRepairAttemptsPerSlot: 2, MaxCostMicros: 17, MaxElapsed: 19 * time.Minute}
	run.Usage = imageagent.BudgetUsage{Images: 3, AgentSteps: 5, ModelCalls: 7, EstimatedCostMicros: 9, Elapsed: 11 * time.Second}
	run.Block = &imageagent.Block{Code: "awaiting_input", Message: "need source", SlotID: "slot-1"}
	require.NoError(t, repo.CreateRun(ctx, run))

	stored, err := repo.GetRun(ctx, imageagent.RunScope{TenantID: run.TenantID, OwnerUserID: run.UserID, RunID: run.ID})
	require.NoError(t, err)
	require.Equal(t, run.Budget, stored.Budget)
	require.Equal(t, run.Usage, stored.Usage)
	require.Equal(t, run.Block, stored.Block)
}

func testSlotResultRequiresCurrentPlanRevision(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-1"}
	require.NoError(t, repo.AppendPlan(ctx, scope, 0, planRevision(1)))

	result := imageagent.SlotResult{SlotID: "slot-1", Attempt: 1, Status: imageagent.SlotStatusAccepted, CandidateAssetIDs: []string{"asset-1"}}
	require.ErrorIs(t, repo.SaveSlotResult(ctx, scope, 0, result), imageagent.ErrRevisionConflict)
	require.NoError(t, repo.SaveSlotResult(ctx, scope, 1, result))
	require.NoError(t, repo.AppendPlan(ctx, scope, 1, planRevision(2)))
	require.ErrorIs(t, repo.SaveSlotResult(ctx, scope, 1, result), imageagent.ErrRevisionConflict)
}

func testSlotResultRetryAndAttemptOrdering(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-1"}
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

func testAppendedEventIsVisibleInCursorOrder(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-1"}
	require.NoError(t, repo.AppendEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, Type: "run.created", Cursor: 2, ProjectionVersion: 0, Payload: json.RawMessage(`{"run_id":"run-1"}`)}))
	require.NoError(t, repo.AppendEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, Type: "plan.requested", Cursor: 3, ProjectionVersion: 0, Payload: json.RawMessage(`{"revision":1}`)}))
	require.ErrorIs(t, repo.AppendEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, Type: "retrograde", Cursor: 1}), imageagent.ErrRevisionConflict)
	require.ErrorIs(t, repo.AppendEvent(ctx, imageagent.RunEvent{TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, Type: "duplicate", Cursor: 3}), imageagent.ErrRevisionConflict)

	events, err := repo.ListEvents(ctx, scope, 2, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "plan.requested", events[0].Type)
	require.EqualValues(t, 3, events[0].Cursor)
}

func testStalePlanRevisionIsRejected(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-1"}
	require.NoError(t, repo.AppendPlan(ctx, scope, 0, planRevision(1)))
	err := repo.AppendPlan(ctx, scope, 0, planRevision(2))
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
}

func testPlanAppendRetriesAreIdempotent(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-1"}
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

func testCrossTenantRunLookupIsHidden(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	run := manualRun("run-1", "tenant-a")
	run.BusinessTaskID = "task-1"
	plan := planRevision(1)
	_, err := repo.InitializeRun(ctx, imageagent.ProjectionInitialization{
		Scope: imageagent.ScopeForRun(*run), Run: *run, Plan: plan,
		Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"},
			{ID: "style-1", Type: imageagent.AuthorizedAssetStyle},
		}},
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: "start:owner", EventType: "run.initialized", EventPayload: json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	_, err = repo.GetRun(ctx, imageagent.RunScope{TenantID: "tenant-b", OwnerUserID: "user-1", RunID: "run-1"})
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	wrongOwner := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-2", RunID: "run-1"}
	_, err = repo.GetRun(ctx, wrongOwner)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	_, err = repo.GetProjection(ctx, wrongOwner)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	_, err = repo.GetAssetCatalog(ctx, wrongOwner)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	_, err = repo.ListEvents(ctx, wrongOwner, 0, 10)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
}

func testRunMutationAppendsDeterministicProjectionEvent(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-1"}
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

func testAttemptIdentitiesAreIdempotentAndNonAliasing(t *testing.T, repo repositoryContract) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
	attempt := imageagent.StepAttempt{
		TenantID:       "tenant-a",
		OwnerUserID:    "user-1",
		RunID:          "run-1",
		PlanRevision:   1,
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
		UserID:         "user-1",
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
