// Package modules 提供SHEIN平台的自动核价讨论处理功能
package modules

import (
	"fmt"

	sheinapi "task-processor/internal/common/shein/api"
	"task-processor/internal/common/shein/api/pricing"

	"github.com/sirupsen/logrus"
)

// AutoPricingDiscussHandler 自动核价讨论处理器，负责处理成本讨论相关操作
type AutoPricingDiscussHandler struct{}

// NewAutoPricingDiscussHandler 创建新的自动核价讨论处理器
// 返回值:
//   - *AutoPricingDiscussHandler: 讨论处理器实例
func NewAutoPricingDiscussHandler() *AutoPricingDiscussHandler {
	return &AutoPricingDiscussHandler{}
}

// HandleCostDiscuss 处理成本讨论
// 参数:
//   - client: SHEIN API客户端
//   - req: 批量处理成本讨论请求
//
// 返回值:
//   - error: 错误信息
func (h *AutoPricingDiscussHandler) HandleCostDiscuss(client sheinapi.APIClient, req *pricing.BatchHandleCostDiscussRequest) error {
	response, err := client.BatchHandleCostDiscuss(req)
	if err != nil {
		return fmt.Errorf("调用批量处理成本讨论接口失败: %w", err)
	}

	if response.Code != "0" {
		return fmt.Errorf("批量处理成本讨论接口返回错误: %s", response.Msg)
	}

	logrus.Infof("成功处理成本讨论，成功处理数量: %d", response.Info.SuccessCount)
	return nil
}
