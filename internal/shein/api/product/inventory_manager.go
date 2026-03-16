package product

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api"
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

	if result.Code != "0" {
		if result.Code == "20302" {
			return nil, &api.AuthenticationExpiredError{
				TenantID: m.baseClient.GetTenantID(),
				ShopID:   m.baseClient.GetShopID(),
				Code:     result.Code,
				Message:  fmt.Sprintf("认证已过期: %s", result.Msg),
			}
		}
		return nil, &api.APIError{StatusCode: 0, Message: fmt.Sprintf("查询库存失败: %s", result.Msg), URL: url}
	}

	return &result, nil
}

func (m *inventoryManager) queryInventory(spuName string) (*InventoryQueryResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetQueryInventoryEndpoint())

	request := &InventoryQueryRequest{SpuName: spuName}

	var result InventoryQueryResponse
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
		return nil, &api.APIError{StatusCode: 0, Message: fmt.Sprintf("查询库存详情失败: %s", result.Msg), URL: url}
	}

	return &result, nil
}

func (m *inventoryManager) updateInventory(request *InventoryUpdateRequest) error {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetUpdateInventoryEndpoint())

	var result InventoryUpdateResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, request, &result); err != nil {
		return err
	}

	if result.Code != "0" {
		if result.Code == "20302" {
			return &api.AuthenticationExpiredError{
				TenantID: m.baseClient.GetTenantID(),
				ShopID:   m.baseClient.GetShopID(),
				Code:     result.Code,
				Message:  fmt.Sprintf("认证已过期: %s", result.Msg),
			}
		}
		return &api.APIError{StatusCode: 0, Message: fmt.Sprintf("更新库存失败: %s", result.Msg), URL: url}
	}

	return nil
}
