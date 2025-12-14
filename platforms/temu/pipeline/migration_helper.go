// Package pipeline TEMU迁移辅助工具
package pipeline

import (
	"context"
	"task-processor/common/amazon/model"
	"task-processor/common/management/api"
	commonPipeline "task-processor/common/pipeline"
	"task-processor/common/types"
	temuContext "task-processor/platforms/temu/context"
	temutypes "task-processor/platforms/temu/types"
)

// MigrationHelper TEMU迁移辅助工具
type MigrationHelper struct{}

// NewMigrationHelper 创建迁移辅助工具
func NewMigrationHelper() *MigrationHelper {
	return &MigrationHelper{}
}

// ConvertLegacyContext 将旧的TaskContext转换为新的TemuTaskContext
func (mh *MigrationHelper) ConvertLegacyContext(legacyCtx *commonPipeline.BaseTaskContext) *temuContext.TemuTaskContext {
	// 注意：这里需要手动构造，因为我们不能直接访问私有字段
	// 应该使用 NewTemuTaskContext 然后复制数据
	task := legacyCtx.GetTask()
	temuCtx := temuContext.NewTemuTaskContext(legacyCtx.GetContext(), task)

	// 迁移通用数据到强类型字段
	mh.migrateDataToTypedFields(temuCtx)

	return temuCtx
}

// migrateDataToTypedFields 将通用数据迁移到强类型字段
func (mh *MigrationHelper) migrateDataToTypedFields(temuCtx *temuContext.TemuTaskContext) {
	// 迁移Amazon产品数据
	if amazonProduct, exists := temuCtx.GetData("amazon_product"); exists {
		if product, ok := amazonProduct.(*model.Product); ok {
			temuCtx.SetAmazonProduct(product)
		}
	}

	// 迁移TEMU产品数据
	if temuProduct, exists := temuCtx.GetData("temu_product"); exists {
		if product, ok := temuProduct.(*temutypes.Product); ok {
			temuCtx.SetTemuProduct(product)
		}
	}

	// 迁移变体数据
	if variants, exists := temuCtx.GetData("variants"); exists {
		if variantList, ok := variants.([]*model.Product); ok {
			temuCtx.SetAmazonVariants(variantList)
		}
	}

	// 迁移店铺信息
	if storeInfo, exists := temuCtx.GetData("store_info"); exists {
		if store, ok := storeInfo.(*api.StoreRespDTO); ok {
			temuCtx.SetStoreInfo(store)
		}
	}
}

// CreateTemuContextFromTask 从任务创建TEMU上下文（推荐的新方式）
func (mh *MigrationHelper) CreateTemuContextFromTask(ctx context.Context, task *types.Task) *temuContext.TemuTaskContext {
	return temuContext.NewTemuTaskContext(ctx, task)
}
