package listingadmin

import "strings"

func (s listingOperationStrategy) toOperationStrategy() OperationStrategy {
	return OperationStrategy{
		ID:                           s.ID,
		TenantID:                     s.TenantID,
		StoreID:                      s.StoreID,
		Name:                         s.Name,
		Platform:                     s.Platform,
		Status:                       s.Status,
		StockChangeThreshold:         intPtrIfPositive(s.StockChangeThreshold),
		StockChangeAction:            s.StockChangeAction,
		OutOfStockAction:             s.OutOfStockAction,
		MinProfitRate:                floatPtrIfPositive(s.MinProfitRate),
		LowProfitAction:              s.LowProfitAction,
		PriceUpdateMultiplier:        floatPtrIfPositive(s.PriceUpdateMultiplier),
		FixedPriceAdjustment:         floatPtrIfPositive(s.FixedPriceAdjustment),
		StockUpdateRatio:             floatPtrIfPositive(s.StockUpdateRatio),
		ActivityEnabled:              s.ActivityEnabled != 0,
		ActivityType:                 s.ActivityType,
		ActivityDiscountRate:         floatPtrIfPositive(s.ActivityDiscountRate),
		ActivityLimitedDiscountRate:  floatPtrIfPositive(s.ActivityLimitedDiscountRate),
		ActivityStockRatio:           floatPtrIfPositive(s.ActivityStockRatio),
		PromotionRatio:               floatPtrIfPositive(s.PromotionRatio),
		ActivityMinProfitRate:        activityProfitRatePtr(s.ActivityPriceMode, s.ActivityMinProfitRate),
		ActivityPriceMode:            s.ActivityPriceMode,
		ActivityPartakeType:          defaultActivityPartakeType(s.ActivityPartakeType),
		TimeLimitedDiscountRate:      floatPtrIfPositive(s.TimeLimitedDiscountRate),
		TimeLimitedMinProfitRate:     floatPtrIfPositive(s.TimeLimitedMinProfitRate),
		TimeLimitedPriceMode:         s.TimeLimitedPriceMode,
		TimeLimitedUserLimit:         s.TimeLimitedUserLimit,
		TimeLimitedUserLimitNum:      intPtrIfPositive(s.TimeLimitedUserLimitNum),
		TimeLimitedStockLimit:        s.TimeLimitedStockLimit,
		TimeLimitedStockLimitPercent: intPtrIfPositive(s.TimeLimitedStockLimitPercent),
		PriceIncreaseThreshold:       floatPtrIfPositive(s.PriceIncreaseThreshold),
		PriceDecreaseThreshold:       floatPtrIfPositive(s.PriceDecreaseThreshold),
		PriceIncreaseAction:          s.PriceIncreaseAction,
		PriceDecreaseAction:          s.PriceDecreaseAction,
		RestoreStockAmount:           intPtrIfPositive(s.RestoreStockAmount),
		Remark:                       s.Remark,
		CreateTime:                   s.CreateTime,
		UpdateTime:                   s.UpdateTime,
	}
}

func listingOperationStrategyFromOperationStrategy(strategy *OperationStrategy) listingOperationStrategy {
	if strategy == nil {
		return listingOperationStrategy{}
	}
	return listingOperationStrategy{
		ID:                           strategy.ID,
		TenantID:                     strategy.TenantID,
		StoreID:                      strategy.StoreID,
		Name:                         strings.TrimSpace(strategy.Name),
		Platform:                     strings.TrimSpace(strategy.Platform),
		Status:                       strategy.Status,
		StockChangeThreshold:         intValue(strategy.StockChangeThreshold),
		StockChangeAction:            strings.TrimSpace(strategy.StockChangeAction),
		OutOfStockAction:             strings.TrimSpace(strategy.OutOfStockAction),
		MinProfitRate:                floatValue(strategy.MinProfitRate),
		LowProfitAction:              strings.TrimSpace(strategy.LowProfitAction),
		PriceUpdateMultiplier:        floatValue(strategy.PriceUpdateMultiplier),
		FixedPriceAdjustment:         floatValue(strategy.FixedPriceAdjustment),
		StockUpdateRatio:             floatValue(strategy.StockUpdateRatio),
		ActivityEnabled:              boolToInt16(strategy.ActivityEnabled),
		ActivityType:                 strings.TrimSpace(strategy.ActivityType),
		ActivityDiscountRate:         floatValue(strategy.ActivityDiscountRate),
		ActivityLimitedDiscountRate:  floatValue(strategy.ActivityLimitedDiscountRate),
		ActivityStockRatio:           floatValue(strategy.ActivityStockRatio),
		PromotionRatio:               floatValue(strategy.PromotionRatio),
		ActivityMinProfitRate:        floatValue(strategy.ActivityMinProfitRate),
		ActivityPriceMode:            strings.TrimSpace(strategy.ActivityPriceMode),
		ActivityPartakeType:          defaultActivityPartakeType(strategy.ActivityPartakeType),
		TimeLimitedDiscountRate:      floatValue(strategy.TimeLimitedDiscountRate),
		TimeLimitedMinProfitRate:     floatValue(strategy.TimeLimitedMinProfitRate),
		TimeLimitedPriceMode:         strings.TrimSpace(strategy.TimeLimitedPriceMode),
		TimeLimitedUserLimit:         strategy.TimeLimitedUserLimit,
		TimeLimitedUserLimitNum:      intValue(strategy.TimeLimitedUserLimitNum),
		TimeLimitedStockLimit:        strategy.TimeLimitedStockLimit,
		TimeLimitedStockLimitPercent: intValue(strategy.TimeLimitedStockLimitPercent),
		PriceIncreaseThreshold:       floatValue(strategy.PriceIncreaseThreshold),
		PriceDecreaseThreshold:       floatValue(strategy.PriceDecreaseThreshold),
		PriceIncreaseAction:          strings.TrimSpace(strategy.PriceIncreaseAction),
		PriceDecreaseAction:          strings.TrimSpace(strategy.PriceDecreaseAction),
		RestoreStockAmount:           intValue(strategy.RestoreStockAmount),
		Remark:                       strings.TrimSpace(strategy.Remark),
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

func boolToInt16(value bool) int16 {
	if value {
		return 1
	}
	return 0
}

func activityProfitRatePtr(priceMode string, value float64) *float64 {
	if strings.ToUpper(strings.TrimSpace(priceMode)) != "PROFIT" {
		return floatPtrIfPositive(value)
	}
	if value < 0 || value >= 1 {
		return nil
	}
	return &value
}

func defaultActivityPartakeType(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "LIMITED", "BOTH":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return "REGULAR"
	}
}
