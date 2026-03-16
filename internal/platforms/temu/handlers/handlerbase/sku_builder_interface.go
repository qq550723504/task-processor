// Package handlerbase 提供TEMU平台处理器的共享接口
package handlerbase

import (
	"task-processor/internal/domain/model"
	models "task-processor/internal/platforms/temu/api/product"
	temucontext "task-processor/internal/platforms/temu/context"
)

// SkuBuilder SKU构建器接口
type SkuBuilder interface {
	// ProcessSkcItem 处理单个SKC项
	ProcessSkcItem(temuCtx *temucontext.TemuTaskContext, skcIndex int) error

	// BuildVariantSkcs 构建变体SKC列表
	BuildVariantSkcs(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) error

	// CreateDefaultSkc 创建默认SKC
	CreateDefaultSkc(temuCtx *temucontext.TemuTaskContext) (models.Skc, error)
}

// SpecHandler 规格处理器接口
type SpecHandler interface {
	// IsSizeSpec 判断是否为尺码规格
	IsSizeSpec(specName string) bool
}
