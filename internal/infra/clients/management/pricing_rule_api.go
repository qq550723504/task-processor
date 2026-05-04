package management

import (
	"encoding/json"
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/jsonx"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// PricingRuleAPIClient 自动核价规则API客户端实现
type PricingRuleAPIClient struct {
	*ManagementAPIClient
	localDataProvider *LocalDataProvider
	logger            *logrus.Entry
}

// GetPricingRule 获取自动核价规则（返回数组）
func (m *PricingRuleAPIClient) GetPricingRule(req *api.PricingRuleReqDTO) ([]api.PricingRuleRespDTO, error) {
	if m.localDataProvider != nil {
		if rules, err := m.localDataProvider.GetPricingRule(req); err != nil || rules != nil {
			return rules, err
		}
	}
	if m.logger == nil {
		m.logger = logger.GetGlobalLogger("PricingRuleAPIClient")
	}

	url := fmt.Sprintf("%s/rpc-api/listing/pricing-rule/get?storeId=%d", m.baseURL, *req.StoreID)
	m.logger.Infof("请求定价规则API: %s", url)

	var result APIResponse
	var rawData json.RawMessage
	result.Data = &rawData

	if err := m.apiRequest(http.MethodGet, url, nil, &result); err != nil {
		m.logger.Errorf("API请求失败: %v", err)
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		m.logger.Errorf("API响应处理失败: %v", err)
		return nil, err
	}

	if result.Data == nil || len(rawData) == 0 {
		m.logger.Warn("API返回的数据为空")
		return []api.PricingRuleRespDTO{}, nil
	}

	// 先尝试解析为数组
	var rulesArray []api.PricingRuleRespDTO
	if err := jsonx.UnmarshalBytes(rawData, &rulesArray, ""); err == nil {
		m.logger.Infof("成功解析为数组，共%d条规则", len(rulesArray))
		return rulesArray, nil
	}

	// 再尝试解析为单个对象
	var singleRule api.PricingRuleRespDTO
	if err := jsonx.UnmarshalBytes(rawData, &singleRule, ""); err == nil {
		m.logger.Info("成功解析为单个对象")
		return []api.PricingRuleRespDTO{singleRule}, nil
	}

	return nil, fmt.Errorf("无法解析自动核价规则数据")
}
