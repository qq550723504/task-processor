// Package impl 提供SHEIN活动产品API的具体实现
package impl

import (
	"fmt"
	"net/http"
	"task-processor/internal/platforms/shein/client/api"
	"task-processor/internal/platforms/shein/client/api/activity"
)

// ActivityProductAPI SHEIN活动产品相关API实现
type ActivityProductAPI struct {
	*BaseAPIClient
}

// NewActivityProductAPI 创建新的活动产品API实现
func NewActivityProductAPI(baseClient *BaseAPIClient) *ActivityProductAPI {
	return &ActivityProductAPI{
		BaseAPIClient: baseClient,
	}
}

// GetAvailableActivityProducts 获取可报名活动的产品列表
func (a *ActivityProductAPI) GetAvailableActivityProducts(req *activity.GetAvailableActivityProductsRequest) (*activity.GetAvailableActivityProductsResponse, error) {
	// 使用营销API的可报名活动产品接口
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), getAvailableSkcListEndpoint)

	reqBody := map[string]any{
		"page_num":  req.PageNum,
		"page_size": req.PageSize,
	}

	var result struct {
		api.APIResponse
		Info *activity.AvailableActivityProductInfo `json:"info"`
		BBL  any                                    `json:"bbl"`
	}

	if err := a.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("获取可报名活动产品列表请求失败: %w", err)
	}

	// 统一错误处理 - 使用 ProcessAPIResponse 检查认证过期
	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		// 其他错误，包装为 APIError
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("获取可报名活动产品列表失败: %s", result.Msg),
			URL:        url,
		}
	}

	// 构造返回结果
	response := &activity.GetAvailableActivityProductsResponse{
		Code: result.Code,
		Msg:  result.Msg,
		Info: result.Info,
		BBL:  result.BBL,
	}

	return response, nil
}
