// Package product 提供SHEIN通用产品管理功能
package product

import (
	"fmt"
	"net/http"
	"task-processor/internal/platforms/shein/api"
	"task-processor/internal/platforms/shein/api/product"
	"task-processor/internal/platforms/shein/client"
)

// ProductManager 产品管理器
type ProductManager struct {
	baseClient   *client.BaseAPIClient
	errorHandler *client.APIErrorHandler
}

// NewProductManager 创建产品管理器
func NewProductManager(baseClient *client.BaseAPIClient, errorHandler *client.APIErrorHandler) *ProductManager {
	return &ProductManager{
		baseClient:   baseClient,
		errorHandler: errorHandler,
	}
}

// GetProduct 获取产品信息
func (m *ProductManager) GetProduct(productID string) (*product.Product, error) {
	url := fmt.Sprintf("%s%s?product_id=%s", m.baseClient.GetBaseURL(), client.GetProductEndpoint(), productID)

	var result struct {
		api.APIResponse
		Info struct {
			Product *product.Product `json:"product"`
		} `json:"info"`
	}

	if err := m.baseClient.APIRequest(http.MethodGet, url, nil, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if err := m.errorHandler.ProcessAPIResponse(&result.APIResponse, "0", url, "获取产品信息失败"); err != nil {
		return nil, err
	}

	return result.Info.Product, nil
}

// UpdateProduct 更新产品信息
func (m *ProductManager) UpdateProduct(product *product.Product) error {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetUpdateProductEndpoint())

	var result api.APIResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, product, &result); err != nil {
		return err
	}

	// 统一错误处理
	return m.errorHandler.ProcessAPIResponse(&result, "0", url, "更新产品失败")
}

// DeleteProduct 删除产品
func (m *ProductManager) DeleteProduct(productID string) error {
	url := fmt.Sprintf("%s%s?product_id=%s", m.baseClient.GetBaseURL(), client.GetDeleteProductEndpoint(), productID)

	var result api.APIResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, nil, &result); err != nil {
		return err
	}

	// 统一错误处理
	return m.errorHandler.ProcessAPIResponse(&result, "0", url, "删除产品失败")
}

// GetPartInfo 获取部件信息
func (m *ProductManager) GetPartInfo(categoryID int) (*product.PartInfoResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetPartInfoEndpoint())

	reqBody := struct {
		CategoryID int `json:"category_id"`
	}{
		CategoryID: categoryID,
	}

	var result product.PartInfoResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	// PartInfoResponse 结构体没有 Code、Msg、BBL 字段，直接检查 Data 字段
	if result.Data == nil {
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    "获取部件信息失败: 返回数据为空",
			URL:        url,
		}
	}

	return &result, nil
}

// SaveDraftProduct 保存产品草稿
func (m *ProductManager) SaveDraftProduct(prod *product.Product) (*product.SheinResponse, string, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetSaveDraftEndpoint())

	var result product.SheinResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, prod, &result); err != nil {
		return &result, prod.SupplierCode, err
	}

	// 统一错误处理
	if result.Code != "0" {
		return &result, prod.SupplierCode, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("保存草稿产品失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, prod.SupplierCode, nil
}

// PublishProduct 发布产品
func (m *ProductManager) PublishProduct(prod *product.Product) (*product.SheinResponse, string, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetPublishProductEndpoint())

	var result product.SheinResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, prod, &result); err != nil {
		return &result, prod.SupplierCode, err
	}

	return &result, prod.SupplierCode, nil
}

// ConfirmPublish 确认发布产品
func (m *ProductManager) ConfirmPublish(product *product.Product) (bool, string, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetConfirmPublishEndpoint())

	var result struct {
		api.APIResponse
		Info struct {
			Data struct {
				NeedConfirm      bool        `json:"need_confirm"`
				SimPriceInfoList any `json:"sim_price_info_list"`
			} `json:"data"`
		} `json:"info"`
	}

	if err := m.baseClient.APIRequest(http.MethodPost, url, product, &result); err != nil {
		return false, product.SupplierCode, err
	}

	// 统一错误处理
	if err := m.errorHandler.ProcessAPIResponse(&result.APIResponse, "0", url, "发布确认失败"); err != nil {
		return false, product.SupplierCode, err
	}

	return result.Info.Data.NeedConfirm, product.SupplierCode, nil
}

// Record 获取产品发布记录
func (m *ProductManager) Record(request *product.ProductRecordRequest) (*product.RecordResponse, error) {
	url := fmt.Sprintf("%s%s?page_num=%d&page_size=%d", m.baseClient.GetBaseURL(), client.GetProductRecordEndpoint(), 1, 100)

	reqBody := product.ProductRecordRequest{
		Language:                  "en",
		OnlyCurrentMonthRecommend: false,
		OnlySpmbCopyProduct:       false,
		QueryTimeOut:              false,
		SearchDiyCustom:           false,
		SupplierCodeList:          request.SupplierCodeList,
		SupplierCodeSearchType:    request.SupplierCodeSearchType,
	}

	var result product.RecordResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("获取产品发布记录失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}

// ListProducts 获取产品列表
func (m *ProductManager) ListProducts(pageNum, pageSize int, request *product.ProductListRequest) (*product.ProductListResponse, error) {
	url := fmt.Sprintf("%s%s?page_num=%d&page_size=%d", m.baseClient.GetBaseURL(), client.GetProductListEndpoint(), pageNum, pageSize)

	var result product.ProductListResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, request, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if result.Code != "0" {
		// 检查认证过期
		if result.Code == "20302" {
			return nil, &api.AuthenticationExpiredError{
				TenantID: m.baseClient.GetTenantID(),
				ShopID:   m.baseClient.GetShopID(),
				Code:     result.Code,
				Message:  fmt.Sprintf("认证已过期: %s", result.Msg),
			}
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("获取产品列表失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}
