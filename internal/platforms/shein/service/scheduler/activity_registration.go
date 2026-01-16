// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/pkg/management"
	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/marketing"
	"task-processor/internal/platforms/shein/repo"

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
	marketingAPI     repo.MarketingAPIInterface
	logger           *logrus.Entry
}

// NewActivityRegistrationService 创建活动报名服务
func NewActivityRegistrationService(
	managementClient *management.ClientManager,
	marketingAPI repo.MarketingAPIInterface,
) ActivityRegistrationService {
	return &activityRegistrationServiceImpl{
		managementClient: managementClient,
		marketingAPI:     marketingAPI,
		logger:           logrus.WithField("component", "ActivityRegistrationService"),
	}
}

// fetchAvailableProducts 获取可报名活动的产品列表（私有方法）
func (s *activityRegistrationServiceImpl) fetchAvailableProducts(ctx context.Context) ([]marketing.SkcInfo, error) {
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

// RegisterProducts 自动报名产品到活动（使用默认配置，私有方法）
func (s *activityRegistrationServiceImpl) registerProducts(ctx context.Context, products []marketing.SkcInfo) (int, error) {
	s.logger.WithField("product_count", len(products)).Debug("开始报名产品到活动")

	if len(products) == 0 {
		s.logger.Info("没有产品需要报名")
		return 0, nil
	}

	// 构建活动配置列表（使用默认配置）
	configList := s.buildActivityConfigs(products, 10, 1.0) // 默认降价10%，使用全部库存

	// 调用 SHEIN API 保存活动配置（报名）
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

	s.logger.Infof("成功报名 %d 个产品到活动", len(products))
	return len(products), nil
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
	products, err := s.fetchAvailableProducts(ctx)
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
		// 按最低利润率定价
		configList = s.buildActivityConfigsByProfit(products, strategy.ActivityMinProfitRate, strategy.ActivityStockRatio, strategy.StoreID)
	} else {
		// 按折扣率定价
		dropRate := int(strategy.ActivityDiscountRate * 100)
		if dropRate <= 0 || dropRate > 100 {
			dropRate = 10 // 默认10%
		}
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
