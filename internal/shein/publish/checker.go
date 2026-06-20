// Package publish 提供SHEIN平台产品发布前检查功能
package publish

import (
	"context"
	"fmt"
	"task-processor/internal/core/logger"

	"task-processor/internal/shein"
)

// PublishProductChecker 产品发布前检查器
type PublishProductChecker struct {
}

// NewPublishProductChecker 创建新的产品发布前检查器
func NewPublishProductChecker() *PublishProductChecker {
	return &PublishProductChecker{}
}

// CheckProductExists 检查产品是否已上架
func (c *PublishProductChecker) CheckProductExists(ctx *shein.TaskContext) error {
	input, err := buildExistenceCheckInput(ctx)
	if err != nil {
		return err
	}
	// 检查必要的上下文信息
	if input.RuntimeRepository == nil {
		logger.GetGlobalLogger("shein/publish").Warn("管理客户端管理器未初始化，跳过产品存在性检查")
		return nil
	}

	if input.Task == nil {
		logger.GetGlobalLogger("shein/publish").Warn("任务信息未初始化，跳过产品存在性检查")
		return nil
	}

	// 检查主产品是否已上架
	if input.Task.ProductID != "" {
		exists, err := checkPublishedProductExists(context.Background(), input.RuntimeRepository, input.Task.StoreID, input.Task.Platform, input.Task.Region, input.Task.ProductID)
		if err != nil {
			logger.GetGlobalLogger("shein/publish").Errorf("检查产品 %s 是否已上架失败: %v", input.Task.ProductID, err)
			return err
		}

		if exists {
			logger.GetGlobalLogger("shein/publish").Warnf("⚠️ 产品 %s 已经上架过，跳过本次上架", input.Task.ProductID)
			return shein.NewNonRetryableError(fmt.Sprintf("产品 %s 已经上架过", input.Task.ProductID), nil)
		}

		logger.GetGlobalLogger("shein/publish").Infof("✅ 产品 %s 未上架，可以继续上架流程", input.Task.ProductID)
	}

	// 检查所有变体是否已上架
	if input.Variants != nil && len(*input.Variants) > 0 {
		for _, variant := range *input.Variants {
			if variant.Asin == "" {
				continue
			}

			exists, err := checkPublishedProductExists(context.Background(), input.RuntimeRepository, input.Task.StoreID, input.Task.Platform, input.Task.Region, variant.Asin)
			if err != nil {
				logger.GetGlobalLogger("shein/publish").Errorf("检查变体 %s 是否已上架失败: %v", variant.Asin, err)
				// 单个变体检查失败不影响整体流程，继续检查下一个
				continue
			}

			if exists {
				logger.GetGlobalLogger("shein/publish").Warnf("⚠️ 变体 %s 已经上架过", variant.Asin)
				// 标记该变体已被筛选掉
				if input.SetVariantFilteredFn != nil {
					input.SetVariantFilteredFn(variant.Asin, true, fmt.Sprintf("产品 %s 已经上架过", variant.Asin))
				}
			} else {
				logger.GetGlobalLogger("shein/publish").Debugf("✅ 变体 %s 未上架", variant.Asin)
			}
		}
	}

	return nil
}
