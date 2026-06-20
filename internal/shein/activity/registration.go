// package activity 提供SHEIN平台调度器相关服务
package activity

import (
	"context"
	"fmt"

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
	Request  *marketing.SaveConfigRequest
	Response *marketing.SaveConfigResponse
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
	_ = ctx
	_ = activityKey

	if len(products) == 0 {
		return &PromotionRegistrationResult{}, nil
	}

	// 2. 根据定价模式构建活动配置
	var configList []marketing.ActivityConfig

	priceMode := strategy.ActivityPriceMode
	if priceMode == "" {
		priceMode = "DISCOUNT" // 默认按折扣率
	}

	if priceMode == "PROFIT" {
		// 按最低利润率定价，使用管理系统配置的固定价格调整值
		configList = s.buildActivityConfigsByProfit(products, strategy.ActivityMinProfitRate, strategy.ActivityStockRatio, strategy.StoreID, strategy.FixedPriceAdjustment)
	} else {
		// 按折扣率定价
		dropRate := CalculateDropRateFromDiscount(strategy.ActivityDiscountRate, s.logger)
		s.logger.Debugf("使用折扣率: %d%%", dropRate)
		configList = s.buildActivityConfigsWithStrategy(products, dropRate, strategy.ActivityStockRatio, strategy.StoreID)
	}

	if len(configList) == 0 {
		s.logger.Info("没有符合条件的产品需要报名")
		return &PromotionRegistrationResult{}, nil
	}

	// 3. 调用 SHEIN API 保存活动配置（报名）
	saveReq := &marketing.SaveConfigRequest{
		ConfigList: configList,
	}

	response, err := s.marketingAPI.SaveConfig(saveReq)
	if err != nil {
		s.logger.Errorf("保存活动配置失败: %v", err)
		return &PromotionRegistrationResult{Request: saveReq}, fmt.Errorf("保存活动配置失败: %w", err)
	}

	if response.Code != "0" {
		return &PromotionRegistrationResult{Request: saveReq, Response: response}, fmt.Errorf("保存活动配置失败: %s", response.Msg)
	}

	s.logger.Infof("成功报名 %d 个产品到促销活动", len(configList))
	return &PromotionRegistrationResult{
		Request:  saveReq,
		Response: response,
	}, nil
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
