package impl

import (
	"fmt"
	"net/http"
	"task-processor/common/shein/api"
	"task-processor/common/shein/api/product"
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
