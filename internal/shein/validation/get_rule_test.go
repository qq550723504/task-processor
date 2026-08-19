package validation

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/model"
	"task-processor/internal/shein"
)

type validationFilterRuleRepository struct {
	rules []listingadmin.FilterRule
}

func (r validationFilterRuleRepository) ListFilterRules(context.Context, listingadmin.FilterRuleQuery) (*listingadmin.FilterRulePage, error) {
	return nil, nil
}

func (r validationFilterRuleRepository) GetFilterRule(context.Context, int64, int64) (*listingadmin.FilterRule, error) {
	return nil, nil
}

func (r validationFilterRuleRepository) ResolveFilterRules(context.Context, int64, int64, int64) ([]listingadmin.FilterRule, error) {
	return r.rules, nil
}

func (r validationFilterRuleRepository) CreateFilterRule(context.Context, *listingadmin.FilterRule) (*listingadmin.FilterRule, error) {
	return nil, nil
}

func (r validationFilterRuleRepository) UpdateFilterRule(context.Context, *listingadmin.FilterRule) (*listingadmin.FilterRule, error) {
	return nil, nil
}

func (r validationFilterRuleRepository) UpdateFilterRuleStatus(context.Context, int64, int64, int16, string) (*listingadmin.FilterRule, error) {
	return nil, nil
}

func (r validationFilterRuleRepository) DeleteFilterRule(context.Context, int64, int64) error {
	return nil
}

type validationProfitRuleRepository struct {
	rule *listingadmin.ProfitRule
}

func (r validationProfitRuleRepository) ListProfitRules(context.Context, listingadmin.ProfitRuleQuery) (*listingadmin.ProfitRulePage, error) {
	return nil, nil
}

func (r validationProfitRuleRepository) GetProfitRule(context.Context, int64, int64) (*listingadmin.ProfitRule, error) {
	return nil, nil
}

func (r validationProfitRuleRepository) ResolveProfitRule(context.Context, int64, int64) (*listingadmin.ProfitRule, error) {
	return r.rule, nil
}

func (r validationProfitRuleRepository) CreateProfitRule(context.Context, *listingadmin.ProfitRule) (*listingadmin.ProfitRule, error) {
	return nil, nil
}

func (r validationProfitRuleRepository) UpdateProfitRule(context.Context, *listingadmin.ProfitRule) (*listingadmin.ProfitRule, error) {
	return nil, nil
}

func (r validationProfitRuleRepository) UpdateProfitRuleStatus(context.Context, int64, int64, int16, string) (*listingadmin.ProfitRule, error) {
	return nil, nil
}

func (r validationProfitRuleRepository) DeleteProfitRule(context.Context, int64, int64) error {
	return nil
}

func TestTaskValidatorTreatsDisabledFilterRuleAsNonRetryable(t *testing.T) {
	handler := NewTaskValidatorHandler(
		validationFilterRuleRepository{rules: []listingadmin.FilterRule{{Name: "ND", Status: listingadmin.RuleStatusDisabled}}},
		validationProfitRuleRepository{rule: &listingadmin.ProfitRule{Name: "ND margin", Status: listingadmin.RuleStatusEnabled}},
	)
	ctx := shein.NewTaskContext(context.Background(), &model.Task{TenantID: 227, StoreID: 883})

	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("Handle() error = nil, want disabled filter rule error")
	}
	if err.Error() != "filter rule is not enabled: ND" {
		t.Fatalf("Handle() error = %q, want disabled filter rule message", err.Error())
	}
	if shein.IsRetryableError(err) {
		t.Fatalf("Handle() error is retryable, want configuration error to be non-retryable: %v", err)
	}
}

func TestTaskValidatorTreatsDisabledProfitRuleAsNonRetryable(t *testing.T) {
	handler := NewTaskValidatorHandler(
		validationFilterRuleRepository{rules: []listingadmin.FilterRule{{Name: "ND", Status: listingadmin.RuleStatusEnabled}}},
		validationProfitRuleRepository{rule: &listingadmin.ProfitRule{Name: "ND margin", Status: listingadmin.RuleStatusDisabled}},
	)
	ctx := shein.NewTaskContext(context.Background(), &model.Task{TenantID: 227, StoreID: 883})

	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("Handle() error = nil, want disabled profit rule error")
	}
	if err.Error() != "profit rule is not enabled: ND margin" {
		t.Fatalf("Handle() error = %q, want disabled profit rule message", err.Error())
	}
	if shein.IsRetryableError(err) {
		t.Fatalf("Handle() error is retryable, want configuration error to be non-retryable: %v", err)
	}
}
