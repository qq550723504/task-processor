package listingadmin

import "strings"

func (s listingOperationStrategy) toOperationStrategy() OperationStrategy {
	return OperationStrategy{
		ID:                    s.ID,
		TenantID:              s.TenantID,
		StoreID:               s.StoreID,
		Name:                  s.Name,
		Platform:              s.Platform,
		Status:                s.Status,
		StockChangeThreshold:  intPtrIfPositive(s.StockChangeThreshold),
		StockChangeAction:     s.StockChangeAction,
		OutOfStockAction:      s.OutOfStockAction,
		MinProfitRate:         floatPtrIfPositive(s.MinProfitRate),
		LowProfitAction:       s.LowProfitAction,
		PriceUpdateMultiplier: floatPtrIfPositive(s.PriceUpdateMultiplier),
		FixedPriceAdjustment:  floatPtrIfPositive(s.FixedPriceAdjustment),
		StockUpdateRatio:      floatPtrIfPositive(s.StockUpdateRatio),
		Remark:                s.Remark,
		CreateTime:            s.CreateTime,
		UpdateTime:            s.UpdateTime,
	}
}

func listingOperationStrategyFromOperationStrategy(strategy *OperationStrategy) listingOperationStrategy {
	if strategy == nil {
		return listingOperationStrategy{}
	}
	return listingOperationStrategy{
		ID:                    strategy.ID,
		TenantID:              strategy.TenantID,
		StoreID:               strategy.StoreID,
		Name:                  strings.TrimSpace(strategy.Name),
		Platform:              strings.TrimSpace(strategy.Platform),
		Status:                strategy.Status,
		StockChangeThreshold:  intValue(strategy.StockChangeThreshold),
		StockChangeAction:     strings.TrimSpace(strategy.StockChangeAction),
		OutOfStockAction:      strings.TrimSpace(strategy.OutOfStockAction),
		MinProfitRate:         floatValue(strategy.MinProfitRate),
		LowProfitAction:       strings.TrimSpace(strategy.LowProfitAction),
		PriceUpdateMultiplier: floatValue(strategy.PriceUpdateMultiplier),
		FixedPriceAdjustment:  floatValue(strategy.FixedPriceAdjustment),
		StockUpdateRatio:      floatValue(strategy.StockUpdateRatio),
		Remark:                strings.TrimSpace(strategy.Remark),
	}
}

func applyOperationStrategyAuditFields(row *listingOperationStrategy, userID string, includeCreate bool) {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return
	}
	row.OwnerUserID = trimmedUserID
	row.Updater = trimmedUserID
	row.UpdatedBy = trimmedUserID
	if includeCreate {
		row.Creator = trimmedUserID
		row.CreatedBy = trimmedUserID
	}
}
