package impl

import (
	"fmt"
	"net/http"
	"task-processor/internal/platforms/shein/client/api"
	"task-processor/internal/platforms/shein/client/api/product"
)

// ProductAPI 产品相关API实现
type ProductAPI struct {
	*BaseAPIClient
}

// NewProductAPI 创建新的产品API实现
func NewProductAPI(baseClient *BaseAPIClient) *ProductAPI {
	return &ProductAPI{
		BaseAPIClient: baseClient,
	}
}

// GetProduct 获取产品信息
func (p *ProductAPI) GetProduct(productID string) (*product.Product, error) {
	url := fmt.Sprintf("%s%s?product_id=%s", p.GetBaseURL(), getProductEndpoint, productID)

	var result struct {
		api.APIResponse
		Info struct {
			Product *product.Product `json:"product"`
		} `json:"info"`
	}

	if err := p.apiRequest(http.MethodGet, url, nil, &result); err != nil {
		return nil, err
	}

	// 统一错误处理 - 使用 ProcessAPIResponse 检查认证过期
	if err := p.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		// 其他错误，包装为 APIError
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("获取产品信息失败: %s", result.Msg),
			URL:        url,
		}
	}

	return result.Info.Product, nil
}

// UpdateProduct 更新产品信息
func (p *ProductAPI) UpdateProduct(product *product.Product) error {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), updateProductEndpoint)

	var result api.APIResponse
	if err := p.apiRequest(http.MethodPost, url, product, &result); err != nil {
		return err
	}

	// 统一错误处理 - 使用 ProcessAPIResponse 检查认证过期
	if err := p.ProcessAPIResponse(&result, "0"); err != nil {
		// 如果是认证过期错误，直接返回
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return err
		}
		// 其他错误，包装为 APIError
		return &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("更新产品失败: %s", result.Msg),
			URL:        url,
		}
	}

	return nil
}

// DeleteProduct 删除产品
func (p *ProductAPI) DeleteProduct(productID string) error {
	url := fmt.Sprintf("%s%s?product_id=%s", p.GetBaseURL(), deleteProductEndpoint, productID)

	var result api.APIResponse
	if err := p.apiRequest(http.MethodPost, url, nil, &result); err != nil {
		return err
	}

	// 统一错误处理 - 使用 ProcessAPIResponse 检查认证过期
	if err := p.ProcessAPIResponse(&result, "0"); err != nil {
		// 如果是认证过期错误，直接返回
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return err
		}
		// 其他错误，包装为 APIError
		return &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("删除产品失败: %s", result.Msg),
			URL:        url,
		}
	}

	return nil
}

// GetPartInfo 获取部件信息
func (p *ProductAPI) GetPartInfo(categoryID int) (*product.PartInfoResponse, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), getPartInfoEndpoint)

	reqBody := struct {
		CategoryID int `json:"category_id"`
	}{
		CategoryID: categoryID,
	}

	var result product.PartInfoResponse
	if err := p.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	// PartInfoResponse 结构体没有 Code、Msg、BBL 字段，直接检查 Data 字段
	if result.Data == nil {
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    "获取部件信息失败: 返回数据为空",
			URL:        url,
		}
	}

	return &result, nil
}

// SaveDraftProduct 保存产品草稿
func (p *ProductAPI) SaveDraftProduct(prod *product.Product) (*product.SheinResponse, string, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), saveDraftEndpoint)

	var result product.SheinResponse
	if err := p.apiRequest(http.MethodPost, url, prod, &result); err != nil {
		return &result, prod.SupplierCode, err
	}

	// 统一错误处理
	if result.Code != "0" {
		return &result, prod.SupplierCode, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("保存草稿产品失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, prod.SupplierCode, nil
}

// PublishProduct 发布产品
func (p *ProductAPI) PublishProduct(prod *product.Product) (*product.SheinResponse, string, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), publishProductEndpoint)

	var result product.SheinResponse
	if err := p.apiRequest(http.MethodPost, url, prod, &result); err != nil {
		return &result, prod.SupplierCode, err
	}

	return &result, prod.SupplierCode, nil
}

// ConfirmPublish 确认发布产品
func (p *ProductAPI) ConfirmPublish(product *product.Product) (bool, string, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), confirmPublishEndpoint)

	var result struct {
		api.APIResponse
		Info struct {
			Data struct {
				NeedConfirm      bool        `json:"need_confirm"`
				SimPriceInfoList interface{} `json:"sim_price_info_list"`
			} `json:"data"`
		} `json:"info"`
	}

	if err := p.apiRequest(http.MethodPost, url, product, &result); err != nil {
		return false, product.SupplierCode, err
	}

	// 统一错误处理 - 使用 ProcessAPIResponse 检查认证过期
	if err := p.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return false, product.SupplierCode, err
		}
		// 其他错误，包装为 APIError
		return false, product.SupplierCode, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("发布确认失败: %s", result.Msg),
			URL:        url,
		}
	}

	return result.Info.Data.NeedConfirm, product.SupplierCode, nil
}

// Record 获取产品发布记录
func (p *ProductAPI) Record(request *product.ProductRecordRequest) (*product.RecordResponse, error) {
	url := fmt.Sprintf("%s%s?page_num=%d&page_size=%d", p.GetBaseURL(), productRecordEndpoint, 1, 100)

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
	if err := p.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("获取产品发布记录失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}

// ListProducts 获取产品列表
func (p *ProductAPI) ListProducts(pageNum, pageSize int, request *product.ProductListRequest) (*product.ProductListResponse, error) {
	url := fmt.Sprintf("%s%s?page_num=%d&page_size=%d", p.GetBaseURL(), productListEndpoint, pageNum, pageSize)

	var result product.ProductListResponse
	if err := p.apiRequest(http.MethodPost, url, request, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if result.Code != "0" {
		// 检查认证过期
		if result.Code == "20302" {
			return nil, &api.AuthenticationExpiredError{
				TenantID: p.GetTenantID(),
				ShopID:   p.GetShopID(),
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

// QueryStock 查询产品库存
func (p *ProductAPI) QueryStock(request *product.StockQueryRequest) (*product.StockQueryResponse, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), queryStockEndpoint)

	var result product.StockQueryResponse
	if err := p.apiRequest(http.MethodPost, url, request, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if result.Code != "0" {
		// 检查认证过期
		if result.Code == "20302" {
			return nil, &api.AuthenticationExpiredError{
				TenantID: p.GetTenantID(),
				ShopID:   p.GetShopID(),
				Code:     result.Code,
				Message:  fmt.Sprintf("认证已过期: %s", result.Msg),
			}
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("查询库存失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}

// QueryPrice 查询产品价格
func (p *ProductAPI) QueryPrice(spuName string) (*product.PriceQueryResponse, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), queryPriceEndpoint)

	request := &product.PriceQueryRequest{
		SpuName: spuName,
	}

	var result product.PriceQueryResponse
	if err := p.apiRequest(http.MethodPost, url, request, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if result.Code != "0" {
		// 检查认证过期
		if result.Code == "20302" {
			return nil, &api.AuthenticationExpiredError{
				TenantID: p.GetTenantID(),
				ShopID:   p.GetShopID(),
				Code:     result.Code,
				Message:  fmt.Sprintf("认证已过期: %s", result.Msg),
			}
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("查询价格失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}

// QueryInventory 查询产品库存详情
func (p *ProductAPI) QueryInventory(spuName string) (*product.InventoryQueryResponse, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), queryInventoryEndpoint)

	request := &product.InventoryQueryRequest{
		SpuName: spuName,
	}

	var result product.InventoryQueryResponse
	if err := p.apiRequest(http.MethodPost, url, request, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if result.Code != "0" {
		// 检查认证过期
		if result.Code == "20302" {
			return nil, &api.AuthenticationExpiredError{
				TenantID: p.GetTenantID(),
				ShopID:   p.GetShopID(),
				Code:     result.Code,
				Message:  fmt.Sprintf("认证已过期: %s", result.Msg),
			}
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("查询库存详情失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}

// UpdateInventory 更新产品库存
func (p *ProductAPI) UpdateInventory(request *product.InventoryUpdateRequest) error {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), updateInventoryEndpoint)

	var result product.InventoryUpdateResponse
	if err := p.apiRequest(http.MethodPost, url, request, &result); err != nil {
		return err
	}

	// 统一错误处理
	if result.Code != "0" {
		// 检查认证过期
		if result.Code == "20302" {
			return &api.AuthenticationExpiredError{
				TenantID: p.GetTenantID(),
				ShopID:   p.GetShopID(),
				Code:     result.Code,
				Message:  fmt.Sprintf("认证已过期: %s", result.Msg),
			}
		}
		return &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("更新库存失败: %s", result.Msg),
			URL:        url,
		}
	}

	return nil
}

// QueryCostPrice 查询成本价格（半托店铺）
func (p *ProductAPI) QueryCostPrice(spuName string, skcNameList []string) (*product.CostPriceQueryResponse, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), queryCostPriceEndpoint)

	request := &product.CostPriceQueryRequest{
		SpuName:     spuName,
		SkcNameList: skcNameList,
	}

	var result product.CostPriceQueryResponse
	if err := p.apiRequest(http.MethodPost, url, request, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if result.Code != "0" {
		// 检查认证过期
		if result.Code == "20302" {
			return nil, &api.AuthenticationExpiredError{
				TenantID: p.GetTenantID(),
				ShopID:   p.GetShopID(),
				Code:     result.Code,
				Message:  fmt.Sprintf("认证已过期: %s", result.Msg),
			}
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("查询成本价格失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}

// OffShelf 下架产品
func (p *ProductAPI) OffShelf(request *product.ShelfOperateRequest) error {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), operateShelfStatusEndpoint)

	var result product.ShelfOperateResponse
	if err := p.apiRequest(http.MethodPost, url, request, &result); err != nil {
		return err
	}

	// 统一错误处理
	if result.Code != "0" {
		// 检查认证过期
		if result.Code == "20302" {
			return &api.AuthenticationExpiredError{
				TenantID: p.GetTenantID(),
				ShopID:   p.GetShopID(),
				Code:     result.Code,
				Message:  fmt.Sprintf("认证已过期: %s", result.Msg),
			}
		}
		return &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("下架产品失败: %s", result.Msg),
			URL:        url,
		}
	}

	return nil
}

// OnShelf 上架产品
func (p *ProductAPI) OnShelf(request *product.ShelfOperateRequest) error {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), operateShelfStatusEndpoint)

	var result product.ShelfOperateResponse
	if err := p.apiRequest(http.MethodPost, url, request, &result); err != nil {
		return err
	}

	// 统一错误处理
	if result.Code != "0" {
		// 检查认证过期
		if result.Code == "20302" {
			return &api.AuthenticationExpiredError{
				TenantID: p.GetTenantID(),
				ShopID:   p.GetShopID(),
				Code:     result.Code,
				Message:  fmt.Sprintf("认证已过期: %s", result.Msg),
			}
		}
		return &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("上架产品失败: %s", result.Msg),
			URL:        url,
		}
	}

	return nil
}
