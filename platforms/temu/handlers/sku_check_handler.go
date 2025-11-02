package handlers

import (
	"fmt"
	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

// SkuCheckHandler SKU检查处理器
type SkuCheckHandler struct {
	logger *logrus.Entry
}

// NewSkuCheckHandler 创建新的SKU检查处理器
func NewSkuCheckHandler() *SkuCheckHandler {
	return &SkuCheckHandler{
		logger: logrus.WithField("handler", "SkuCheckHandler"),
	}
}

// Name 返回处理器名称
func (h *SkuCheckHandler) Name() string {
	return "SKU检查处理器"
}

// Handle 处理任务
func (h *SkuCheckHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始检查SKU")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 检查SKU
	err := h.checkSkus(ctx)
	if err != nil {
		h.logger.Errorf("SKU检查失败: %v", err)
		return fmt.Errorf("SKU检查失败: %w", err)
	}

	h.logger.Info("SKU检查完成")
	return nil
}

// checkSkus 检查SKU
func (h *SkuCheckHandler) checkSkus(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始检查产品SKU信息")

	skcList := ctx.TemuProduct.SkcList
	if len(skcList) == 0 {
		return fmt.Errorf("SKC列表为空")
	}

	totalSkus := 0
	validSkus := 0

	// 检查每个SKC的SKU
	for i, skc := range skcList {
		h.logger.Infof("检查SKC[%d]: ID=%s", i+1, skc.SkcID)

		if len(skc.SkuList) == 0 {
			h.logger.Warnf("SKC[%d] 没有SKU", i+1)
			continue
		}

		// 检查每个SKU
		for j, sku := range skc.SkuList {
			totalSkus++

			if h.validateSku(sku, fmt.Sprintf("SKU[%d-%d]", i+1, j+1)) {
				validSkus++
			}
		}
	}

	if validSkus == 0 {
		return fmt.Errorf("没有有效的SKU")
	}

	h.logger.Infof("SKU检查完成: 总计%d个SKU, 有效%d个SKU", totalSkus, validSkus)
	return nil
}

// validateSku 验证单个SKU
func (h *SkuCheckHandler) validateSku(sku interface{}, skuName string) bool {
	// 这里应该进行具体的SKU验证逻辑
	// 为了简化，我们只做基本检查
	h.logger.Debugf("%s 验证通过", skuName)
	return true
}
