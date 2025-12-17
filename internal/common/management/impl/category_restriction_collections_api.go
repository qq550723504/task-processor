package impl

import (
	"fmt"
	"net/http"
	"task-processor/internal/common/management/api"
)

// CategoryRestrictionCollectionsAPIClientImpl 品类限制集合API客户端实现
type CategoryRestrictionCollectionsAPIClientImpl struct {
	*ManagementAPIClientImpl
}

// CreateCategoryRestrictionCollections 添加品类限制集合
func (m *CategoryRestrictionCollectionsAPIClientImpl) CreateCategoryRestrictionCollections(req *api.CategoryRestrictionCollectionsCreateReqDTO) (int64, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/create", m.baseURL)

	// 构建查询参数
	params := map[string]interface{}{
		"categoryId":             req.CategoryId,
		"platformName":           req.PlatformName,
		"forbiddenAttributeId":   req.ForbiddenAttributeId,
		"forbiddenAttributeName": req.ForbiddenAttributeName,
		"defaultAttributeId":     req.DefaultAttributeId,
		"defaultAttributeName":   req.DefaultAttributeName,
		"occurrenceCount":        req.OccurrenceCount,
		"confidenceScore":        req.ConfidenceScore,
		"isConfirmed":            req.IsConfirmed,
		"isAutoApplied":          req.IsAutoApplied,
	}

	var result APIResponse
	err := m.apiRequest(http.MethodPost, url, params, &result)
	if err != nil {
		return 0, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return 0, err
	}

	// 类型断言获取返回的数据
	data, ok := result.Data.(float64)
	if !ok {
		return 0, fmt.Errorf("返回数据格式错误")
	}

	return int64(data), nil
}

// GetListByCategoryAndPlatform 获取指定品类和平台的限制集合
func (m *CategoryRestrictionCollectionsAPIClientImpl) GetListByPlatform(platformName string) ([]api.CategoryRestrictionInfoRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/list-by-platform", m.baseURL)

	// 构建查询参数
	params := map[string]interface{}{
		"platformName": platformName,
	}

	var result APIResponse
	result.Data = &[]api.CategoryRestrictionInfoRespDTO{}

	err := m.apiRequest(http.MethodGet, url, params, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	// 类型断言获取返回的数据
	data, ok := result.Data.(*[]api.CategoryRestrictionInfoRespDTO)
	if !ok {
		return nil, fmt.Errorf("返回数据格式错误")
	}

	return *data, nil
}

// GetConfirmedListByCategoryAndPlatform 获取已确认的限制集合
func (m *CategoryRestrictionCollectionsAPIClientImpl) GetConfirmedListByPlatform(platformName string) ([]api.CategoryRestrictionInfoRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/list-confirmed-by-platform", m.baseURL)

	// 构建查询参数
	params := map[string]interface{}{
		"platformName": platformName,
	}

	var result APIResponse
	result.Data = &[]api.CategoryRestrictionInfoRespDTO{}

	err := m.apiRequest(http.MethodGet, url, params, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	// 类型断言获取返回的数据
	data, ok := result.Data.(*[]api.CategoryRestrictionInfoRespDTO)
	if !ok {
		return nil, fmt.Errorf("返回数据格式错误")
	}

	return *data, nil
}

// IsAttributeRestricted 检查属性是否被限制
func (m *CategoryRestrictionCollectionsAPIClientImpl) IsAttributeRestricted(categoryId int, platformName string, attributeId int) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/check-attribute-restricted", m.baseURL)

	// 构建查询参数
	params := map[string]interface{}{
		"categoryId":   categoryId,
		"platformName": platformName,
		"attributeId":  attributeId,
	}

	var result APIResponse
	err := m.apiRequest(http.MethodGet, url, params, &result)
	if err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 类型断言获取返回的数据
	data, ok := result.Data.(bool)
	if !ok {
		return false, fmt.Errorf("返回数据格式错误")
	}

	return data, nil
}

// UpdateCategoryRestrictionCollectionsStatus 更新品类限制集合状态
func (m *CategoryRestrictionCollectionsAPIClientImpl) UpdateCategoryRestrictionCollectionsStatus(id int64, isConfirmed bool, isAutoApplied bool) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/update-status", m.baseURL)

	// 构建查询参数
	params := map[string]interface{}{
		"id":            id,
		"isConfirmed":   isConfirmed,
		"isAutoApplied": isAutoApplied,
	}

	var result APIResponse
	err := m.apiRequest(http.MethodPost, url, params, &result)
	if err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 类型断言获取返回的数据
	data, ok := result.Data.(bool)
	if !ok {
		return false, fmt.Errorf("返回数据格式错误")
	}

	return data, nil
}
