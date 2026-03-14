package sku

import (
	"fmt"
	"task-processor/internal/pipeline"
	"task-processor/internal/infra/clients/management/api"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/handlers/filter"

	"github.com/sirupsen/logrus"
)

// VariantFilterHandler 变体筛选处理器
type VariantFilterHandler struct {
	logger           *logrus.Entry
	filterRuleClient api.FilterRuleAPI
	filterHandler    *filter.FilterRuleHandler
}

// NewVariantFilterHandler 创建新的变体筛选处理器
func NewVariantFilterHandler(filterRuleClient api.FilterRuleAPI) *VariantFilterHandler {
	return &VariantFilterHandler{
		logger:           logrus.WithField("handler", "VariantFilterHandler"),
		filterRuleClient: filterRuleClient,
		filterHandler:    filter.NewFilterRuleHandler(filterRuleClient),
	}
}

// Name 返回处理器名称
func (h *VariantFilterHandler) Name() string {
	return "变体筛选处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *VariantFilterHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务 - 筛选变体产品（强类型上下文）
func (h *VariantFilterHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始筛选变体产品")

	// 调用筛选规则处理器的变体筛选方法
	if err := h.filterHandler.FilterVariants(temuCtx); err != nil {
		return err
	}

	h.logger.Info("变体筛选完成")
	return nil
}
