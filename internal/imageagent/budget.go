package imageagent

import (
	"errors"
	"fmt"
	"math"
)

var (
	ErrBudgetExceeded = errors.New("image agent budget exceeded")
	ErrBudgetOverflow = errors.New("image agent budget usage overflow")
)

type BudgetLimitSet uint8

const (
	BudgetLimitImages BudgetLimitSet = 1 << iota
	BudgetLimitAgentSteps
	BudgetLimitModelCalls
	BudgetLimitRepairAttemptsPerSlot
	BudgetLimitCostMicros
	BudgetLimitElapsed

	allBudgetLimits = BudgetLimitImages | BudgetLimitAgentSteps | BudgetLimitModelCalls |
		BudgetLimitRepairAttemptsPerSlot | BudgetLimitCostMicros | BudgetLimitElapsed
)

const (
	BudgetLimitNameImages                = "max_images"
	BudgetLimitNameAgentSteps            = "max_agent_steps"
	BudgetLimitNameModelCalls            = "max_model_calls"
	BudgetLimitNameRepairAttemptsPerSlot = "max_repair_attempts_per_slot"
	BudgetLimitNameCostMicros            = "max_cost_micros"
	BudgetLimitNameElapsed               = "max_elapsed"
)

type Limit struct {
	Enabled bool
	Value   int64
}

type BudgetPolicy struct {
	Images                Limit
	AgentSteps            Limit
	ModelCalls            Limit
	RepairAttemptsPerSlot Limit
	CostMicros            Limit
	MaxElapsed            Limit
}

type UsageVector struct {
	Images     int64
	AgentSteps int64
	ModelCalls int64
	CostMicros int64
}

func (budget Budget) Policy() (BudgetPolicy, error) {
	if budget.EnabledLimits&^allBudgetLimits != 0 {
		return BudgetPolicy{}, fmt.Errorf("%w: budget contains unknown enabled limits", ErrValidation)
	}
	presenceAware := budget.EnabledLimits != 0
	values := []struct {
		bit   BudgetLimitSet
		value int64
		set   func(*BudgetPolicy, Limit)
	}{
		{BudgetLimitImages, int64(budget.MaxImages), func(policy *BudgetPolicy, limit Limit) { policy.Images = limit }},
		{BudgetLimitAgentSteps, int64(budget.MaxAgentSteps), func(policy *BudgetPolicy, limit Limit) { policy.AgentSteps = limit }},
		{BudgetLimitModelCalls, int64(budget.MaxModelCalls), func(policy *BudgetPolicy, limit Limit) { policy.ModelCalls = limit }},
		{BudgetLimitRepairAttemptsPerSlot, int64(budget.MaxRepairAttemptsPerSlot), func(policy *BudgetPolicy, limit Limit) { policy.RepairAttemptsPerSlot = limit }},
		{BudgetLimitCostMicros, budget.MaxCostMicros, func(policy *BudgetPolicy, limit Limit) { policy.CostMicros = limit }},
		{BudgetLimitElapsed, int64(budget.MaxElapsed), func(policy *BudgetPolicy, limit Limit) { policy.MaxElapsed = limit }},
	}
	var policy BudgetPolicy
	for _, item := range values {
		if item.value < 0 {
			return BudgetPolicy{}, fmt.Errorf("%w: budget limits must be non-negative", ErrValidation)
		}
		enabled := item.value > 0
		if presenceAware {
			enabled = budget.EnabledLimits&item.bit != 0
			if item.value > 0 && !enabled {
				return BudgetPolicy{}, fmt.Errorf("%w: positive budget limit is missing presence metadata", ErrValidation)
			}
		}
		item.set(&policy, Limit{Enabled: enabled, Value: item.value})
	}
	return policy, nil
}

func (budget Budget) EnabledLimitNames() ([]string, error) {
	policy, err := budget.Policy()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, 6)
	for _, item := range []struct {
		name  string
		limit Limit
	}{
		{BudgetLimitNameImages, policy.Images},
		{BudgetLimitNameAgentSteps, policy.AgentSteps},
		{BudgetLimitNameModelCalls, policy.ModelCalls},
		{BudgetLimitNameRepairAttemptsPerSlot, policy.RepairAttemptsPerSlot},
		{BudgetLimitNameCostMicros, policy.CostMicros},
		{BudgetLimitNameElapsed, policy.MaxElapsed},
	} {
		if item.limit.Enabled {
			names = append(names, item.name)
		}
	}
	return names, nil
}

func (policy BudgetPolicy) Allows(committed, reserved, quote UsageVector) error {
	if err := policy.validate(); err != nil {
		return err
	}
	current, err := CheckedAddUsage(committed, reserved)
	if err != nil {
		return err
	}
	projected, err := CheckedAddUsage(current, quote)
	if err != nil {
		return err
	}
	for _, item := range []struct {
		name  string
		limit Limit
		value int64
	}{
		{BudgetLimitNameImages, policy.Images, projected.Images},
		{BudgetLimitNameAgentSteps, policy.AgentSteps, projected.AgentSteps},
		{BudgetLimitNameModelCalls, policy.ModelCalls, projected.ModelCalls},
		{BudgetLimitNameCostMicros, policy.CostMicros, projected.CostMicros},
	} {
		if item.limit.Enabled && item.value > item.limit.Value {
			return fmt.Errorf("%w: %s limit is %d and projected usage is %d", ErrBudgetExceeded, item.name, item.limit.Value, item.value)
		}
	}
	return nil
}

func (policy BudgetPolicy) AllowsRepairAttempt(repairAttempt int) error {
	if err := policy.validate(); err != nil {
		return err
	}
	if repairAttempt < 0 {
		return fmt.Errorf("%w: repair attempt must be non-negative", ErrValidation)
	}
	if policy.RepairAttemptsPerSlot.Enabled && int64(repairAttempt) > policy.RepairAttemptsPerSlot.Value {
		return fmt.Errorf("%w: %s limit is %d and requested repair attempt is %d", ErrBudgetExceeded, BudgetLimitNameRepairAttemptsPerSlot, policy.RepairAttemptsPerSlot.Value, repairAttempt)
	}
	return nil
}

func (policy BudgetPolicy) validate() error {
	for _, limit := range []Limit{policy.Images, policy.AgentSteps, policy.ModelCalls, policy.RepairAttemptsPerSlot, policy.CostMicros, policy.MaxElapsed} {
		if limit.Value < 0 || (!limit.Enabled && limit.Value != 0) {
			return fmt.Errorf("%w: normalized budget limit is invalid", ErrValidation)
		}
	}
	return nil
}

func CheckedAddUsage(left, right UsageVector) (UsageVector, error) {
	if err := validateUsageVector(left); err != nil {
		return UsageVector{}, err
	}
	if err := validateUsageVector(right); err != nil {
		return UsageVector{}, err
	}
	images, err := checkedAddNonNegative(left.Images, right.Images)
	if err != nil {
		return UsageVector{}, err
	}
	agentSteps, err := checkedAddNonNegative(left.AgentSteps, right.AgentSteps)
	if err != nil {
		return UsageVector{}, err
	}
	modelCalls, err := checkedAddNonNegative(left.ModelCalls, right.ModelCalls)
	if err != nil {
		return UsageVector{}, err
	}
	costMicros, err := checkedAddNonNegative(left.CostMicros, right.CostMicros)
	if err != nil {
		return UsageVector{}, err
	}
	return UsageVector{Images: images, AgentSteps: agentSteps, ModelCalls: modelCalls, CostMicros: costMicros}, nil
}

func validateUsageVector(usage UsageVector) error {
	if usage.Images < 0 || usage.AgentSteps < 0 || usage.ModelCalls < 0 || usage.CostMicros < 0 {
		return fmt.Errorf("%w: usage vector values must be non-negative", ErrValidation)
	}
	return nil
}

func checkedAddNonNegative(left, right int64) (int64, error) {
	if left > math.MaxInt64-right {
		return 0, ErrBudgetOverflow
	}
	return left + right, nil
}
