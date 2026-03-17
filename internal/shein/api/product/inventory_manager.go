package product

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/client"
)

type inventoryManager struct {
	baseClient   *client.BaseAPIClient
	errorHandler *client.APIErrorHandler
}

func newInventoryManager(baseClient *client.BaseAPIClient, errorHandler *client.APIErrorHandler) *inventoryManager {
	return &inventoryManager{baseClient: baseClient, errorHandler: errorHandler}
}

// InventoryManager 导出的库存管理器，供外部包直接使用
type InventoryManager struct {
	*inventoryManager
}

// NewInventoryManager 创建导出的库存管理器
func NewInventoryManager(baseClient *client.BaseAPIClient, errorHandler *client.APIErrorHandler) *InventoryManager {
	return &InventoryManager{inventoryManager: newInventoryManager(baseClient, errorHandler)}
}

// QueryStock 查询库存（导出方法）
func (m *InventoryManager) QueryStock(request *StockQueryRequest) (*StockQueryResponse, error) {
	return m.queryStock(request)
}

// QueryInventory 查询库存详情（导出方法）
func (m *InventoryManager) QueryInventory(spuName string) (*InventoryQueryResponse, error) {
	return m.queryInventory(spuName)
}

// UpdateInventory 更新库存（导出方法）
func (m *InventoryManager) UpdateInventory(request *InventoryUpdateRequest) error {
	return m.updateInventory(request)
}

func (m *inventoryManager) queryStock(request *StockQueryRequest) (*StockQueryResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetQueryStockEndpoint())
	var result StockQueryResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, request, &result); err != nil {
		return nil, err
	}
	if err := m.baseClient.CheckCode(result.Code, result.Msg, url, "查询库存失败"); err != nil {
		return nil, err
	}
	return &result, nil
}

func (m *inventoryManager) queryInventory(spuName string) (*InventoryQueryResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetQueryInventoryEndpoint())
	var result InventoryQueryResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, &InventoryQueryRequest{SpuName: spuName}, &result); err != nil {
		return nil, err
	}
	if err := m.baseClient.CheckCode(result.Code, result.Msg, url, "查询库存详情失败"); err != nil {
		return nil, err
	}
	return &result, nil
}

func (m *inventoryManager) updateInventory(request *InventoryUpdateRequest) error {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetUpdateInventoryEndpoint())
	var result InventoryUpdateResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, request, &result); err != nil {
		return err
	}
	return m.baseClient.CheckCode(result.Code, result.Msg, url, "更新库存失败")
}
