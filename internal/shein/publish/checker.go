// Package publish 提供SHEIN平台产品发布前检查功能
package publish

import (
	"task-processor/internal/core/logger"
	"fmt"

	management_api "task-processor/internal/infra/clients/management/api"
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
	// 检查必要的上下文信息
	if ctx.ManagementClientMgr == nil {
		logger.GetGlobalLogger("shein/publish").Warn("管理客户端管理器未初始化，跳过产品存在性检查")
		return nil
	}

	if ctx.Task == nil {
		logger.GetGlobalLogger("shein/publish").Warn("任务信息未初始化，跳过产品存在性检查")
		return nil
	}

	// 获取产品导入映射客户端
	mappingClient := ctx.ManagementClientMgr.GetProductImportMappingClient()
	if mappingClient == nil {
		logger.GetGlobalLogger("shein/publish").Warn("产品导入映射客户端未初始化，跳过产品存在性检查")
		return nil
	}

	// 检查主产品是否已上架
	if ctx.Task.ProductID != "" {
		req := &management_api.ProductImportMappingCheckReqDTO{
			StoreId:   ctx.Task.StoreID,
			Platform:  ctx.Task.Platform,
			Region:    ctx.Task.Region,
			ProductId: ctx.Task.ProductID,
		}

		exists, err := mappingClient.CheckProductExists(req)
		if err != nil {
			logger.GetGlobalLogger("shein/publish").Errorf("检查产品 %s 是否已上架失败: %v", ctx.Task.ProductID, err)
			return err
		}

		if exists {
			logger.GetGlobalLogger("shein/publish").Warnf("⚠️ 产品 %s 已经上架过，跳过本次上架", ctx.Task.ProductID)
			return shein.NewNonRetryableError(fmt.Sprintf("产品 %s 已经上架过", ctx.Task.ProductID), nil)
		}

		logger.GetGlobalLogger("shein/publish").Infof("✅ 产品 %s 未上架，可以继续上架流程", ctx.Task.ProductID)
	}

	// 检查所有变体是否已上架
	if ctx.Variants != nil && len(*ctx.Variants) > 0 {
		for _, variant := range *ctx.Variants {
			if variant.Asin == "" {
				continue
			}

			req := &management_api.ProductImportMappingCheckReqDTO{
				StoreId:   ctx.Task.StoreID,
				Platform:  ctx.Task.Platform,
				Region:    ctx.Task.Region,
				ProductId: variant.Asin,
			}

			exists, err := mappingClient.CheckProductExists(req)
			if err != nil {
				logger.GetGlobalLogger("shein/publish").Errorf("检查变体 %s 是否已上架失败: %v", variant.Asin, err)
				// 单个变体检查失败不影响整体流程，继续检查下一个
				continue
			}

			if exists {
				logger.GetGlobalLogger("shein/publish").Warnf("⚠️ 变体 %s 已经上架过", variant.Asin)
				// 标记该变体已被筛选掉
				ctx.SetVariantFiltered(variant.Asin, true, fmt.Sprintf("产品 %s 已经上架过", variant.Asin))
			} else {
				logger.GetGlobalLogger("shein/publish").Debugf("✅ 变体 %s 未上架", variant.Asin)
			}
		}
	}

	return nil
}
