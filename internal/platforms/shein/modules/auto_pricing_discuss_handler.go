// Package modules 提供SHEIN平台的自动核价处理功能
package modules

import (
	"fmt"

	sheinapi "task-processor/internal/platforms/shein/api"
	"task-processor/internal/platforms/shein/api/pricing"

	"github.com/sirupsen/logrus"
)

// AutoPricingDiscussHandler 自动核价处理器，负责处理产品核价相关操作
type AutoPricingDiscussHandler struct{}

// NewAutoPricingDiscussHandler 创建新的自动核价处理器
func NewAutoPricingDiscussHandler() *AutoPricingDiscussHandler {
	return &AutoPricingDiscussHandler{}
}

// HandleCostDiscuss 处理产品核价
func (h *AutoPricingDiscussHandler) HandleCostDiscuss(client sheinapi.APIClient, req interface{}) error {
	// 类型断言获取具体的批量请求
	batchReq, ok := req.(*pricing.BatchHandleCostDiscussRequest)
	if !ok {
		return fmt.Errorf("批量请求类型断言失败")
	}

	response, err := client.BatchHandleCostDiscuss(batchReq)
	if err != nil {
		return fmt.Errorf("调用批量处理产品核价接口失败: %w", err)
	}

	if response.Code != "0" {
		return fmt.Errorf("批量处理产品核价接口返回错误: %s", response.Msg)
	}

	logrus.Infof("成功处理产品核价，成功处理数量: %d", response.Info.SuccessCount)
	return nil
}
