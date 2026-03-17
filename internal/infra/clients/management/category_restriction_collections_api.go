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

	params := map[string]any{
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
	return m.fetchListByPlatform(platformName, "list-by-platform")
}

// GetConfirmedListByPlatform 获取已确认的限制集合
func (m *CategoryRestrictionCollectionsAPIClient) GetConfirmedListByPlatform(platformName string) ([]api.CategoryRestrictionInfoRespDTO, error) {
	return m.fetchListByPlatform(platformName, "list-confirmed-by-platform")
}

// fetchListByPlatform 按平台获取限制集合列表的通用实现
func (m *CategoryRestrictionCollectionsAPIClient) fetchListByPlatform(platformName, endpoint string) ([]api.CategoryRestrictionInfoRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/%s", m.baseURL, endpoint)
	return getSliceResult[api.CategoryRestrictionInfoRespDTO](m.ManagementAPIClient, url, map[string]any{
		"platformName": platformName,
	})
}

// IsAttributeRestricted 检查属性是否被限制
func (m *CategoryRestrictionCollectionsAPIClient) IsAttributeRestricted(categoryId int, platformName string, attributeId int) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/check-attribute-restricted", m.baseURL)
	return getTypedResult[bool](m.ManagementAPIClient, http.MethodGet, url, map[string]any{
		"categoryId":   categoryId,
		"platformName": platformName,
		"attributeId":  attributeId,
	})
}

// UpdateCategoryRestrictionCollectionsStatus 更新品类限制集合状态
func (m *CategoryRestrictionCollectionsAPIClient) UpdateCategoryRestrictionCollectionsStatus(id int64, isConfirmed bool, isAutoApplied bool) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/category-restriction-collections/update-status", m.baseURL)
	return getTypedResult[bool](m.ManagementAPIClient, http.MethodPost, url, map[string]any{
		"id":            id,
		"isConfirmed":   isConfirmed,
		"isAutoApplied": isAutoApplied,
	})
}
