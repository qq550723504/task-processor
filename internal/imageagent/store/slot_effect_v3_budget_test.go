package store

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/imageagent"
)

func TestSlotEffectV3BudgetConcurrentLastUnitHasOneOwner(t *testing.T) {
	repository := NewGormRepository(newConcurrentSQLite(t))
	scope, policy := initializeBudgetedSlotEffectRun(t, repository, "run-budget-concurrent")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	var group sync.WaitGroup
	var mu sync.Mutex
	claimedCount := 0
	for attempt := 1; attempt <= 8; attempt++ {
		attempt := attempt
		group.Add(1)
		go func() {
			defer group.Done()
			_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), budgetedV3Reservation(scope, policy, attempt, fmt.Sprintf("quote-%d", attempt)))
			if err != nil {
				require.ErrorIs(t, err, imageagent.ErrBudgetExceeded)
				return
			}
			if claimed {
				mu.Lock()
				claimedCount++
				mu.Unlock()
			}
		}()
	}
	group.Wait()
	require.Equal(t, 1, claimedCount)
}

func TestBudgetProjectionExposesCommittedAndElapsedUsageOnly(t *testing.T) {
	now := time.Date(2026, 8, 28, 1, 2, 3, 0, time.UTC)
	repository := NewMemoryRepositoryWithClock(func() time.Time { return now })
	scope, policy := initializeBudgetedSlotEffectRun(t, repository, "run-budget-projection")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	reservation := budgetedV3Reservation(scope, policy, 1, "quote-projection")
	_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err)
	require.True(t, claimed)

	before, err := repository.GetProjection(context.Background(), scope)
	require.NoError(t, err)
	require.Zero(t, before.Run.Usage.Images, "reserved usage must stay internal")
	now = now.Add(3 * time.Second)
	_, err = effects.SettleSlotProviderV3(context.Background(), reservation, imageagent.SlotUsageReceipt{Actual: imageagent.UsageVector{Images: 1, AgentSteps: 1}, CostBasis: imageagent.UsageCostReservedUpperBound})
	require.NoError(t, err)
	after, err := repository.GetProjection(context.Background(), scope)
	require.NoError(t, err)
	require.Equal(t, 1, after.Run.Usage.Images)
	require.Equal(t, 3*time.Second, after.Run.Usage.Elapsed)
	next := after
	next.Run.Status = imageagent.RunStatusExecuting
	next.Run.CurrentNode = "execute_slots"
	next.Run.Version++
	committed, err := repository.CommitProjection(context.Background(), imageagent.ProjectionCommit{
		Scope: scope, CommitID: "run:after-budget-settlement", ExpectedProjectionVersion: after.ProjectionVersion,
		Snapshot: next, EventType: "run.updated", EventPayload: []byte(`{}`), ExpectedRunVersion: after.Run.Version,
		RunMutation: &imageagent.RunMutation{Status: next.Run.Status, CurrentNode: next.Run.CurrentNode, ActivePlanRevision: next.Run.ActivePlanRevision},
	})
	require.NoError(t, err)
	require.Equal(t, 1, committed.Run.Usage.Images, "projection commits must not erase authoritative usage")
}

func TestSlotEffectV3BudgetReservationLifecycle(t *testing.T) {
	factories := []struct {
		name string
		new  func(*testing.T) imageagent.Repository
	}{
		{"memory", func(*testing.T) imageagent.Repository { return NewMemoryRepository() }},
		{"gorm", func(t *testing.T) imageagent.Repository { return NewGormRepository(newConcurrentSQLite(t)) }},
	}
	for _, factory := range factories {
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			repository := factory.new(t)
			scope, policy := initializeBudgetedSlotEffectRun(t, repository, "run-budget-"+factory.name)
			effects := repository.(imageagent.SlotExternalEffectV3Repository)
			first := budgetedV3Reservation(scope, policy, 1, "quote-1")

			attempt, claimed, err := effects.ReserveSlotProviderV3(ctx, first)
			require.NoError(t, err)
			require.True(t, claimed)
			require.Equal(t, imageagent.SlotBudgetReserved, attempt.BudgetStatus)

			_, replayClaimed, err := effects.ReserveSlotProviderV3(ctx, first)
			require.NoError(t, err)
			require.False(t, replayClaimed, "idempotent replay must not reserve twice")

			mismatched := first
			mismatched.Quote.Fingerprint = "different-quote"
			_, _, err = effects.ReserveSlotProviderV3(ctx, mismatched)
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)

			second := budgetedV3Reservation(scope, policy, 2, "quote-2")
			_, _, err = effects.ReserveSlotProviderV3(ctx, second)
			require.ErrorIs(t, err, imageagent.ErrBudgetExceeded)

			receipt := imageagent.SlotUsageReceipt{Actual: imageagent.UsageVector{Images: 1, AgentSteps: 1}, CostBasis: imageagent.UsageCostReservedUpperBound}
			attempt, err = effects.SettleSlotProviderV3(ctx, first, receipt)
			require.NoError(t, err)
			require.Equal(t, imageagent.SlotBudgetCommitted, attempt.BudgetStatus)
			_, err = effects.SettleSlotProviderV3(ctx, first, receipt)
			require.NoError(t, err)

			projection, err := repository.GetProjection(ctx, scope)
			require.NoError(t, err)
			require.Equal(t, 1, projection.Run.Usage.Images)
			require.Equal(t, 1, projection.Run.Usage.AgentSteps)
			next := projection
			next.Run.Status = imageagent.RunStatusExecuting
			next.Run.CurrentNode = "execute_slots"
			next.Run.Version++
			committedProjection, err := repository.CommitProjection(ctx, imageagent.ProjectionCommit{
				Scope: scope, CommitID: "run:after-budget-settlement", ExpectedProjectionVersion: projection.ProjectionVersion,
				Snapshot: next, EventType: "run.updated", EventPayload: []byte(`{}`), ExpectedRunVersion: projection.Run.Version,
				RunMutation: &imageagent.RunMutation{Status: next.Run.Status, CurrentNode: next.Run.CurrentNode, ActivePlanRevision: next.Run.ActivePlanRevision},
			})
			require.NoError(t, err)
			require.Equal(t, 1, committedProjection.Run.Usage.Images)
		})
	}
}

func TestSlotEffectV3BudgetReleaseAndUnknownAdmission(t *testing.T) {
	for _, factory := range []struct {
		name string
		new  func(*testing.T) imageagent.Repository
	}{{"memory", func(*testing.T) imageagent.Repository { return NewMemoryRepository() }}, {"gorm", func(t *testing.T) imageagent.Repository { return NewGormRepository(newConcurrentSQLite(t)) }}} {
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			repository := factory.new(t)
			scope, policy := initializeBudgetedSlotEffectRun(t, repository, "run-budget-transition-"+factory.name)
			effects := repository.(imageagent.SlotExternalEffectV3Repository)
			first := budgetedV3Reservation(scope, policy, 1, "quote-release")
			_, claimed, err := effects.ReserveSlotProviderV3(ctx, first)
			require.NoError(t, err)
			require.True(t, claimed)
			_, err = effects.ReleaseSlotProviderBudgetV3(ctx, first)
			require.NoError(t, err)

			second := budgetedV3Reservation(scope, policy, 2, "quote-unknown")
			_, claimed, err = effects.ReserveSlotProviderV3(ctx, second)
			require.NoError(t, err)
			require.True(t, claimed)
			attempt, err := effects.MarkSlotProviderBudgetUnknownV3(ctx, second)
			require.NoError(t, err)
			require.Equal(t, imageagent.SlotBudgetUnknown, attempt.BudgetStatus)

			third := budgetedV3Reservation(scope, policy, 3, "quote-denied")
			_, _, err = effects.ReserveSlotProviderV3(ctx, third)
			require.ErrorIs(t, err, imageagent.ErrBudgetExceeded)
		})
	}
}

func TestSlotEffectV3ProviderNotDispatchedReclaimsOnlyAfterProvenNoEffect(t *testing.T) {
	for _, factory := range []struct {
		name string
		new  func(*testing.T) imageagent.Repository
	}{{"memory", func(*testing.T) imageagent.Repository { return NewMemoryRepository() }}, {"gorm", func(t *testing.T) imageagent.Repository { return NewGormRepository(newConcurrentSQLite(t)) }}} {
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			repository := factory.new(t)
			scope, policy := initializeBudgetedSlotEffectRun(t, repository, "run-provider-not-dispatched-"+factory.name)
			effects := repository.(imageagent.SlotExternalEffectV3Repository)
			reservation := budgetedV3Reservation(scope, policy, 1, "quote-not-dispatched")
			_, claimed, err := effects.ReserveSlotProviderV3(ctx, reservation)
			require.NoError(t, err)
			require.True(t, claimed)

			effect, err := effects.RecordSlotProviderNotDispatchedV3(ctx, reservation)
			require.NoError(t, err)
			require.Equal(t, imageagent.SlotEffectV3ProviderNotDispatched, effect.Phase)
			require.Equal(t, imageagent.SlotBudgetReleased, effect.BudgetStatus)

			effect, claimed, err = effects.ReserveSlotProviderV3(ctx, reservation)
			require.NoError(t, err)
			require.True(t, claimed)
			require.Equal(t, imageagent.SlotEffectV3ProviderClaimed, effect.Phase)
			require.Equal(t, imageagent.SlotBudgetReserved, effect.BudgetStatus)
		})
	}
}

func TestSlotEffectV3PolicyDriftOnlyBlocksAdditionalBudgetAdmission(t *testing.T) {
	factories := []struct {
		name string
		new  func(*testing.T) imageagent.Repository
	}{
		{"memory", func(*testing.T) imageagent.Repository { return NewMemoryRepository() }},
		{"gorm", func(t *testing.T) imageagent.Repository { return NewGormRepository(newConcurrentSQLite(t)) }},
	}
	for _, factory := range factories {
		factory := factory
		t.Run(factory.name, func(t *testing.T) {
			tests := []struct {
				name string
				run  func(*testing.T, imageagent.SlotExternalEffectV3Repository, imageagent.SlotEffectV3Reservation, imageagent.SlotEffectV3Reservation)
			}{
				{name: "reserve_noop", run: func(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, reservation, drifted imageagent.SlotEffectV3Reservation) {
					_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), drifted)
					require.NoError(t, err)
					require.False(t, claimed)
				}},
				{name: "not_dispatched_and_redispatch", run: func(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, reservation, drifted imageagent.SlotEffectV3Reservation) {
					attempt, err := effects.RecordSlotProviderNotDispatchedV3(context.Background(), drifted)
					require.NoError(t, err)
					require.Equal(t, imageagent.SlotEffectV3ProviderNotDispatched, attempt.Phase)
					_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), drifted)
					require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
					require.False(t, claimed)
					_, claimed, err = effects.ReserveSlotProviderV3(context.Background(), reservation)
					require.NoError(t, err)
					require.True(t, claimed)
				}},
				{name: "settle", run: func(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, _, drifted imageagent.SlotEffectV3Reservation) {
					receipt := imageagent.SlotUsageReceipt{Actual: imageagent.UsageVector{Images: 1, AgentSteps: 1}, CostBasis: imageagent.UsageCostReservedUpperBound}
					attempt, err := effects.SettleSlotProviderV3(context.Background(), drifted, receipt)
					require.NoError(t, err)
					require.Equal(t, imageagent.SlotBudgetCommitted, attempt.BudgetStatus)
					_, err = effects.SettleSlotProviderV3(context.Background(), drifted, receipt)
					require.NoError(t, err)
				}},
				{name: "release_and_reacquire", run: func(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, reservation, drifted imageagent.SlotEffectV3Reservation) {
					attempt, err := effects.ReleaseSlotProviderBudgetV3(context.Background(), drifted)
					require.NoError(t, err)
					require.Equal(t, imageagent.SlotBudgetReleased, attempt.BudgetStatus)
					_, err = effects.ReleaseSlotProviderBudgetV3(context.Background(), drifted)
					require.NoError(t, err)
					_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), drifted)
					require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
					require.False(t, claimed)
					_, claimed, err = effects.ReserveSlotProviderV3(context.Background(), reservation)
					require.NoError(t, err)
					require.True(t, claimed)
				}},
				{name: "unknown", run: func(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, _, drifted imageagent.SlotEffectV3Reservation) {
					attempt, err := effects.MarkSlotProviderBudgetUnknownV3(context.Background(), drifted)
					require.NoError(t, err)
					require.Equal(t, imageagent.SlotBudgetUnknown, attempt.BudgetStatus)
					_, err = effects.MarkSlotProviderBudgetUnknownV3(context.Background(), drifted)
					require.NoError(t, err)
				}},
				{name: "staging", run: func(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, _, drifted imageagent.SlotEffectV3Reservation) {
					attempt, err := effects.PrepareSlotStagingV3(context.Background(), drifted, v3StagingManifest())
					require.NoError(t, err)
					require.Equal(t, imageagent.SlotEffectV3StagingPrepared, attempt.Phase)
					attempt, err = effects.CommitSlotStagedV3(context.Background(), drifted, attempt.StagingManifestFingerprint)
					require.NoError(t, err)
					require.Equal(t, imageagent.SlotEffectV3ArtifactStaged, attempt.Phase)
				}},
			}
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					repository := factory.new(t)
					scope, policy := initializeBudgetedSlotEffectRun(t, repository, "run-policy-drift-"+factory.name+"-"+test.name)
					effects := repository.(imageagent.SlotExternalEffectV3Repository)
					reservation := budgetedV3Reservation(scope, policy, 1, "quote-policy-drift")
					_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
					require.NoError(t, err)
					require.True(t, claimed)
					drifted := reservation
					drifted.Policy.Images.Value++
					test.run(t, effects, reservation, drifted)
				})
			}
		})
	}
}

func initializeBudgetedSlotEffectRun(t *testing.T, repository imageagent.Repository, runID string) (imageagent.RunScope, imageagent.BudgetPolicy) {
	t.Helper()
	run := manualRun(runID, "tenant-a")
	run.BusinessTaskID = "task-" + runID
	run.Budget = imageagent.Budget{MaxImages: 1, EnabledLimits: imageagent.BudgetLimitImages}
	policy, err := run.Budget.Policy()
	require.NoError(t, err)
	plan := planRevision(1)
	scope := imageagent.ScopeForRun(*run)
	_, err = repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{
		Scope: scope, Run: *run, Plan: plan,
		Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"},
			{ID: "style-1", Type: imageagent.AuthorizedAssetStyle, URL: "https://style.example/style.png"},
		}},
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: fmt.Sprintf("start:%s", runID), EventType: "run.initialized", EventPayload: []byte(`{}`),
	})
	require.NoError(t, err)
	return scope, policy
}

func budgetedV3Reservation(scope imageagent.RunScope, policy imageagent.BudgetPolicy, attempt int, quoteFingerprint string) imageagent.SlotEffectV3Reservation {
	operation := imageagent.SlotUsageOperation{Name: "render_scene", Fingerprint: "operation-" + quoteFingerprint, MaximumOutputs: 1, Maximum: imageagent.UsageVector{Images: 1, AgentSteps: 1}}
	return imageagent.SlotEffectV3Reservation{
		Identity:       imageagent.SlotExternalEffectIdentity{RunScope: scope, PlanRevision: 1, SlotID: "slot-1", Attempt: attempt},
		IdempotencyKey: fmt.Sprintf("attempt-%d", attempt), InputFingerprint: fmt.Sprintf("input-%d", attempt), Policy: policy,
		Quote: imageagent.SlotUsageQuote{Maximum: operation.Maximum, Operations: []imageagent.SlotUsageOperation{operation}, Fingerprint: quoteFingerprint},
	}
}
