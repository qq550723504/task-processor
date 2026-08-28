package store

import (
	"context"
	"fmt"
	"sync"
	"testing"

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
