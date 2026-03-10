package impl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"task-processor/internal/pkg/jsonutil"
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// PricingRuleAPIClientImpl 自动核价规则API客户端实现
type PricingRuleAPIClientImpl struct {
	*ManagementAPIClientImpl
	logger *logrus.Entry
}

// GetPricingRule 获取自动核价规则（返回数组）
func (m *PricingRuleAPIClientImpl) GetPricingRule(req *api.PricingRuleReqDTO) ([]api.PricingRuleRespDTO, error) {
	if m.logger == nil {
		m.logger = logrus.WithField("component", "PricingRuleAPIClient")
	}

	// 构建URL，根据参数情况添加查询参数
	url := fmt.Sprintf("%s/rpc-api/listing/pricing-rule/get?storeId=%d", m.baseURL, *req.StoreID)
	m.logger.Infof("请求定价规则API: %s", url)

	// 使用 json.RawMessage 来处理不确定的数据结构
	var result APIResponse
	var rawData json.RawMessage
	result.Data = &rawData

	err := m.apiRequest(http.MethodGet, url, nil, &result)
	if err != nil {
		m.logger.Errorf("API请求失败: %v", err)
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		m.logger.Errorf("API响应处理失败: %v", err)
		return nil, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil || len(rawData) == 0 {
		m.logger.Warn("API返回的数据为空")
		return []api.PricingRuleRespDTO{}, nil
	}

	// 先尝试解析为数组
	var rulesArray []api.PricingRuleRespDTO
	if err := jsonutil.UnmarshalBytes(rawData, &rulesArray, ""); err == nil {
		m.logger.Infof("成功解析为数组，共%d条规则", len(rulesArray))
		return rulesArray, nil
	} else {
		m.logger.Warnf("解析为数组失败: %v", err)
	}

	// 如果数组解析失败，尝试解析为单个对象
	var singleRule api.PricingRuleRespDTO
	if err := jsonutil.UnmarshalBytes(rawData, &singleRule, ""); err == nil {
		m.logger.Info("成功解析为单个对象")
		return []api.PricingRuleRespDTO{singleRule}, nil
	} else {
		m.logger.Errorf("解析为单个对象也失败: %v", err)
	}

	return nil, fmt.Errorf("无法解析自动核价规则数据")
}
