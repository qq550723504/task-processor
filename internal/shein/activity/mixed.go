// package activity 提供SHEIN平台调度器相关服务
package activity

import (
	"context"
	"fmt"

	"task-processor/internal/listingruntime"
	"task-processor/internal/shein/api/marketing"

	"github.com/sirupsen/logrus"
)

// RegisterMixedActivity 根据运营策略按比例执行混合活动
func (s *activityRegistrationServiceImpl) RegisterMixedActivity(
	ctx context.Context,
	strategy *listingruntime.OperationStrategy,
) (int, int, error) {
	s.logger.WithFields(logrus.Fields{
		"store_id":        strategy.StoreID,
		"promotion_ratio": strategy.PromotionRatio,
	}).Info("暂未开放")

	return 0, 0, nil
}

// registerPromotionProducts 报名指定产品到促销活动（私有方法）
func (s *activityRegistrationServiceImpl) registerPromotionProducts(
	ctx context.Context,
	products []marketing.SkcInfo,
	strategy *listingruntime.OperationStrategy,
) (int, error) {
	if len(products) == 0 {
		return 0, nil
	}

	// 计算折扣率
	dropRate := CalculateDropRateFromDiscount(strategy.ActivityDiscountRate, s.logger)
	s.logger.Debugf("使用折扣率: %d%%", dropRate)

	// 构建活动配置列表
	configList := s.buildActivityConfigsWithStrategy(products, dropRate, autoPartakeStockRatioFromStrategy(strategy), strategy.StoreID)

	if len(configList) == 0 {
		return 0, nil
	}

	activityTypes := autoPartakeActivityTypesFromStrategy(strategy)
	if err := validateAutoPartakeDiscountsForStrategy(strategy); err != nil {
		return 0, err
	}
	_, _, err := s.savePromotionConfigs(products, configList, activityTypes, strategy, "DISCOUNT")
	if err != nil {
		return 0, err
	}

	if err := s.enableSavedPromotionConfigs(ctx, configList, activityTypes); err != nil {
		return 0, fmt.Errorf("开启活动配置失败: %w", err)
	}

	return len(configList), nil
}
