package product

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api"
	"task-processor/internal/shein/client"
)

type productManager struct {
	baseClient   *client.BaseAPIClient
	errorHandler *client.APIErrorHandler
}

func newProductManager(baseClient *client.BaseAPIClient, errorHandler *client.APIErrorHandler) *productManager {
	return &productManager{baseClient: baseClient, errorHandler: errorHandler}
}

func (m *productManager) getProduct(productID string) (*Product, error) {
	url := fmt.Sprintf("%s%s?product_id=%s", m.baseClient.GetBaseURL(), client.GetProductEndpoint(), productID)

	var result struct {
		api.APIResponse
		Info struct {
			Product *Product `json:"product"`
		} `json:"info"`
	}

	if err := m.baseClient.APIRequest(http.MethodGet, url, nil, &result); err != nil {
		return nil, err
	}

	if err := m.errorHandler.ProcessAPIResponse(&result.APIResponse, "0", url, "获取产品信息失败"); err != nil {
		return nil, err
	}

	return result.Info.Product, nil
}

func (m *productManager) updateProduct(product *Product) error {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetUpdateProductEndpoint())

	var result api.APIResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, product, &result); err != nil {
		return err
	}

	return m.errorHandler.ProcessAPIResponse(&result, "0", url, "更新产品失败")
}

func (m *productManager) deleteProduct(productID string) error {
	url := fmt.Sprintf("%s%s?product_id=%s", m.baseClient.GetBaseURL(), client.GetDeleteProductEndpoint(), productID)

	var result api.APIResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, nil, &result); err != nil {
		return err
	}

	return m.errorHandler.ProcessAPIResponse(&result, "0", url, "删除产品失败")
}

func (m *productManager) getPartInfo(categoryID int) (*PartInfoResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetPartInfoEndpoint())

	reqBody := struct {
		CategoryID int `json:"category_id"`
	}{CategoryID: categoryID}

	var result PartInfoResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, &api.APIError{StatusCode: 0, Message: "获取部件信息失败: 返回数据为空", URL: url}
	}

	return &result, nil
}

func (m *productManager) saveDraftProduct(prod *Product) (*SheinResponse, string, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetSaveDraftEndpoint())

	var result SheinResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, prod, &result); err != nil {
		return &result, prod.SupplierCode, err
	}

	if result.Code != "0" {
		return &result, prod.SupplierCode, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("保存草稿产品失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, prod.SupplierCode, nil
}

func (m *productManager) publishProduct(prod *Product) (*SheinResponse, string, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetPublishProductEndpoint())

	var result SheinResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, prod, &result); err != nil {
		return &result, prod.SupplierCode, err
	}

	return &result, prod.SupplierCode, nil
}

func (m *productManager) confirmPublish(product *Product) (bool, string, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetConfirmPublishEndpoint())

	var result struct {
		api.APIResponse
		Info struct {
			Data struct {
				NeedConfirm      bool `json:"need_confirm"`
				SimPriceInfoList any  `json:"sim_price_info_list"`
			} `json:"data"`
		} `json:"info"`
	}

	if err := m.baseClient.APIRequest(http.MethodPost, url, product, &result); err != nil {
		return false, product.SupplierCode, err
	}

	if err := m.errorHandler.ProcessAPIResponse(&result.APIResponse, "0", url, "发布确认失败"); err != nil {
		return false, product.SupplierCode, err
	}

	return result.Info.Data.NeedConfirm, product.SupplierCode, nil
}

func (m *productManager) record(request *ProductRecordRequest) (*RecordResponse, error) {
	url := fmt.Sprintf("%s%s?page_num=%d&page_size=%d", m.baseClient.GetBaseURL(), client.GetProductRecordEndpoint(), 1, 100)
	reqBody := ProductRecordRequest{
		Language:                  "en",
		OnlyCurrentMonthRecommend: false,
		OnlySpmbCopyProduct:       false,
		QueryTimeOut:              false,
		SearchDiyCustom:           false,
		SupplierCodeList:          request.SupplierCodeList,
		SupplierCodeSearchType:    request.SupplierCodeSearchType,
	}
	var result RecordResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}
	if err := m.baseClient.CheckCode(result.Code, result.Msg, url, "获取产品发布记录失败"); err != nil {
		return nil, err
	}
	return &result, nil
}

func (m *productManager) listProducts(pageNum, pageSize int, request *ProductListRequest) (*ProductListResponse, error) {
	url := fmt.Sprintf("%s%s?page_num=%d&page_size=%d", m.baseClient.GetBaseURL(), client.GetProductListEndpoint(), pageNum, pageSize)
	var result ProductListResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, request, &result); err != nil {
		return nil, err
	}
	if err := m.baseClient.CheckCode(result.Code, result.Msg, url, "获取产品列表失败"); err != nil {
		return nil, err
	}
	return &result, nil
}

func (m *productManager) queryBrandList() (*BrandListResponse, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetQueryBrandListEndpoint())

	var result BrandListResponse
	if err := m.baseClient.APIRequest(http.MethodPost, url, nil, &result); err != nil {
		return nil, err
	}
	if err := m.baseClient.CheckCode(result.Code, result.Msg, url, "获取品牌列表失败"); err != nil {
		return nil, err
	}
	return &result, nil
}
