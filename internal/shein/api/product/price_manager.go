package product

import (
	"fmt"
	"net/http"
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
	var result PriceQueryResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, &PriceQueryRequest{SpuName: spuName}, &result); err != nil {
		return nil, err
	}
	if err := m.baseClient.CheckCode(result.Code, result.Msg, url, "查询价格失败"); err != nil {
		return nil, err
	}
	return &result, nil
}

func (m *priceManager) queryCostPrice(spuName string, skcNameList []string) (*CostPriceQueryResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetQueryCostPriceEndpoint())
	var result CostPriceQueryResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, &CostPriceQueryRequest{SpuName: spuName, SkcNameList: skcNameList}, &result); err != nil {
		return nil, err
	}
	if err := m.baseClient.CheckCode(result.Code, result.Msg, url, "查询成本价格失败"); err != nil {
		return nil, err
	}
	return &result, nil
}
