package product

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api"
	"task-processor/internal/shein/client"
)

type priceManager struct {
	baseClient   *client.BaseAPIClient
	errorHandler *client.APIErrorHandler
}

func newPriceManager(baseClient *client.BaseAPIClient, errorHandler *client.APIErrorHandler) *priceManager {
	return &priceManager{baseClient: baseClient, errorHandler: errorHandler}
}

// PriceManager 导出的价格管理器，供外部包直接使用
type PriceManager struct {
	*priceManager
}

// NewPriceManager 创建导出的价格管理器
func NewPriceManager(baseClient *client.BaseAPIClient, errorHandler *client.APIErrorHandler) *PriceManager {
	return &PriceManager{priceManager: newPriceManager(baseClient, errorHandler)}
}

// QueryPrice 查询价格（导出方法）
func (m *PriceManager) QueryPrice(spuName string) (*PriceQueryResponse, error) {
	return m.queryPrice(spuName)
}

// QueryCostPrice 查询成本价格（导出方法）
func (m *PriceManager) QueryCostPrice(spuName string, skcNameList []string) (*CostPriceQueryResponse, error) {
	return m.queryCostPrice(spuName, skcNameList)
}

func (m *priceManager) queryPrice(spuName string) (*PriceQueryResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetQueryPriceEndpoint())

	request := &PriceQueryRequest{SpuName: spuName}

	var result PriceQueryResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, request, &result); err != nil {
		return nil, err
	}

	if result.Code != "0" {
		if result.Code == "20302" {
			return nil, &api.AuthenticationExpiredError{
				TenantID: m.baseClient.GetTenantID(),
				ShopID:   m.baseClient.GetShopID(),
				Code:     result.Code,
				Message:  fmt.Sprintf("认证已过期: %s", result.Msg),
			}
		}
		return nil, &api.APIError{StatusCode: 0, Message: fmt.Sprintf("查询价格失败: %s", result.Msg), URL: url}
	}

	return &result, nil
}

func (m *priceManager) queryCostPrice(spuName string, skcNameList []string) (*CostPriceQueryResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetQueryCostPriceEndpoint())

	request := &CostPriceQueryRequest{SpuName: spuName, SkcNameList: skcNameList}

	var result CostPriceQueryResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, request, &result); err != nil {
		return nil, err
	}

	if result.Code != "0" {
		if result.Code == "20302" {
			return nil, &api.AuthenticationExpiredError{
				TenantID: m.baseClient.GetTenantID(),
				ShopID:   m.baseClient.GetShopID(),
				Code:     result.Code,
				Message:  fmt.Sprintf("认证已过期: %s", result.Msg),
			}
		}
		return nil, &api.APIError{StatusCode: 0, Message: fmt.Sprintf("查询成本价格失败: %s", result.Msg), URL: url}
	}

	return &result, nil
}
