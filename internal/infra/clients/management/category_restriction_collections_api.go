package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
)

// CategoryRestrictionCollectionsAPIClient 品类限制集合API客户端实现
type CategoryRestrictionCollectionsAPIClient struct {
	*ManagementAPIClient
}

// CreateCategoryRestrictionCollections 添加品类限制集合
func (m *CategoryRestrictionCollectionsAPIClient) CreateCategoryRestrictionCollections(req *api.CategoryRestrictionCollectionsCreateReqDTO) (int64, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/create", m.baseURL)

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
	if err := m.apiRequest(http.MethodPost, url, params, &result); err != nil {
		return 0, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return 0, err
	}

	data, ok := result.Data.(float64)
	if !ok {
		return 0, fmt.Errorf("返回数据格式错误")
	}

	return int64(data), nil
}

// GetListByPlatform 获取指定平台的限制集合
func (m *CategoryRestrictionCollectionsAPIClient) GetListByPlatform(platformName string) ([]api.CategoryRestrictionInfoRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/list-by-platform", m.baseURL)

	params := map[string]interface{}{
		"platformName": platformName,
	}

	var result APIResponse
	result.Data = &[]api.CategoryRestrictionInfoRespDTO{}

	if err := m.apiRequest(http.MethodGet, url, params, &result); err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	data, ok := result.Data.(*[]api.CategoryRestrictionInfoRespDTO)
	if !ok {
		return nil, fmt.Errorf("返回数据格式错误")
	}

	return *data, nil
}

// GetConfirmedListByPlatform 获取已确认的限制集合
func (m *CategoryRestrictionCollectionsAPIClient) GetConfirmedListByPlatform(platformName string) ([]api.CategoryRestrictionInfoRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/list-confirmed-by-platform", m.baseURL)

	params := map[string]interface{}{
		"platformName": platformName,
	}

	var result APIResponse
	result.Data = &[]api.CategoryRestrictionInfoRespDTO{}

	if err := m.apiRequest(http.MethodGet, url, params, &result); err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	data, ok := result.Data.(*[]api.CategoryRestrictionInfoRespDTO)
	if !ok {
		return nil, fmt.Errorf("返回数据格式错误")
	}

	return *data, nil
}

// IsAttributeRestricted 检查属性是否被限制
func (m *CategoryRestrictionCollectionsAPIClient) IsAttributeRestricted(categoryId int, platformName string, attributeId int) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/check-attribute-restricted", m.baseURL)

	params := map[string]interface{}{
		"categoryId":   categoryId,
		"platformName": platformName,
		"attributeId":  attributeId,
	}

	var result APIResponse
	if err := m.apiRequest(http.MethodGet, url, params, &result); err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	data, ok := result.Data.(bool)
	if !ok {
		return false, fmt.Errorf("返回数据格式错误")
	}

	return data, nil
}

// UpdateCategoryRestrictionCollectionsStatus 更新品类限制集合状态
func (m *CategoryRestrictionCollectionsAPIClient) UpdateCategoryRestrictionCollectionsStatus(id int64, isConfirmed bool, isAutoApplied bool) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/update-status", m.baseURL)

	params := map[string]interface{}{
		"id":            id,
		"isConfirmed":   isConfirmed,
		"isAutoApplied": isAutoApplied,
	}

	var result APIResponse
	if err := m.apiRequest(http.MethodPost, url, params, &result); err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	data, ok := result.Data.(bool)
	if !ok {
		return false, fmt.Errorf("返回数据格式错误")
	}

	return data, nil
}
