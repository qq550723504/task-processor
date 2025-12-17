package handlers

import (
	"task-processor/internal/common/management/api"
	"task-processor/internal/common/pipeline"

	"github.com/sirupsen/logrus"
)

// VariantFilterHandler 变体筛选处理器
type VariantFilterHandler struct {
	logger           *logrus.Entry
	filterRuleClient api.FilterRuleAPI
	filterHandler    *FilterRuleHandler
}

// NewVariantFilterHandler 创建新的变体筛选处理器
func NewVariantFilterHandler(filterRuleClient api.FilterRuleAPI) *VariantFilterHandler {
	return &VariantFilterHandler{
		logger:           logrus.WithField("handler", "VariantFilterHandler"),
		filterRuleClient: filterRuleClient,
		filterHandler:    NewFilterRuleHandler(filterRuleClient),
	}
}

// Name 返回处理器名称
func (h *VariantFilterHandler) Name() string {
	return "变体筛选处理器"
}

// Handle 处理任务 - 筛选变体产品
func (h *VariantFilterHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始筛选变体产品")

	// 调用筛选规则处理器的变体筛选方法
	if err := h.filterHandler.FilterVariants(ctx); err != nil {
		return err
	}

	h.logger.Info("变体筛选完成")
	return nil
}
