// Package pricing 提供SHEIN通用价格管理功能
package pricing

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api/product"
	"task-processor/internal/shein/client"
)

// PriceManager 价格管理器
type PriceManager struct {
	baseClient   *client.BaseAPIClient
	errorHandler *client.APIErrorHandler
}

// NewPriceManager 创建价格管理器
func NewPriceManager(baseClient *client.BaseAPIClient, errorHandler *client.APIErrorHandler) *PriceManager {
	return &PriceManager{
		baseClient:   baseClient,
		errorHandler: errorHandler,
	}
}

// QueryPrice 查询产品价格
func (m *PriceManager) QueryPrice(spuName string) (*product.PriceQueryResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetQueryPriceEndpoint())
	var result product.PriceQueryResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, &product.PriceQueryRequest{SpuName: spuName}, &result); err != nil {
		return nil, err
	}
	if err := m.baseClient.CheckCode(result.Code, result.Msg, url, "查询价格失败"); err != nil {
		return nil, err
	}
	return &result, nil
}

// QueryCostPrice 查询成本价格（半托店铺）
func (m *PriceManager) QueryCostPrice(spuName string, skcNameList []string) (*product.CostPriceQueryResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetQueryCostPriceEndpoint())
	var result product.CostPriceQueryResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, &product.CostPriceQueryRequest{SpuName: spuName, SkcNameList: skcNameList}, &result); err != nil {
		return nil, err
	}
	if err := m.baseClient.CheckCode(result.Code, result.Msg, url, "查询成本价格失败"); err != nil {
		return nil, err
	}
	return &result, nil
}

// BatchQueryPrice 批量查询价格
func (m *PriceManager) BatchQueryPrice(spuNames []string) ([]*product.PriceQueryResponse, error) {
	results := make([]*product.PriceQueryResponse, 0, len(spuNames))

	for _, spuName := range spuNames {
		result, err := m.QueryPrice(spuName)
		if err != nil {
			return nil, fmt.Errorf("批量查询价格失败 (SPU: %s): %w", spuName, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// BatchQueryCostPrice 批量查询成本价格
func (m *PriceManager) BatchQueryCostPrice(requests []CostPriceRequest) ([]*product.CostPriceQueryResponse, error) {
	results := make([]*product.CostPriceQueryResponse, 0, len(requests))

	for _, req := range requests {
		result, err := m.QueryCostPrice(req.SpuName, req.SkcNameList)
		if err != nil {
			return nil, fmt.Errorf("批量查询成本价格失败 (SPU: %s): %w", req.SpuName, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// ValidatePriceRequest 验证价格查询请求
func (m *PriceManager) ValidatePriceRequest(spuName string) error {
	if spuName == "" {
		return fmt.Errorf("SPU名称不能为空")
	}

	return nil
}

// ValidateCostPriceRequest 验证成本价格查询请求
func (m *PriceManager) ValidateCostPriceRequest(spuName string, skcNameList []string) error {
	if spuName == "" {
		return fmt.Errorf("SPU名称不能为空")
	}

	if len(skcNameList) == 0 {
		return fmt.Errorf("SKC名称列表不能为空")
	}

	return nil
}

// CostPriceRequest 成本价格查询请求
type CostPriceRequest struct {
	SpuName     string   `json:"spu_name"`
	SkcNameList []string `json:"skc_name_list"`
}
