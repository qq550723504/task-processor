package listingadmin

import (
	"context"
	"time"

	"task-processor/internal/pkg/types"
)

// NewGormPricingRuleAPI adapts the pricing rule repository for callers using
// the management API DTO contract.
func NewGormPricingRuleAPI(repository *GormPricingRuleRepository) PricingRuleAPI {
	if repository == nil {
		return nil
	}
	return gormPricingRuleAPI{repository: repository}
}

type gormPricingRuleAPI struct {
	repository *GormPricingRuleRepository
}

func (a gormPricingRuleAPI) GetPricingRule(req *PricingRuleReqDTO) ([]PricingRuleRespDTO, error) {
	if a.repository == nil || req == nil || req.StoreID == nil {
		return nil, nil
	}
	items, err := a.repository.ListByStoreID(context.Background(), *req.StoreID)
	if err != nil {
		return nil, err
	}
	rows := make([]PricingRuleRespDTO, 0, len(items))
	for _, item := range items {
		rows = append(rows, PricingRuleToRespDTO(item))
	}
	return rows, nil
}

// PricingRuleToRespDTO exposes the management API projection for a pricing rule.
func PricingRuleToRespDTO(rule PricingRule) PricingRuleRespDTO {
	dto := PricingRuleRespDTO{
		ID:         rule.ID,
		Name:       rule.Name,
		RuleCode:   rule.RuleCode,
		StoreID:    rule.StoreID,
		CategoryID: rule.CategoryID,
		PriceMin:   ptrFloat64(rule.PriceMin),
		PriceMax:   ptrFloat64(rule.PriceMax),
		RuleType:   rule.RuleType,
		RuleValue:  ptrFloat64(rule.RuleValue),
		FixedValue: rule.FixedValue,
		Status:     int(rule.Status),
		CreateTime: flexibleTimeValue(rule.CreateTime),
		TenantID:   rule.TenantID,
	}
	if rule.Description != "" {
		dto.Description = ptrString(rule.Description)
	}
	if rule.AcceptCondition != "" {
		dto.AcceptCondition = ptrString(rule.AcceptCondition)
	}
	if rule.RejectCondition != "" {
		dto.RejectCondition = ptrString(rule.RejectCondition)
	}
	if rule.Remark != "" {
		dto.Remark = ptrString(rule.Remark)
	}
	return dto
}

func ptrFloat64(value float64) *float64 {
	return &value
}

func ptrString(value string) *string {
	return &value
}

func flexibleTimeValue(value *time.Time) types.FlexibleTime {
	if value == nil {
		return types.FlexibleTime{}
	}
	return types.FlexibleTime{Time: *value}
}
