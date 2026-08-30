package listingadmin

import "context"

// NewGormFilterRuleAPI adapts the filter rule repository for callers using
// the management API DTO contract.
func NewGormFilterRuleAPI(repository *GormFilterRuleRepository) FilterRuleAPI {
	if repository == nil {
		return nil
	}
	return gormFilterRuleAPI{repository: repository}
}

type gormFilterRuleAPI struct {
	repository *GormFilterRuleRepository
}

func (a gormFilterRuleAPI) GetFilterRule(req *FilterRuleReqDTO) (*[]FilterRuleRespDTO, error) {
	if a.repository == nil || req == nil {
		return nil, nil
	}
	items, err := a.repository.ResolveFilterRules(context.Background(), req.TenantID, req.StoreID, req.CategoryID)
	if err != nil {
		return nil, err
	}
	rows := make([]FilterRuleRespDTO, 0, len(items))
	for _, item := range items {
		rows = append(rows, FilterRuleToRespDTO(item))
	}
	return &rows, nil
}

// FilterRuleToRespDTO exposes the management API projection for a filter rule.
func FilterRuleToRespDTO(rule FilterRule) FilterRuleRespDTO {
	dto := FilterRuleRespDTO{
		ID:              rule.ID,
		Name:            rule.Name,
		RuleCode:        rule.RuleCode,
		Description:     rule.Description,
		TenantID:        rule.TenantID,
		PriceType:       rule.PriceType,
		FulfillmentType: rule.FulfillmentType,
		Status:          rule.Status,
		Remark:          rule.Remark,
		CreateTime:      flexibleTimeValue(rule.CreateTime),
	}
	if rule.StoreID != nil {
		dto.StoreID = *rule.StoreID
	}
	if rule.CategoryID != nil {
		dto.CategoryID = *rule.CategoryID
	}
	dto.PriceMin = ptrFloat64(rule.PriceMin)
	dto.PriceMax = ptrFloat64(rule.PriceMax)
	dto.StockMin = ptrInt(rule.StockMin)
	dto.RatingMin = ptrFloat64(rule.RatingMin)
	dto.ReviewCountMin = ptrInt(rule.ReviewCountMin)
	dto.DeliveryTimeMax = rule.DeliveryTimeMax
	return dto
}

func ptrInt(value int) *int {
	return &value
}
