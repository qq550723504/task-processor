// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"context"
	"fmt"

	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/marketing"

	"github.com/sirupsen/logrus"
)

// RegisterMixedActivity 根据运营策略按比例执行混合活动
func (s *activityRegistrationServiceImpl) RegisterMixedActivity(
	ctx context.Context,
	strategy *managementapi.OperationStrategyDTO,
) (int, int, error) {
	s.logger.WithFields(logrus.Fields{
		"store_id":        strategy.StoreID,
		"promotion_ratio": strategy.PromotionRatio,
	}).Info("开始按比例执行混合活动")

	// 1. 获取可报名活动的产品列表
	products, err := s.fetchAvailableProducts(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("获取可报名产品列表失败: %w", err)
	}

	s.logger.Infof("获取到 %d 个可报名产品", len(products))

	if len(products) == 0 {
		s.logger.Info("没有可报名的产品")
		return 0, 0, nil
	}

	// 2. 确定促销活动比例
	promotionRatio := strategy.PromotionRatio
	if promotionRatio < 0 {
		promotionRatio = 0
	} else if promotionRatio > 1 {
		promotionRatio = 1
	}

	// 3. 按比例分割产品列表
	promotionCount := int(float64(len(products)) * promotionRatio)
	promotionProducts := products[:promotionCount]
	timeLimitedProducts := products[promotionCount:]

	s.logger.Infof("按比例分割产品：促销活动 %d 个，限时折扣 %d 个", len(promotionProducts), len(timeLimitedProducts))

	var registeredPromotionCount, registeredTimeLimitedCount int

	// 4. 执行促销活动报名
	if len(promotionProducts) > 0 {
		registeredPromotionCount, err = s.registerPromotionProducts(ctx, promotionProducts, strategy)
		if err != nil {
			s.logger.WithError(err).Error("促销活动报名失败")
			return 0, 0, fmt.Errorf("促销活动报名失败: %w", err)
		}
		s.logger.Infof("成功报名 %d 个产品到促销活动", registeredPromotionCount)
	}

	// 5. 执行限时折扣活动创建（如果有剩余产品）
	if len(timeLimitedProducts) > 0 {
		// 注意：限时折扣活动会内部查询商品，这里我们只是记录数量
		// 实际创建时会使用 QueryPromotionGoods 接口
		registeredTimeLimitedCount, err = s.CreateTimeLimitedDiscountActivity(ctx, strategy)
		if err != nil {
			s.logger.WithError(err).Error("限时折扣活动创建失败")
			return registeredPromotionCount, 0, fmt.Errorf("限时折扣活动创建失败: %w", err)
		}
		s.logger.Infof("成功创建限时折扣活动，参与商品数: %d", registeredTimeLimitedCount)
	}

	s.logger.WithFields(logrus.Fields{
		"promotion_count":    registeredPromotionCount,
		"time_limited_count": registeredTimeLimitedCount,
	}).Info("混合活动执行完成")

	return registeredPromotionCount, registeredTimeLimitedCount, nil
}

// registerPromotionProducts 报名指定产品到促销活动（私有方法）
func (s *activityRegistrationServiceImpl) registerPromotionProducts(
	ctx context.Context,
	products []marketing.SkcInfo,
	strategy *managementapi.OperationStrategyDTO,
) (int, error) {
	if len(products) == 0 {
		return 0, nil
	}

	// 计算折扣率
	dropRate := int(strategy.ActivityDiscountRate * 100)
	if dropRate <= 0 || dropRate > 100 {
		dropRate = 10
	}

	// 构建活动配置列表
	configList := s.buildActivityConfigsWithStrategy(products, dropRate, strategy.ActivityStockRatio, strategy.StoreID)

	if len(configList) == 0 {
		return 0, nil
	}

	// 调用 SHEIN API 保存活动配置
	saveReq := &marketing.SaveConfigRequest{
		ConfigList: configList,
	}

	response, err := s.marketingAPI.SaveConfig(saveReq)
	if err != nil {
		return 0, fmt.Errorf("保存活动配置失败: %w", err)
	}

	if response.Code != "0" {
		return 0, fmt.Errorf("保存活动配置失败: %s", response.Msg)
	}

	return len(configList), nil
}
