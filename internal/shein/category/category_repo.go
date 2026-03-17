package category

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api"
	"task-processor/internal/shein/api/category"
	"task-processor/internal/shein/client"
)

// CategoryAPI 分类相关API实现
type CategoryAPI struct {
	*client.BaseAPIClient
}

// NewCategoryAPI 创建新的分类API实现
func NewCategoryAPI(baseClient *client.BaseAPIClient) *CategoryAPI {
	return &CategoryAPI{
		BaseAPIClient: baseClient,
	}
}

func (a *CategoryAPI) GetCategory(categoryID int) (*category.CategoryInfo, error) {
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetCategoryEndpoint())

	reqBody := struct {
		CategoryID int `json:"category_id"`
	}{
		CategoryID: categoryID,
	}

	var result struct {
		api.APIResponse
		Info category.CategoryInfo `json:"info"`
	}

	if err := a.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	// 统一错误处理 - 认证过期错误直接返回，其他错误包装为 APIError
	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回不包装
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		// 其他错误包装为 APIError
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("获取分类信息失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info, nil
}

func (a *CategoryAPI) GetCategoryTree() (*category.CategoryTreeResponse, error) {
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetCategoryTreeEndpoint())

	var result struct {
		api.APIResponse
		Info category.CategoryTreeResponse `json:"info"`
	}

	if err := a.APIRequest(http.MethodPost, url, map[string]any{}, &result); err != nil {
		return nil, err
	}

	// 统一错误处理 - 认证过期错误直接返回，其他错误包装为 APIError
	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回不包装
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		// 其他错误包装为 APIError
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("获取分类树失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info, nil
}
