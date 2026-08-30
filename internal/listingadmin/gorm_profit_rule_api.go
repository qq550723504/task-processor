package listingadmin

import "context"

// NewGormProfitRuleAPI adapts the profit rule repository for callers using
// the management API DTO contract.
func NewGormProfitRuleAPI(repository *GormProfitRuleRepository) ProfitRuleAPI {
	if repository == nil {
		return nil
	}
	return gormProfitRuleAPI{repository: repository}
}

type gormProfitRuleAPI struct {
	repository *GormProfitRuleRepository
}

func (a gormProfitRuleAPI) GetProfitRule(req *ProfitRuleReqDTO) (*ProfitRuleRespDTO, error) {
	if a.repository == nil || req == nil {
		return nil, nil
	}
	rule, err := a.repository.ResolveProfitRule(context.Background(), req.TenantID, req.StoreID)
	if err != nil || rule == nil {
		return nil, err
	}
	return ProfitRuleToRespDTO(rule), nil
}

// ProfitRuleToRespDTO exposes the management API projection for a profit rule.
func ProfitRuleToRespDTO(rule *ProfitRule) *ProfitRuleRespDTO {
	if rule == nil {
		return nil
	}
	return &ProfitRuleRespDTO{
		ID:                      rule.ID,
		Name:                    rule.Name,
		RuleCode:                rule.RuleCode,
		Description:             rule.Description,
		StoreID:                 rule.StoreID,
		CategoryID:              rule.CategoryID,
		SalePriceMultiplier:     rule.SalePriceMultiplier,
		DiscountPriceMultiplier: rule.DiscountPriceMultiplier,
		Status:                  rule.Status,
		Remark:                  rule.Remark,
		CreateTime:              flexibleTimeValue(rule.CreateTime),
		TenantID:                rule.TenantID,
	}
}
