package imageagent

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBudgetPolicyDistinguishesOmittedFromExplicitZero(t *testing.T) {
	omitted, err := (Budget{}).Policy()
	require.NoError(t, err)
	require.False(t, omitted.Images.Enabled)
	require.NoError(t, omitted.Allows(UsageVector{}, UsageVector{}, UsageVector{Images: 1}))

	explicitZero, err := (Budget{EnabledLimits: BudgetLimitImages}).Policy()
	require.NoError(t, err)
	require.Equal(t, Limit{Enabled: true, Value: 0}, explicitZero.Images)
	require.ErrorIs(t, explicitZero.Allows(UsageVector{}, UsageVector{}, UsageVector{Images: 1}), ErrBudgetExceeded)
}

func TestBudgetPolicyInfersPositiveLegacyLimitsWithoutPresenceMetadata(t *testing.T) {
	policy, err := (Budget{MaxImages: 2, MaxElapsed: 5 * time.Second}).Policy()

	require.NoError(t, err)
	require.Equal(t, Limit{Enabled: true, Value: 2}, policy.Images)
	require.Equal(t, Limit{Enabled: true, Value: int64(5 * time.Second)}, policy.MaxElapsed)
	require.False(t, policy.ModelCalls.Enabled)
}

func TestBudgetPolicyRejectsNegativeAndInconsistentLimits(t *testing.T) {
	for name, budget := range map[string]Budget{
		"negative legacy value":  {MaxImages: -1},
		"negative enabled value": {MaxCostMicros: -1, EnabledLimits: BudgetLimitCostMicros},
		"positive disabled value with presence metadata": {
			MaxImages: 1, EnabledLimits: BudgetLimitModelCalls,
		},
		"unknown presence bit": {EnabledLimits: BudgetLimitSet(1 << 7)},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := budget.Policy()
			require.ErrorIs(t, err, ErrValidation)
		})
	}
}

func TestBudgetPolicyRejectsOverflowAndEachExceededDimension(t *testing.T) {
	_, err := CheckedAddUsage(UsageVector{Images: math.MaxInt64}, UsageVector{Images: 1})
	require.ErrorIs(t, err, ErrBudgetOverflow)

	for name, test := range map[string]struct {
		policy BudgetPolicy
		quote  UsageVector
	}{
		"images":      {BudgetPolicy{Images: Limit{Enabled: true, Value: 0}}, UsageVector{Images: 1}},
		"agent steps": {BudgetPolicy{AgentSteps: Limit{Enabled: true, Value: 0}}, UsageVector{AgentSteps: 1}},
		"model calls": {BudgetPolicy{ModelCalls: Limit{Enabled: true, Value: 0}}, UsageVector{ModelCalls: 1}},
		"cost micros": {BudgetPolicy{CostMicros: Limit{Enabled: true, Value: 0}}, UsageVector{CostMicros: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			err := test.policy.Allows(UsageVector{}, UsageVector{}, test.quote)
			require.ErrorIs(t, err, ErrBudgetExceeded)
		})
	}
}

func TestBudgetPolicyChecksRepairAttemptsPerSlot(t *testing.T) {
	policy := BudgetPolicy{RepairAttemptsPerSlot: Limit{Enabled: true, Value: 1}}

	require.NoError(t, policy.AllowsRepairAttempt(1))
	require.ErrorIs(t, policy.AllowsRepairAttempt(2), ErrBudgetExceeded)
	require.ErrorIs(t, policy.AllowsRepairAttempt(-1), ErrValidation)
}
