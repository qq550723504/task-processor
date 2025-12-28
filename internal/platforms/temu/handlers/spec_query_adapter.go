// Package handlers 提供规格查询适配器
package handlers

import (
	"fmt"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// SkuBuilderAdapter 适配器，为SkuVariantProcessor提供规格查询能力
type SkuBuilderAdapter struct {
	logger *logrus.Entry
}

// QuerySpecID 实现SpecQueryAPI接口，委托给临时的SkuBuilder处理
func (adapter *SkuBuilderAdapter) QuerySpecID(ctx pipeline.TaskContext, parentSpecID, specName string) (string, error) {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return "", fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}

	// 创建临时的SkuBuilder来处理API调用
	tempSkuBuilder := &SkuBuilder{
		logger: adapter.logger,
	}

	return tempSkuBuilder.querySpecID(temuCtx, parentSpecID, specName)
}
