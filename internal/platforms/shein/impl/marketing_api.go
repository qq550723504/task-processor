// Package impl 提供SHEIN营销活动API的具体实现
package impl

import (
	"fmt"
	"net/http"
	"task-processor/internal/platforms/shein/api"
	"task-processor/internal/platforms/shein/api/marketing"
)

// MarketingAPI 营销活动相关API实现
type MarketingAPI struct {
	*BaseAPIClient
}

// NewMarketingAPI 创建新的营销API实现
func NewMarketingAPI(baseClient *BaseAPIClient) *MarketingAPI {
	return &MarketingAPI{
		BaseAPIClient: baseClient,
	}
}

// GetAvailableSkcList 获取可报名活动的产品列表
func (m *MarketingAPI) GetAvailableSkcList(req *marketing.GetAvailableSkcListRequest) (*marketing.GetAvailableSkcListResponse, error) {
	url := fmt.Sprintf("%s%s", m.GetBaseURL(), getAvailableSkcListEndpoint)

	reqBody := map[string]any{
		"page_num":  req.PageNum,
		"page_size": req.PageSize,
	}

	var result struct {
		api.APIResponse
		Info *marketing.AvailableSkcListInfo `json:"info"`
		BBL  any                             `json:"bbl"`
	}

	if err := m.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("获取可报名活动产品列表请求失败: %w", err)
	}

	// 统一错误处理
	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("获取可报名活动产品列表失败: %s", result.Msg),
			URL:        url,
		}
	}

	// 构造返回结果
	response := &marketing.GetAvailableSkcListResponse{
		Code: result.Code,
		Msg:  result.Msg,
		Info: result.Info,
		BBL:  result.BBL,
	}

	return response, nil
}

// SaveConfig 保存活动配置（报名活动）
func (m *MarketingAPI) SaveConfig(req *marketing.SaveConfigRequest) (*marketing.SaveConfigResponse, error) {
	url := fmt.Sprintf("%s%s", m.GetBaseURL(), saveConfigEndpoint)

	reqBody := map[string]any{
		"config_list": req.ConfigList,
	}

	var result struct {
		api.APIResponse
		Info any `json:"info"`
		BBL  any `json:"bbl"`
	}

	if err := m.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("保存活动配置请求失败: %w", err)
	}

	// 统一错误处理
	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("保存活动配置失败: %s", result.Msg),
			URL:        url,
		}
	}

	// 构造返回结果
	response := &marketing.SaveConfigResponse{
		Code: result.Code,
		Msg:  result.Msg,
		Info: result.Info,
		BBL:  result.BBL,
	}

	return response, nil
}

// GetConfigList 获取已报名活动的产品列表
func (m *MarketingAPI) GetConfigList(req *marketing.GetConfigListRequest) (*marketing.GetConfigListResponse, error) {
	url := fmt.Sprintf("%s%s", m.GetBaseURL(), getConfigListEndpoint)

	reqBody := map[string]any{
		"page_num":  req.PageNum,
		"page_size": req.PageSize,
	}

	var result struct {
		api.APIResponse
		Info *marketing.ConfigListInfo `json:"info"`
		BBL  any                       `json:"bbl"`
	}

	if err := m.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("获取已报名活动产品列表请求失败: %w", err)
	}

	// 统一错误处理
	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("获取已报名活动产品列表失败: %s", result.Msg),
			URL:        url,
		}
	}

	// 构造返回结果
	response := &marketing.GetConfigListResponse{
		Code: result.Code,
		Msg:  result.Msg,
		Info: result.Info,
		BBL:  result.BBL,
	}

	return response, nil
}
