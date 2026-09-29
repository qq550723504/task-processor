package store

import (
	"context"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"sync/atomic"
	"task-processor/internal/aicapability"
	"testing"
	"time"
)

func TestModelPointTerminalCannotChangeFrozenTariffOrDispatchIdentity(t *testing.T) {
	recorder := NewGormInvocationRecorder(newInvocationLedgerDB(t))
	now := time.Now().UTC().Truncate(time.Microsecond)
	original := aicapability.InvocationRecord{InvocationID: "priced-invocation", TenantID: "org", UserID: "actor", MemberID: "member", AgentRunID: "run", InputHash: "input", Operation: aicapability.OperationProductAgentDecision, Outcome: aicapability.InvocationDispatched, StartedAt: now, ProviderID: "grsai", ModelID: "gemini", MaximumPromptTokens: 100, MaximumCompletionTokens: 50, PointTariff: aicapability.ModelPointTariff{PriceVersion: "synthetic-v1", InputPointsPerMillionTokens: 2000000, OutputPointsPerMillionTokens: 3000000}}
	won, err := recorder.ClaimInvocation(context.Background(), original)
	require.NoError(t, err)
	require.True(t, won)
	changedDispatch := original
	changedDispatch.PointTariff.OutputPointsPerMillionTokens++
	_, err = recorder.ClaimInvocation(context.Background(), changedDispatch)
	require.Error(t, err)
	terminal := original
	terminal.Outcome = aicapability.InvocationSucceeded
	terminal.FinishedAt = now.Add(time.Second)
	terminal.UsageKnown = true
	terminal.PromptTokens = 2
	terminal.CompletionTokens = 1
	terminal.TotalTokens = 3
	unclaimed := terminal
	unclaimed.InvocationID = "unclaimed-terminal"
	require.Error(t, recorder.RecordInvocation(context.Background(), unclaimed))
	for _, mutate := range []func(*aicapability.InvocationRecord){func(r *aicapability.InvocationRecord) { r.PointTariff.InputPointsPerMillionTokens++ }, func(r *aicapability.InvocationRecord) { r.MaximumPromptTokens++ }, func(r *aicapability.InvocationRecord) { r.ModelID = "other-model" }} {
		changed := terminal
		mutate(&changed)
		require.Error(t, recorder.RecordInvocation(context.Background(), changed))
	}
	require.NoError(t, recorder.RecordInvocation(context.Background(), terminal))
	require.NoError(t, recorder.RecordInvocation(context.Background(), terminal))
	changed := terminal
	changed.PromptTokens++
	changed.TotalTokens++
	require.Error(t, recorder.RecordInvocation(context.Background(), changed))
	found, ok, err := recorder.FindInvocation(context.Background(), "org", "member", original.InvocationID, "input")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, original.PointTariff, found.PointTariff)
}

func TestModelPointConcurrentTerminalUsesOneImmutableWinner(t *testing.T) {
	db := newInvocationLedgerDB(t)
	recorder := NewGormInvocationRecorder(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	original := aicapability.InvocationRecord{InvocationID: "concurrent", TenantID: "org", UserID: "actor", MemberID: "member", AgentRunID: "run", InputHash: "input", Operation: aicapability.OperationProductAgentDecision, Outcome: aicapability.InvocationDispatched, StartedAt: now, MaximumPromptTokens: 100, MaximumCompletionTokens: 50, PointTariff: aicapability.ModelPointTariff{PriceVersion: "synthetic", InputPointsPerMillionTokens: 1000000, OutputPointsPerMillionTokens: 1000000}}
	_, err := recorder.ClaimInvocation(context.Background(), original)
	require.NoError(t, err)
	var arrivals atomic.Int32
	gate := make(chan struct{})
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("terminal_test_barrier", func(tx *gorm.DB) {
		if row, ok := tx.Statement.Dest.(*invocationRow); ok && row.InvocationID == original.InvocationID && row.Outcome == string(aicapability.InvocationDispatched) {
			if arrivals.Add(1) == 2 {
				close(gate)
			}
			select {
			case <-gate:
			case <-tx.Statement.Context.Done():
			}
		}
	}))
	t.Cleanup(func() { db.Callback().Query().Remove("terminal_test_barrier") })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results := make(chan error, 2)
	for _, tokens := range []int{3, 7} {
		terminal := original
		terminal.Outcome = aicapability.InvocationSucceeded
		terminal.FinishedAt = now.Add(time.Second)
		terminal.UsageKnown = true
		terminal.PromptTokens, terminal.TotalTokens = tokens, tokens
		go func() { results <- recorder.RecordInvocation(ctx, terminal) }()
	}
	a, b := <-results, <-results
	require.True(t, (a == nil) != (b == nil), "exactly one terminal writer wins: %v / %v", a, b)
	winner, found, err := recorder.FindInvocation(ctx, "org", "member", "concurrent", "input")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, aicapability.InvocationSucceeded, winner.Outcome)
	require.Contains(t, []int{3, 7}, winner.TotalTokens)
	require.NoError(t, recorder.RecordInvocation(ctx, winner))
}
