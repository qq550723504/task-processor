package impl

import (
	"fmt"
	"net/http"
	"task-processor/common/management/api"
)

type ProductImportMappingAPIClientImpl struct {
	*ManagementAPIClientImpl
}

// CreateProductImportMapping 创建产品导入映射关系
func (m *ProductImportMappingAPIClientImpl) CreateProductImportMapping(createReqDTO *api.ProductImportMappingCreateReqDTO) (int64, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/create", m.baseURL)

	var result APIResponse
	result.Data = new(int64)

	err := m.apiRequest(http.MethodPost, url, createReqDTO, &result)
	if err != nil {
		return 0, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return 0, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return 0, fmt.Errorf("创建产品导入映射关系响应数据为空")
	}

	// 安全的类型断言
	id, ok := result.Data.(*int64)
	if !ok {
		return 0, fmt.Errorf("创建产品导入映射关系响应数据类型转换失败")
	}

	return *id, nil
}

// GetProductImportMappingByPlatformProductId 通过店铺ID、平台、区域和平台产品ID获取产品导入映射关系
func (m *ProductImportMappingAPIClientImpl) GetProductImportMappingByPlatformProductId(req *api.ProductImportMappingGetReqDTO) (*api.ProductImportMappingRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/get", m.baseURL)

	// 构建查询参数
	params := map[string]any{
		"platformProductId": req.PlatformProductId,
	}

	var result APIResponse
	result.Data = &api.ProductImportMappingRespDTO{}

	err := m.apiRequest(http.MethodGet, url, params, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return nil, NewNonRetryableError("产品导入映射关系数据为空: 可能不存在对应的映射关系", nil)
	}

	// 安全的类型断言
	mapping, ok := result.Data.(*api.ProductImportMappingRespDTO)
	if !ok {
		return nil, fmt.Errorf("产品导入映射关系数据类型转换失败")
	}

	return mapping, nil
}

// GetProductImportMappingByTaskAndSku 根据任务ID和SKU查询映射关系
func (m *ProductImportMappingAPIClientImpl) GetProductImportMappingByTaskAndSku(importTaskId int64, sku string) (*api.ProductImportMappingRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/get-by-task-and-sku?importTaskId=%d&sku=%s",
		m.baseURL, importTaskId, sku)

	var result APIResponse
	result.Data = &api.ProductImportMappingRespDTO{}

	err := m.apiRequest(http.MethodGet, url, nil, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, nil // 不存在记录时返回nil而不是错误
	}

	mapping, ok := result.Data.(*api.ProductImportMappingRespDTO)
	if !ok {
		return nil, fmt.Errorf("产品导入映射关系数据类型转换失败")
	}

	return mapping, nil
}

// UpdateProductImportMapping 更新产品导入映射关系
func (m *ProductImportMappingAPIClientImpl) UpdateProductImportMapping(updateReqDTO *api.ProductImportMappingCreateReqDTO) error {
	url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/update", m.baseURL)

	var result APIResponse
	result.Data = new(bool)

	err := m.apiRequest(http.MethodPost, url, updateReqDTO, &result)
	if err != nil {
		return err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return err
	}

	return nil
}

// CheckProductExists 检查产品是否已上架
func (m *ProductImportMappingAPIClientImpl) CheckProductExists(req *api.ProductImportMappingCheckReqDTO) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/check-exists?storeId=%d&platform=%s&region=%s&productId=%s",
		m.baseURL, req.StoreId, req.Platform, req.Region, req.ProductId)

	var result APIResponse
	result.Data = new(bool)

	err := m.apiRequest(http.MethodGet, url, nil, &result)
	if err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return false, fmt.Errorf("检查产品是否存在响应数据为空")
	}

	// 安全的类型断言
	exists, ok := result.Data.(*bool)
	if !ok {
		return false, fmt.Errorf("检查产品是否存在响应数据类型转换失败")
	}

	return *exists, nil
}

// GetProductImportMappingBySku 通过SKU获取产品导入映射关系
func (m *ProductImportMappingAPIClientImpl) GetProductImportMappingBySku(req *api.ProductImportMappingGetBySkuReqDTO) (*api.ProductImportMappingRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/get-by-sku", m.baseURL)

	params := map[string]any{
		"sku":     req.Sku,
		"storeId": req.StoreId,
	}

	var result APIResponse
	result.Data = &api.ProductImportMappingRespDTO{}

	err := m.apiRequest(http.MethodGet, url, params, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, NewNonRetryableError("产品导入映射关系数据为空: 可能不存在对应的SKU映射关系", nil)
	}

	mapping, ok := result.Data.(*api.ProductImportMappingRespDTO)
	if !ok {
		return nil, fmt.Errorf("产品导入映射关系数据类型转换失败")
	}

	return mapping, nil
}

// GetProductImportMappingByPlatformProductIdAndStore 通过平台产品ID和店铺ID获取产品导入映射关系
func (m *ProductImportMappingAPIClientImpl) GetProductImportMappingByPlatformProductIdAndStore(req *api.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO) (*api.ProductImportMappingRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/get-by-platform-product-id-and-store", m.baseURL)

	params := map[string]any{
		"platformProductId": req.PlatformProductId,
		"storeId":           req.StoreId,
	}

	var result APIResponse
	result.Data = &api.ProductImportMappingRespDTO{}

	err := m.apiRequest(http.MethodGet, url, params, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, nil // 不存在记录时返回nil而不是错误
	}

	mapping, ok := result.Data.(*api.ProductImportMappingRespDTO)
	if !ok {
		return nil, fmt.Errorf("产品导入映射关系数据类型转换失败")
	}

	return mapping, nil
}
