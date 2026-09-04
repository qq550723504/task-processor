package effectpolicy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/imageagent"
)

func TestReviewUsageReservationIsIndependentAndIdempotent(t *testing.T) {
	identity := imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: "tenant", OwnerUserID: "owner", RunID: "run"}, PlanRevision: 1, SlotID: "slot", Attempt: 1}
	quote := imageagent.SlotUsageQuote{Maximum: imageagent.UsageVector{Images: 1, AgentSteps: 1, ModelCalls: 1, CostMicros: 10}, Operations: []imageagent.SlotUsageOperation{{Name: "review", Fingerprint: "provider-quote", Maximum: imageagent.UsageVector{Images: 1, AgentSteps: 1, ModelCalls: 1, CostMicros: 10}, MaximumOutputs: 1}}, Fingerprint: "review-quote"}
	effect := imageagent.SlotEffectV3Attempt{Identity: identity, Policy: imageagent.BudgetPolicy{Images: imageagent.Limit{Enabled: true, Value: 4}}, Phase: imageagent.SlotEffectV3ReviewTransportRequired, BlockedCode: imageagent.SlotReviewTransportRequiredCode}
	reservation := imageagent.SlotReviewUsageReservation{Identity: identity, ActionID: "review-action", InputFingerprint: "input", Policy: effect.Policy, Quote: quote}
	accounting := AccountingSnapshot{Policy: effect.Policy, Committed: imageagent.UsageVector{Images: 1}, StartedAt: time.Now().Add(-time.Second)}

	reserved, err := ReserveReview(&effect, reservation, accounting)
	require.NoError(t, err)
	require.True(t, reserved.Acquired)
	require.Equal(t, int64(1), reserved.Accounting.Reserved.Images)

	repeat, err := ReserveReview(&reserved.Attempt, reservation, reserved.Accounting)
	require.NoError(t, err)
	require.False(t, repeat.Acquired)
	require.Equal(t, reserved.Accounting, repeat.Accounting)

	settled, err := SettleReview(reserved.Attempt, reservation, imageagent.SlotUsageReceipt{Actual: quote.Maximum, CostBasis: imageagent.UsageCostReservedUpperBound}, reserved.Accounting, time.Now())
	require.NoError(t, err)
	require.Zero(t, settled.Accounting.Reserved)
	require.Equal(t, int64(2), settled.Accounting.Committed.Images)
	require.Equal(t, quote.Maximum.CostMicros, settled.Accounting.Committed.CostMicros)
	require.Equal(t, imageagent.SlotBudgetCommitted, settled.Attempt.ReviewUsage[0].BudgetStatus)
}
