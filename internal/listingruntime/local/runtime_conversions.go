package local

import (
	"strconv"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
)

const runtimeStoreDiscoveryPageSize = 200

func runtimeStoreFromListing(store *listingadmin.Store) *listingruntime.StoreInfo {
	if store == nil {
		return nil
	}
	return &listingruntime.StoreInfo{
		ID:                       store.ID,
		TenantID:                 store.TenantID,
		StoreID:                  store.StoreID,
		Username:                 store.Username,
		Platform:                 store.Platform,
		Name:                     store.Name,
		Region:                   store.Region,
		ShopType:                 store.ShopType,
		LoginURL:                 store.LoginURL,
		Proxy:                    store.Proxy,
		PriceType:                store.PriceType,
		DailyLimit:               store.DailyLimit,
		DailyLimitType:           store.DailyLimitType,
		EnableDraft:              store.EnableDraft,
		EnableAutoListing:        store.EnableAutoListing,
		FixedStockCount:          store.FixedStockCount,
		SkuGenerateStrategy:      store.SKUGenerateStrategy,
		Prefix:                   store.Prefix,
		Suffix:                   store.Suffix,
		EnableBrandAuthorization: store.EnableBrandAuthorization,
		AuthorizedBrandCode:      store.AuthorizedBrandCode,
		AuthorizedBrandName:      store.AuthorizedBrandName,
	}
}

func runtimeStoreFromListingAdminDTO(store *listingadmin.StoreRespDTO) *listingruntime.StoreInfo {
	if store == nil {
		return nil
	}
	return &listingruntime.StoreInfo{
		ID:                       store.ID,
		TenantID:                 store.TenantID,
		StoreID:                  store.StoreID,
		Username:                 store.Username,
		Platform:                 store.Platform,
		Name:                     store.Name,
		Region:                   store.Region,
		ShopType:                 store.ShopType,
		LoginURL:                 store.LoginUrl,
		Proxy:                    store.Proxy,
		PriceType:                store.PriceType,
		DailyLimit:               store.DailyLimit,
		DailyLimitType:           store.DailyLimitType,
		EnableDraft:              store.EnableDraft,
		EnableAutoListing:        store.EnableAutoListing,
		FixedStockCount:          store.FixedStockCount,
		SkuGenerateStrategy:      store.SkuGenerateStrategy,
		Prefix:                   store.Prefix,
		Suffix:                   store.Suffix,
		EnableBrandAuthorization: store.EnableBrandAuthorization,
		AuthorizedBrandCode:      store.AuthorizedBrandCode,
		AuthorizedBrandName:      store.AuthorizedBrandName,
	}
}

func runtimePauseDetailFromListingAdminDTO(detail *listingadmin.StorePauseStatusRespDTO) *listingruntime.StorePauseStatusDetail {
	if detail == nil {
		return nil
	}
	return &listingruntime.StorePauseStatusDetail{
		Paused:     detail.Paused,
		PauseType:  detail.PauseType,
		Reason:     detail.Reason,
		TTLSeconds: detail.TTLSeconds,
	}
}

func runtimeOperationStrategyFromListing(strategy *listingadmin.OperationStrategy) *listingruntime.OperationStrategy {
	if strategy == nil {
		return nil
	}
	return &listingruntime.OperationStrategy{
		ID:                           strategy.ID,
		TenantID:                     strategy.TenantID,
		StoreID:                      strategy.StoreID,
		Name:                         strategy.Name,
		Platform:                     strategy.Platform,
		Status:                       strategy.Status,
		StockChangeThreshold:         runtimeIntValue(strategy.StockChangeThreshold),
		StockChangeAction:            strategy.StockChangeAction,
		OutOfStockAction:             strategy.OutOfStockAction,
		MinProfitRate:                runtimeFloat64Value(strategy.MinProfitRate),
		LowProfitAction:              strategy.LowProfitAction,
		PriceUpdateMultiplier:        runtimeFloat64Value(strategy.PriceUpdateMultiplier),
		StockUpdateRatio:             runtimeFloat64Value(strategy.StockUpdateRatio),
		ActivityEnabled:              strategy.ActivityEnabled,
		ActivityType:                 strategy.ActivityType,
		ActivityDiscountRate:         runtimeFloat64Value(strategy.ActivityDiscountRate),
		ActivityStockRatio:           runtimeFloat64Value(strategy.ActivityStockRatio),
		PromotionRatio:               runtimeFloat64Value(strategy.PromotionRatio),
		ActivityMinProfitRate:        runtimeFloat64Value(strategy.ActivityMinProfitRate),
		ActivityPriceMode:            strategy.ActivityPriceMode,
		TimeLimitedDiscountRate:      runtimeFloat64Value(strategy.TimeLimitedDiscountRate),
		TimeLimitedMinProfitRate:     runtimeFloat64Value(strategy.TimeLimitedMinProfitRate),
		TimeLimitedPriceMode:         strategy.TimeLimitedPriceMode,
		TimeLimitedUserLimit:         strategy.TimeLimitedUserLimit,
		TimeLimitedUserLimitNum:      runtimeIntValue(strategy.TimeLimitedUserLimitNum),
		TimeLimitedStockLimit:        strategy.TimeLimitedStockLimit,
		TimeLimitedStockLimitPercent: runtimeIntValue(strategy.TimeLimitedStockLimitPercent),
		FixedPriceAdjustment:         runtimeFloat64Value(strategy.FixedPriceAdjustment),
		PriceIncreaseThreshold:       runtimeFloat64Value(strategy.PriceIncreaseThreshold),
		PriceDecreaseThreshold:       runtimeFloat64Value(strategy.PriceDecreaseThreshold),
		PriceIncreaseAction:          strategy.PriceIncreaseAction,
		PriceDecreaseAction:          strategy.PriceDecreaseAction,
		RestoreStockAmount:           runtimeIntValue(strategy.RestoreStockAmount),
		Remark:                       strategy.Remark,
	}
}

func runtimeOperationStrategyFromListingAdminDTO(strategy *listingadmin.OperationStrategyDTO) *listingruntime.OperationStrategy {
	if strategy == nil {
		return nil
	}
	return &listingruntime.OperationStrategy{
		ID:                           strategy.ID,
		TenantID:                     strategy.TenantID,
		StoreID:                      strategy.StoreID,
		Name:                         strategy.Name,
		Platform:                     strategy.Platform,
		Status:                       strategy.Status,
		StockChangeThreshold:         strategy.StockChangeThreshold,
		StockChangeAction:            strategy.StockChangeAction,
		OutOfStockAction:             strategy.OutOfStockAction,
		MinProfitRate:                strategy.MinProfitRate,
		LowProfitAction:              strategy.LowProfitAction,
		PriceUpdateMultiplier:        strategy.PriceUpdateMultiplier,
		StockUpdateRatio:             strategy.StockUpdateRatio,
		ActivityEnabled:              strategy.ActivityEnabled,
		ActivityType:                 strategy.ActivityType,
		ActivityDiscountRate:         strategy.ActivityDiscountRate,
		ActivityStockRatio:           strategy.ActivityStockRatio,
		PromotionRatio:               strategy.PromotionRatio,
		ActivityMinProfitRate:        strategy.ActivityMinProfitRate,
		ActivityPriceMode:            strategy.ActivityPriceMode,
		TimeLimitedDiscountRate:      strategy.TimeLimitedDiscountRate,
		TimeLimitedMinProfitRate:     strategy.TimeLimitedMinProfitRate,
		TimeLimitedPriceMode:         strategy.TimeLimitedPriceMode,
		TimeLimitedUserLimit:         strategy.TimeLimitedUserLimit,
		TimeLimitedUserLimitNum:      strategy.TimeLimitedUserLimitNum,
		TimeLimitedStockLimit:        strategy.TimeLimitedStockLimit,
		TimeLimitedStockLimitPercent: strategy.TimeLimitedStockLimitPercent,
		FixedPriceAdjustment:         strategy.FixedPriceAdjustment,
		PriceIncreaseThreshold:       strategy.PriceIncreaseThreshold,
		PriceDecreaseThreshold:       strategy.PriceDecreaseThreshold,
		PriceIncreaseAction:          strategy.PriceIncreaseAction,
		PriceDecreaseAction:          strategy.PriceDecreaseAction,
		RestoreStockAmount:           strategy.RestoreStockAmount,
		Remark:                       strategy.Remark,
	}
}

func runtimeProductImportMappingFromListing(mapping *listingadmin.ProductImportMapping) *listingruntime.ProductImportMapping {
	if mapping == nil {
		return nil
	}
	return &listingruntime.ProductImportMapping{
		ID:                      mapping.ID,
		ImportTaskID:            mapping.ImportTaskID,
		StoreID:                 mapping.StoreID,
		Platform:                mapping.Platform,
		Region:                  mapping.Region,
		ProductID:               mapping.ProductID,
		ParentProductID:         runtimeStringPtr(mapping.ParentProductID),
		SKU:                     runtimeStringPtr(mapping.SKU),
		PlatformProductID:       runtimeStringPtr(mapping.PlatformProductID),
		PlatformParentProductID: runtimeStringPtr(mapping.PlatformParentProductID),
		CostPrice:               runtimeFloat64Value(mapping.CostPrice),
		FilterRuleID:            runtimeInt64Value(mapping.FilterRuleID),
		FilterRuleRange:         runtimeStringPtr(mapping.FilterRuleRange),
		ProfitRuleID:            runtimeInt64Value(mapping.ProfitRuleID),
		SalePriceMultiplier:     runtimeFloat64Ptr(mapping.SalePriceMultiplier),
		DiscountPriceMultiplier: runtimeFloat64Ptr(mapping.DiscountPriceMultiplier),
		Status:                  mapping.Status,
		Remark:                  runtimeStringPtr(mapping.Remark),
		TenantID:                mapping.TenantID,
	}
}

func runtimeProductImportMappingFromListingAdminDTO(mapping *listingadmin.ProductImportMappingRespDTO) *listingruntime.ProductImportMapping {
	if mapping == nil {
		return nil
	}
	return &listingruntime.ProductImportMapping{
		ID:                      mapping.ID,
		ImportTaskID:            mapping.ImportTaskId,
		StoreID:                 mapping.StoreId,
		Platform:                mapping.Platform,
		Region:                  mapping.Region,
		ProductID:               mapping.ProductId,
		ParentProductID:         mapping.ParentProductId,
		SKU:                     mapping.Sku,
		PlatformProductID:       mapping.PlatformProductId,
		PlatformParentProductID: mapping.PlatformParentProductId,
		CostPrice:               runtimeFloat64Value(mapping.CostPrice),
		FilterRuleID:            runtimeInt64Value(mapping.FilterRuleId),
		FilterRuleRange:         mapping.FilterRuleRange,
		ProfitRuleID:            runtimeInt64Value(mapping.ProfitRuleId),
		SalePriceMultiplier:     mapping.SalePriceMultiplier,
		DiscountPriceMultiplier: mapping.DiscountPriceMultiplier,
		Status:                  mapping.Status,
		Remark:                  mapping.Remark,
		TenantID:                mapping.TenantId,
	}
}

func listingProductImportMappingFromRuntime(req *listingruntime.ProductImportMappingUpsert) *listingadmin.ProductImportMapping {
	if req == nil {
		return nil
	}
	return &listingadmin.ProductImportMapping{
		ID:                      runtimeInt64Value(req.ID),
		TenantID:                req.TenantID,
		ImportTaskID:            req.ImportTaskID,
		StoreID:                 req.StoreID,
		Platform:                req.Platform,
		Region:                  req.Region,
		ProductID:               req.ProductID,
		ParentProductID:         runtimeStringValue(req.ParentProductID),
		SKU:                     runtimeStringValue(req.SKU),
		CostPrice:               req.CostPrice,
		PlatformProductID:       runtimeStringValue(req.PlatformProductID),
		PlatformParentProductID: runtimeStringValue(req.PlatformParentProductID),
		FilterRuleID:            runtimePositiveInt64Ptr(req.FilterRuleID),
		FilterRuleRange:         runtimeStringValue(req.FilterRuleRange),
		ProfitRuleID:            runtimePositiveInt64Ptr(req.ProfitRuleID),
		SalePriceMultiplier:     runtimeFloat64Value(req.SalePriceMultiplier),
		DiscountPriceMultiplier: runtimeFloat64Value(req.DiscountPriceMultiplier),
		Status:                  runtimeInt16Value(req.Status),
		Remark:                  runtimeStringValue(req.Remark),
	}
}

func listingAdminProductImportMappingCreateDTOFromRuntime(req *listingruntime.ProductImportMappingUpsert) *listingadmin.ProductImportMappingCreateReqDTO {
	if req == nil {
		return nil
	}
	return &listingadmin.ProductImportMappingCreateReqDTO{
		ID:                      req.ID,
		TenantID:                req.TenantID,
		ImportTaskId:            req.ImportTaskID,
		StoreId:                 req.StoreID,
		Platform:                req.Platform,
		Region:                  req.Region,
		ProductId:               req.ProductID,
		Sku:                     req.SKU,
		CostPrice:               req.CostPrice,
		PlatformProductId:       req.PlatformProductID,
		ProfitRuleId:            req.ProfitRuleID,
		SalePriceMultiplier:     runtimeFormatFloat(req.SalePriceMultiplier),
		DiscountPriceMultiplier: runtimeFormatFloat(req.DiscountPriceMultiplier),
		Status:                  req.Status,
		Remark:                  req.Remark,
		ParentProductId:         req.ParentProductID,
		PlatformParentProductId: req.PlatformParentProductID,
		FilterRuleId:            req.FilterRuleID,
		FilterRuleRange:         req.FilterRuleRange,
	}
}

func runtimeStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	out := value
	return &out
}

func runtimeStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func runtimeFloat64Ptr(value float64) *float64 {
	if value == 0 {
		return nil
	}
	out := value
	return &out
}

func runtimeFloat64Value(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func runtimePositiveInt64Ptr(value *int64) *int64 {
	if value == nil || *value <= 0 {
		return nil
	}
	out := *value
	return &out
}

func runtimeInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func runtimeIntValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func runtimeInt16Value(value *int16) int16 {
	if value == nil {
		return 0
	}
	return *value
}

func runtimeFormatFloat(value *float64) *string {
	if value == nil {
		return nil
	}
	out := strconv.FormatFloat(*value, 'f', -1, 64)
	return &out
}

func dedupeInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
