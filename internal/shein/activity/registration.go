// package activity 提供SHEIN平台调度器相关服务
package activity

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/shein/api/marketing"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ActivityRegistrationService 活动报名服务接口
type ActivityRegistrationService interface {
	// RegisterPromotionActivity 根据运营策略报名促销活动（完整流程）
	RegisterPromotionActivity(ctx context.Context, strategy *listingruntime.OperationStrategy) (int, error)

	// RegisterPromotionProducts 使用调用方提供的产品集合执行促销活动报名。
	RegisterPromotionProducts(ctx context.Context, strategy *listingruntime.OperationStrategy, activityKey string, products []marketing.SkcInfo) (*PromotionRegistrationResult, error)

	// CreateTimeLimitedDiscountActivity 根据运营策略创建限时折扣活动（完整流程）
	CreateTimeLimitedDiscountActivity(ctx context.Context, strategy *listingruntime.OperationStrategy) (int, error)

	// RegisterMixedActivity 根据运营策略按比例执行混合活动（部分促销 + 部分限时折扣）
	RegisterMixedActivity(ctx context.Context, strategy *listingruntime.OperationStrategy) (promotionCount int, timeLimitedCount int, err error)
}

type PromotionRegistrationBridge interface {
	RegisterPromotionProducts(ctx context.Context, strategy *listingruntime.OperationStrategy, activityKey string, products []marketing.SkcInfo) (*PromotionRegistrationResult, error)
}

type PromotionRegistrationSessionFactory interface {
	NewPromotionRegistrationSession(ctx context.Context, strategy *listingruntime.OperationStrategy, activityKey string) (PromotionRegistrationSession, error)
}

type PromotionRegistrationSession interface {
	RegisterPromotionProducts(ctx context.Context, activityKey string, products []marketing.SkcInfo) (*PromotionRegistrationResult, error)
}

// activityRegistrationServiceImpl 活动报名服务实现
type activityRegistrationServiceImpl struct {
	storeService    listingruntime.StoreService
	storeRepo       activityStoreFinder
	mappingRepo     activityMappingFinder
	productDataRepo listingadmin.ProductDataRepository
	marketingAPI    marketing.MarketingAPI
	logger          *logrus.Entry
}

type activityStoreFinder interface {
	FindStoreByID(ctx context.Context, id int64) (*listingadmin.Store, error)
}

type activityMappingFinder interface {
	FindLatest(ctx context.Context, query listingadmin.ProductImportMappingQuery) (*listingadmin.ProductImportMapping, error)
}

type PromotionRegistrationResult struct {
	Request          *marketing.SaveConfigRequest
	Response         *marketing.SaveConfigResponse
	ActivityRequest  *marketing.CreateActivityRequest
	ActivityResponse *marketing.CreateActivityResponse
	FilterReasons    map[string]string
}

func (r *PromotionRegistrationResult) GetRequest() *marketing.SaveConfigRequest {
	if r == nil {
		return nil
	}
	return r.Request
}

func (r *PromotionRegistrationResult) GetResponse() *marketing.SaveConfigResponse {
	if r == nil {
		return nil
	}
	return r.Response
}

// NewActivityRegistrationService 创建活动报名服务
func NewActivityRegistrationService(
	storeService listingruntime.StoreService,
	storeRepo activityStoreFinder,
	mappingRepo activityMappingFinder,
	productDataRepo listingadmin.ProductDataRepository,
	marketingAPI marketing.MarketingAPI,
) ActivityRegistrationService {
	return &activityRegistrationServiceImpl{
		storeService:    storeService,
		storeRepo:       storeRepo,
		mappingRepo:     mappingRepo,
		productDataRepo: productDataRepo,
		marketingAPI:    marketingAPI,
		logger:          logger.GetGlobalLogger("ActivityRegistrationService"),
	}
}

// fetchAvailableProducts 获取可报名活动的产品列表（私有方法）
func (s *activityRegistrationServiceImpl) fetchAvailableProducts() ([]marketing.SkcInfo, error) {
	s.logger.Debug("开始获取可报名活动的产品列表")

	var allProducts []marketing.SkcInfo

	// 分页获取所有可报名的产品
	pageNum := 1
	const pageSize = 100

	for {
		req := &marketing.GetAvailableSkcListRequest{
			PageNum:  pageNum,
			PageSize: pageSize,
		}

		// 调用 SHEIN API 获取可报名产品列表
		response, err := s.marketingAPI.GetAvailableSkcList(req)
		if err != nil {
			s.logger.Errorf("获取可报名产品列表失败(页面%d): %v", pageNum, err)
			return nil, fmt.Errorf("获取可报名产品列表失败: %w", err)
		}

		if response.Info == nil {
			break
		}

		s.logger.Debugf("页面%d获取到%d个可报名产品", pageNum, len(response.Info.SkcInfoList))

		allProducts = append(allProducts, response.Info.SkcInfoList...)

		// 如果当前页数据少于页面大小，说明已经到最后一页
		if len(response.Info.SkcInfoList) < pageSize {
			break
		}
		pageNum++
	}

	s.logger.Infof("获取可报名产品列表完成，共%d个产品", len(allProducts))
	return allProducts, nil
}

// RegisterPromotionActivity 根据运营策略报名促销活动（完整流程）
func (s *activityRegistrationServiceImpl) RegisterPromotionActivity(
	ctx context.Context,
	strategy *listingruntime.OperationStrategy,
) (int, error) {
	s.logger.WithFields(logrus.Fields{
		"store_id":      strategy.StoreID,
		"price_mode":    strategy.ActivityPriceMode,
		"discount_rate": strategy.ActivityDiscountRate,
		"min_profit":    strategy.ActivityMinProfitRate,
		"stock_ratio":   strategy.ActivityStockRatio,
	}).Info("开始根据运营策略报名促销活动")

	// 1. 获取可报名活动的产品列表
	products, err := s.fetchAvailableProducts()
	if err != nil {
		return 0, fmt.Errorf("获取可报名产品列表失败: %w", err)
	}

	s.logger.Infof("获取到 %d 个可报名产品", len(products))

	if len(products) == 0 {
		s.logger.Info("没有可报名的产品")
		return 0, nil
	}

	result, err := s.RegisterPromotionProducts(ctx, strategy, "", products)
	if err != nil {
		return 0, err
	}
	if result == nil || result.Request == nil {
		return 0, nil
	}
	return len(result.Request.ConfigList), nil
}

func (s *activityRegistrationServiceImpl) RegisterPromotionProducts(
	ctx context.Context,
	strategy *listingruntime.OperationStrategy,
	activityKey string,
	products []marketing.SkcInfo,
) (*PromotionRegistrationResult, error) {
	if len(products) == 0 {
		return &PromotionRegistrationResult{}, nil
	}
	if activityKey != "" {
		return s.createPromotionActivityFromProducts(ctx, strategy, activityKey, products)
	}

	// 2. 根据定价模式构建活动配置
	var configList []marketing.ActivityConfig

	priceMode := autoPartakePriceModeFromStrategy(strategy)

	configList = s.buildPromotionConfigList(products, strategy, priceMode)

	if len(configList) == 0 {
		s.logger.Info("没有符合条件的产品需要报名")
		return &PromotionRegistrationResult{}, nil
	}

	activityTypes := autoPartakeActivityTypesFromStrategy(strategy)
	if err := validateAutoPartakeDiscountsForStrategy(strategy); err != nil {
		return &PromotionRegistrationResult{}, err
	}
	firstReq, firstResponse, err := s.savePromotionConfigs(products, configList, activityTypes, strategy, priceMode)
	if err != nil {
		return &PromotionRegistrationResult{Request: firstReq, Response: firstResponse}, err
	}

	if err := s.enableSavedPromotionConfigs(ctx, configList, activityTypes); err != nil {
		s.logger.Errorf("开启活动配置失败: %v", err)
		return &PromotionRegistrationResult{Request: firstReq, Response: firstResponse}, fmt.Errorf("开启活动配置失败: %w", err)
	}

	s.logger.Infof("成功报名 %d 个产品到促销活动", len(configList))
	return &PromotionRegistrationResult{
		Request:  firstReq,
		Response: firstResponse,
	}, nil
}

func (s *activityRegistrationServiceImpl) buildPromotionConfigList(products []marketing.SkcInfo, strategy *listingruntime.OperationStrategy, priceMode string) []marketing.ActivityConfig {
	if priceMode == "PROFIT" {
		configList := s.buildActivityConfigsByProfit(products, strategy.ActivityMinProfitRate, autoPartakeStockRatioFromStrategy(strategy), strategy.StoreID, strategy.FixedPriceAdjustment)
		if len(configList) == 0 {
			configList = s.buildActivityConfigsFromProvidedProducts(products, strategy, priceMode)
		}
		return configList
	}

	dropRate := CalculateDropRateFromDiscount(strategy.ActivityDiscountRate, s.logger)
	s.logger.Debugf("使用折扣率: %d%%", dropRate)
	configList := s.buildActivityConfigsWithStrategy(products, dropRate, autoPartakeStockRatioFromStrategy(strategy), strategy.StoreID)
	if len(configList) == 0 {
		configList = s.buildActivityConfigsFromProvidedProducts(products, strategy, priceMode)
	}
	return configList
}

func (s *activityRegistrationServiceImpl) savePromotionConfigs(products []marketing.SkcInfo, configList []marketing.ActivityConfig, activityTypes []int, strategy *listingruntime.OperationStrategy, priceMode string) (*marketing.SaveConfigRequest, *marketing.SaveConfigResponse, error) {
	var firstReq *marketing.SaveConfigRequest
	var firstResponse *marketing.SaveConfigResponse
	for _, activityType := range activityTypes {
		saveReq := &marketing.SaveConfigRequest{
			ConfigList: s.promotionConfigListForActivityType(products, configList, activityType, strategy, priceMode),
			Type:       activityType,
		}
		if firstReq == nil {
			firstReq = saveReq
		}

		response, err := s.marketingAPI.SaveConfig(saveReq)
		if firstResponse == nil {
			firstResponse = response
		}
		if err != nil {
			s.logger.Errorf("保存活动配置失败: %v", err)
			return saveReq, response, fmt.Errorf("保存活动配置失败: %w", err)
		}
		if response.Code != "0" {
			return saveReq, response, fmt.Errorf("保存活动配置失败: %s", response.Msg)
		}
	}
	return firstReq, firstResponse, nil
}

func (s *activityRegistrationServiceImpl) promotionConfigListForActivityType(products []marketing.SkcInfo, configList []marketing.ActivityConfig, activityType int, strategy *listingruntime.OperationStrategy, priceMode string) []marketing.ActivityConfig {
	copied := append([]marketing.ActivityConfig(nil), configList...)
	if activityType != marketing.AutoPartakeActivityTypeLimited || strategy == nil {
		return copied
	}
	if priceMode == "PROFIT" && strategy.ActivityLimitedMinProfitRate >= 0 && strategy.ActivityLimitedMinProfitRate < 1 {
		limitedStrategy := *strategy
		limitedStrategy.ActivityMinProfitRate = strategy.ActivityLimitedMinProfitRate
		limitedConfigList := s.buildPromotionConfigList(products, &limitedStrategy, priceMode)
		if len(limitedConfigList) > 0 {
			return ensureLimitedPromotionDropRatesGreater(copied, limitedConfigList)
		}
		return copied
	}
	if autoPartakePriceModeFromStrategy(strategy) != "DISCOUNT" || strategy.ActivityLimitedDiscountRate <= 0 {
		return copied
	}
	limitedDropRate := CalculateDropRateFromDiscount(strategy.ActivityLimitedDiscountRate, nil)
	for i := range copied {
		copied[i].DropRate = limitedDropRate
	}
	return copied
}

func ensureLimitedPromotionDropRatesGreater(regularConfigs, limitedConfigs []marketing.ActivityConfig) []marketing.ActivityConfig {
	regularDropRateBySKC := make(map[string]int, len(regularConfigs))
	for _, config := range regularConfigs {
		if config.Skc == "" {
			continue
		}
		regularDropRateBySKC[config.Skc] = config.DropRate
	}
	if len(regularDropRateBySKC) == 0 {
		return limitedConfigs
	}

	out := append([]marketing.ActivityConfig(nil), limitedConfigs...)
	for i := range out {
		regularDropRate, ok := regularDropRateBySKC[out[i].Skc]
		if !ok || out[i].DropRate > regularDropRate || regularDropRate >= 99 {
			continue
		}
		out[i].DropRate = regularDropRate + 1
	}
	return out
}

func validateAutoPartakeDiscountsForStrategy(strategy *listingruntime.OperationStrategy) error {
	if strategy == nil {
		return nil
	}
	if strings.ToUpper(strings.TrimSpace(strategy.ActivityPartakeType)) != "BOTH" {
		return nil
	}
	switch autoPartakePriceModeFromStrategy(strategy) {
	case "DISCOUNT":
		if strategy.ActivityLimitedDiscountRate <= 0 || strategy.ActivityLimitedDiscountRate >= 1 {
			return fmt.Errorf("限量活动折扣率必须在 0 到 1 之间")
		}
		if strategy.ActivityLimitedDiscountRate <= strategy.ActivityDiscountRate {
			return fmt.Errorf("同时报名常规和限量活动时，限量活动折扣率必须大于常规活动折扣率")
		}
		return nil
	case "PROFIT":
		if strategy.ActivityLimitedMinProfitRate < 0 || strategy.ActivityLimitedMinProfitRate >= 1 {
			return fmt.Errorf("限量活动最低利润率必须在 0 到 1 之间")
		}
		if strategy.ActivityLimitedMinProfitRate >= strategy.ActivityMinProfitRate {
			return fmt.Errorf("同时报名常规和限量活动时，限量活动最低利润率必须小于常规活动最低利润率")
		}
	}
	return nil
}

func autoPartakePriceModeFromStrategy(strategy *listingruntime.OperationStrategy) string {
	if strategy == nil {
		return "DISCOUNT"
	}
	priceMode := strings.ToUpper(strings.TrimSpace(strategy.ActivityPriceMode))
	if priceMode == "" {
		return "DISCOUNT"
	}
	return priceMode
}

func (s *activityRegistrationServiceImpl) enableSavedPromotionConfigs(ctx context.Context, configList []marketing.ActivityConfig, activityTypes []int) error {
	if len(configList) == 0 || len(activityTypes) == 0 {
		return nil
	}

	ids, err := s.findSavedPromotionConfigIDs(ctx, configList, activityTypes)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}

	response, err := s.marketingAPI.UpdateConfigState(&marketing.UpdateConfigStateRequest{
		IDs:   ids,
		State: marketing.AutoPartakeConfigStateOpen,
	})
	if err != nil {
		return err
	}
	if response.Code != "0" {
		return fmt.Errorf("开启活动配置失败: %s", response.Msg)
	}
	return nil
}

func (s *activityRegistrationServiceImpl) findSavedPromotionConfigIDs(ctx context.Context, configList []marketing.ActivityConfig, activityTypes []int) ([]int64, error) {
	targetSkcs := make(map[string]struct{}, len(configList))
	for _, config := range configList {
		if config.Skc != "" {
			targetSkcs[config.Skc] = struct{}{}
		}
	}
	if len(targetSkcs) == 0 {
		return nil, nil
	}
	targetCount := len(targetSkcs) * len(activityTypes)

	const pageSize = 500
	foundConfigs := make(map[string]struct{}, targetCount)
	ids := make([]int64, 0, targetCount)
	for pageNum := 1; ; pageNum++ {
		response, err := s.marketingAPI.GetConfigList(&marketing.GetConfigListRequest{
			PageNum:  pageNum,
			PageSize: pageSize,
		})
		if err != nil {
			return nil, err
		}
		if response.Code != "0" {
			return nil, fmt.Errorf("获取已报名活动配置失败: %s", response.Msg)
		}
		if response.Info == nil || len(response.Info.ConfigList) == 0 {
			break
		}

		for _, row := range response.Info.ConfigList {
			if _, ok := targetSkcs[row.Skc]; !ok {
				continue
			}
			for _, activityType := range activityTypes {
				id, shouldOpen := promotionConfigIDForActivityType(row, activityType)
				if id <= 0 {
					continue
				}
				key := fmt.Sprintf("%s:%d", row.Skc, activityType)
				if _, ok := foundConfigs[key]; ok {
					continue
				}
				foundConfigs[key] = struct{}{}
				if shouldOpen {
					ids = append(ids, id)
				}
			}
		}

		if len(foundConfigs) == targetCount || pageNum*pageSize >= response.Info.Total {
			break
		}
	}

	if len(foundConfigs) != targetCount {
		return nil, fmt.Errorf("保存活动配置后未找到可开启的配置ID: 已找到 %d/%d", len(foundConfigs), targetCount)
	}
	return ids, nil
}

func promotionConfigIDForActivityType(row marketing.ActivityConfigInfo, activityType int) (int64, bool) {
	for _, config := range row.ActivityConfigList {
		if config.ActivityType != activityType || config.ID <= 0 {
			continue
		}
		return config.ID, config.State != marketing.AutoPartakeConfigStateOpen
	}
	if len(row.ActivityConfigList) > 0 {
		return 0, false
	}
	if row.ID <= 0 {
		return 0, false
	}
	return row.ID, row.State != marketing.AutoPartakeConfigStateOpen
}

func autoPartakeActivityTypeFromStrategy(strategy *listingruntime.OperationStrategy) int {
	return autoPartakeActivityTypesFromStrategy(strategy)[0]
}

func autoPartakeActivityTypesFromStrategy(strategy *listingruntime.OperationStrategy) []int {
	if strategy == nil {
		return []int{marketing.AutoPartakeActivityTypeRegular}
	}
	switch strings.ToUpper(strings.TrimSpace(strategy.ActivityPartakeType)) {
	case "LIMITED":
		return []int{marketing.AutoPartakeActivityTypeLimited}
	case "BOTH":
		return []int{marketing.AutoPartakeActivityTypeRegular, marketing.AutoPartakeActivityTypeLimited}
	default:
		return []int{marketing.AutoPartakeActivityTypeRegular}
	}
}

func autoPartakeStockRatioFromStrategy(strategy *listingruntime.OperationStrategy) float64 {
	if strategy == nil {
		return 1
	}
	if !autoPartakeRequiresStockRatio(strategy) && strategy.ActivityStockRatio <= 0 {
		return 1
	}
	return strategy.ActivityStockRatio
}

func autoPartakeRequiresStockRatio(strategy *listingruntime.OperationStrategy) bool {
	if strategy == nil {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(strategy.ActivityPartakeType)) {
	case "LIMITED", "BOTH":
		return true
	default:
		return false
	}
}

func (s *activityRegistrationServiceImpl) getStoreInfo(ctx context.Context, storeID int64) (*listingruntime.StoreInfo, error) {
	if s.storeRepo != nil {
		store, err := s.storeRepo.FindStoreByID(ctx, storeID)
		if err != nil {
			s.logger.WithError(err).WithField("store_id", storeID).Warn("通过本地仓储获取店铺信息失败，回退 runtime store service")
		} else if store != nil {
			return activityStoreInfoFromListingStore(store), nil
		}
	}
	if s.storeService == nil {
		return nil, fmt.Errorf("runtime store service is nil")
	}
	return s.storeService.GetStore(storeID)
}

func (s *activityRegistrationServiceImpl) getMappingByPlatformProductIDAndStore(ctx context.Context, platformProductID string, storeID int64) (*listingruntime.ProductImportMapping, error) {
	if s.mappingRepo != nil {
		mapping, err := s.mappingRepo.FindLatest(ctx, listingadmin.ProductImportMappingQuery{
			PlatformProductID: platformProductID,
			StoreID:           &storeID,
		})
		if err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"platform_product_id": platformProductID,
				"store_id":            storeID,
			}).Warn("通过本地仓储获取产品映射失败")
		} else if mapping != nil {
			return activityProductImportMappingFromListing(mapping), nil
		}
	}
	return nil, nil
}

func activityStoreInfoFromListingStore(store *listingadmin.Store) *listingruntime.StoreInfo {
	if store == nil {
		return nil
	}
	return &listingruntime.StoreInfo{
		ID:                       store.ID,
		TenantID:                 store.TenantID,
		StoreID:                  store.StoreID,
		Username:                 store.Username,
		Name:                     store.Name,
		ShopType:                 store.ShopType,
		Region:                   store.Region,
		Platform:                 store.Platform,
		LoginURL:                 store.LoginURL,
		Proxy:                    store.Proxy,
		DailyLimit:               store.DailyLimit,
		DailyLimitType:           store.DailyLimitType,
		PriceType:                store.PriceType,
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

func activityProductImportMappingFromListing(mapping *listingadmin.ProductImportMapping) *listingruntime.ProductImportMapping {
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
		ParentProductID:         activityStringPtr(mapping.ParentProductID),
		SKU:                     activityStringPtr(mapping.SKU),
		PlatformProductID:       activityStringPtr(mapping.PlatformProductID),
		PlatformParentProductID: activityStringPtr(mapping.PlatformParentProductID),
		CostPrice:               activityFloat64Value(mapping.CostPrice),
		FilterRuleID:            activityInt64Value(mapping.FilterRuleID),
		FilterRuleRange:         activityStringPtr(mapping.FilterRuleRange),
		ProfitRuleID:            activityInt64Value(mapping.ProfitRuleID),
		SalePriceMultiplier:     activityFloat64PtrFromValue(mapping.SalePriceMultiplier),
		DiscountPriceMultiplier: activityFloat64PtrFromValue(mapping.DiscountPriceMultiplier),
		Status:                  mapping.Status,
		Remark:                  activityStringPtr(mapping.Remark),
		TenantID:                mapping.TenantID,
	}
}

func activityStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	out := value
	return &out
}

func activityFloat64PtrFromValue(value float64) *float64 {
	if value == 0 {
		return nil
	}
	out := value
	return &out
}

func activityFloat64Value(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func activityInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
