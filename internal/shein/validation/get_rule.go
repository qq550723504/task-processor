package validation

import (
	"fmt"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/shein"
)

type TaskValidatorHandler struct {
	filterRuleRepo listingadmin.FilterRuleRepository
	profitRuleRepo listingadmin.ProfitRuleRepository
}

func NewTaskValidatorHandler(filterRuleRepo listingadmin.FilterRuleRepository, profitRuleRepo listingadmin.ProfitRuleRepository) *TaskValidatorHandler {
	return &TaskValidatorHandler{
		filterRuleRepo: filterRuleRepo,
		profitRuleRepo: profitRuleRepo,
	}
}

func (t *TaskValidatorHandler) Name() string {
	return "task_validator"
}

func (t *TaskValidatorHandler) Handle(ctx *shein.TaskContext) error {
	filterRule, err := t.loadFilterRule(ctx)
	if err != nil {
		return err
	}
	if filterRule.Status != 0 {
		return fmt.Errorf("filter rule is not enabled: %s", filterRule.Name)
	}

	profitRule, err := t.loadProfitRule(ctx)
	if err != nil {
		return err
	}
	if profitRule.Status != 0 {
		return fmt.Errorf("profit rule is not enabled: %s", profitRule.Name)
	}

	ctx.SetValidationRules(filterRule, profitRule)
	return nil
}

func (t *TaskValidatorHandler) loadFilterRule(ctx *shein.TaskContext) (*listingruntime.FilterRule, error) {
	if t.filterRuleRepo == nil {
		return nil, fmt.Errorf("filter rule repository is not configured")
	}
	filterRules, err := t.filterRuleRepo.ResolveFilterRules(ctx.GetContext(), ctx.Task.TenantID, ctx.Task.StoreID, ctx.Task.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("get filter rule failed: %w", err)
	}
	if len(filterRules) == 0 {
		return nil, fmt.Errorf("filter rule not found")
	}

	return runtimeFilterRuleFromListing(&filterRules[0]), nil
}

func (t *TaskValidatorHandler) loadProfitRule(ctx *shein.TaskContext) (*listingruntime.ProfitRule, error) {
	if t.profitRuleRepo == nil {
		return nil, fmt.Errorf("profit rule repository is not configured")
	}
	profitRule, err := t.profitRuleRepo.ResolveProfitRule(ctx.GetContext(), ctx.Task.TenantID, ctx.Task.StoreID)
	if err != nil {
		return nil, fmt.Errorf("get profit rule failed: %w", err)
	}
	return runtimeProfitRuleFromListing(profitRule), nil
}

func runtimeFilterRuleFromListing(rule *listingadmin.FilterRule) *listingruntime.FilterRule {
	if rule == nil {
		return nil
	}
	return &listingruntime.FilterRule{
		ID:              rule.ID,
		Name:            rule.Name,
		TenantID:        rule.TenantID,
		StoreID:         listingInt64Value(rule.StoreID),
		CategoryID:      listingInt64Value(rule.CategoryID),
		PriceType:       rule.PriceType,
		PriceMin:        listingFloat64Ptr(rule.PriceMin),
		PriceMax:        listingFloat64Ptr(rule.PriceMax),
		StockMin:        listingIntPtr(rule.StockMin),
		RatingMin:       listingFloat64Ptr(rule.RatingMin),
		ReviewCountMin:  listingIntPtr(rule.ReviewCountMin),
		DeliveryTimeMax: rule.DeliveryTimeMax,
		FulfillmentType: rule.FulfillmentType,
		Status:          rule.Status,
	}
}

func runtimeProfitRuleFromListing(rule *listingadmin.ProfitRule) *listingruntime.ProfitRule {
	if rule == nil {
		return nil
	}
	return &listingruntime.ProfitRule{
		ID:                      rule.ID,
		Name:                    rule.Name,
		TenantID:                rule.TenantID,
		StoreID:                 rule.StoreID,
		CategoryID:              rule.CategoryID,
		SalePriceMultiplier:     rule.SalePriceMultiplier,
		DiscountPriceMultiplier: rule.DiscountPriceMultiplier,
		Status:                  rule.Status,
	}
}

func listingInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func listingFloat64Ptr(value float64) *float64 {
	out := value
	return &out
}

func listingIntPtr(value int) *int {
	out := value
	return &out
}
