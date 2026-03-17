// Package product 提供SHEIN通用库存管理功能
package product

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api/product"
	"task-processor/internal/shein/client"
)

// InventoryManager 库存管理器
type InventoryManager struct {
	baseClient   *client.BaseAPIClient
	errorHandler *client.APIErrorHandler
}

// NewInventoryManager 创建库存管理器
func NewInventoryManager(baseClient *client.BaseAPIClient, errorHandler *client.APIErrorHandler) *InventoryManager {
	return &InventoryManager{
		baseClient:   baseClient,
		errorHandler: errorHandler,
	}
}

// QueryStock 查询产品库存
func (m *InventoryManager) QueryStock(request *product.StockQueryRequest) (*product.StockQueryResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetQueryStockEndpoint())
	var result product.StockQueryResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, request, &result); err != nil {
		return nil, err
	}
	if err := m.baseClient.CheckCode(result.Code, result.Msg, url, "查询库存失败"); err != nil {
		return nil, err
	}
	return &result, nil
}

// QueryInventory 查询产品库存详情
func (m *InventoryManager) QueryInventory(spuName string) (*product.InventoryQueryResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetQueryInventoryEndpoint())
	var result product.InventoryQueryResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, &product.InventoryQueryRequest{SpuName: spuName}, &result); err != nil {
		return nil, err
	}
	if err := m.baseClient.CheckCode(result.Code, result.Msg, url, "查询库存详情失败"); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateInventory 更新产品库存
func (m *InventoryManager) UpdateInventory(request *product.InventoryUpdateRequest) error {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetUpdateInventoryEndpoint())
	var result product.InventoryUpdateResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, request, &result); err != nil {
		return err
	}
	return m.baseClient.CheckCode(result.Code, result.Msg, url, "更新库存失败")
}

// BatchQueryStock 批量查询库存
func (m *InventoryManager) BatchQueryStock(requests []*product.StockQueryRequest) ([]*product.StockQueryResponse, error) {
	results := make([]*product.StockQueryResponse, 0, len(requests))

	for _, request := range requests {
		result, err := m.QueryStock(request)
		if err != nil {
			return nil, fmt.Errorf("批量查询库存失败: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// BatchUpdateInventory 批量更新库存
func (m *InventoryManager) BatchUpdateInventory(requests []*product.InventoryUpdateRequest) error {
	for i, request := range requests {
		if err := m.UpdateInventory(request); err != nil {
			return fmt.Errorf("批量更新库存失败 (第%d个): %w", i+1, err)
		}
	}

	return nil
}

// ValidateInventoryRequest 验证库存请求
func (m *InventoryManager) ValidateInventoryRequest(request *product.InventoryUpdateRequest) error {
	if request == nil {
		return fmt.Errorf("库存更新请求不能为空")
	}

	// 添加更多验证逻辑
	return nil
}
