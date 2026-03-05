// Package common 提供TEMU平台处理器的共享接口
package common

import (
	"task-processor/internal/domain/model"
	"task-processor/internal/platforms/temu/api/models"
	temucontext "task-processor/internal/platforms/temu/context"
)

// SkuBuilderInterface SKU构建器接口
type SkuBuilderInterface interface {
	// ProcessSkcItem 处理单个SKC项
	ProcessSkcItem(temuCtx *temucontext.TemuTaskContext, skcIndex int) error

	// BuildVariantSkcs 构建变体SKC列表
	BuildVariantSkcs(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) error

	// CreateDefaultSkc 创建默认SKC
	CreateDefaultSkc(temuCtx *temucontext.TemuTaskContext) (models.Skc, error)
}

// SpecHandlerInterface 规格处理器接口
type SpecHandlerInterface interface {
	// IsSizeSpec 判断是否为尺码规格
	IsSizeSpec(specName string) bool
}
