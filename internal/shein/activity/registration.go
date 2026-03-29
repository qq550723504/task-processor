// package activity 提供SHEIN平台调度器相关服务
package activity

import (
	"context"
	"fmt"

	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/shein/api/marketing"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ActivityRegistrationService 活动报名服务接口
type ActivityRegistrationService interface {
	// RegisterPromotionActivity 根据运营策略报名促销活动（完整流程）
	RegisterPromotionActivity(ctx context.Context, strategy *managementapi.OperationStrategyDTO) (int, error)

	// CreateTimeLimitedDiscountActivity 根据运营策略创建限时折扣活动（完整流程）
	CreateTimeLimitedDiscountActivity(ctx context.Context, strategy *managementapi.OperationStrategyDTO) (int, error)

	// RegisterMixedActivity 根据运营策略按比例执行混合活动（部分促销 + 部分限时折扣）
	RegisterMixedActivity(ctx context.Context, strategy *managementapi.OperationStrategyDTO) (promotionCount int, timeLimitedCount int, err error)
}

// activityRegistrationServiceImpl 活动报名服务实现
type activityRegistrationServiceImpl struct {
	managementClient *management.ClientManager
	marketingAPI     marketing.MarketingAPI
	logger           *logrus.Entry
}

// NewActivityRegistrationService 创建活动报名服务
func NewActivityRegistrationService(
	managementClient *management.ClientManager,
	marketingAPI marketing.MarketingAPI,
) ActivityRegistrationService {
	return &activityRegistrationServiceImpl{
		managementClient: managementClient,
		marketingAPI:     marketingAPI,
		logger:           logger.GetGlobalLogger("ActivityRegistrationService"),
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
	strategy *managementapi.OperationStrategyDTO,
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
		return 0, nil
	}

	// 3. 调用 SHEIN API 保存活动配置（报名）
	saveReq := &marketing.SaveConfigRequest{
		ConfigList: configList,
	}

	response, err := s.marketingAPI.SaveConfig(saveReq)
	if err != nil {
		s.logger.Errorf("保存活动配置失败: %v", err)
		return 0, fmt.Errorf("保存活动配置失败: %w", err)
	}

	if response.Code != "0" {
		return 0, fmt.Errorf("保存活动配置失败: %s", response.Msg)
	}

	s.logger.Infof("成功报名 %d 个产品到促销活动", len(configList))
	return len(configList), nil
}
