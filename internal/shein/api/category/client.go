package category

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api"
	"task-processor/internal/shein/client"
)

// Client 分类相关API实现
type Client struct {
	*client.BaseAPIClient
}

// NewClient 创建新的分类API客户端
func NewClient(baseClient *client.BaseAPIClient) *Client {
	return &Client{BaseAPIClient: baseClient}
}

// GetCategory 获取分类信息
func (a *Client) GetCategory(categoryID int) (*CategoryInfo, error) {
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetCategoryEndpoint())

	reqBody := struct {
		CategoryID int `json:"category_id"`
	}{CategoryID: categoryID}

	var result struct {
		api.APIResponse
		Info CategoryInfo `json:"info"`
	}

	if err := a.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("获取分类信息失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info, nil
}

// GetCategoryTree 获取分类树
func (a *Client) GetCategoryTree() (*CategoryTreeResponse, error) {
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetCategoryTreeEndpoint())

	var result struct {
		api.APIResponse
		Info CategoryTreeResponse `json:"info"`
	}

	if err := a.APIRequest(http.MethodPost, url, map[string]any{}, &result); err != nil {
		return nil, err
	}

	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("获取分类树失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info, nil
}
