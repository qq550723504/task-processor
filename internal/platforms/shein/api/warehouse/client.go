package warehouse

import (
	"fmt"
	"net/http"
	"task-processor/internal/platforms/shein/api"
	"task-processor/internal/platforms/shein/client"
)

// Client 仓库相关API实现
type Client struct {
	*client.BaseAPIClient
}

// NewClient 创建新的仓库API客户端
func NewClient(baseClient *client.BaseAPIClient) *Client {
	return &Client{BaseAPIClient: baseClient}
}

// GetWarehouses 获取仓库列表
func (a *Client) GetWarehouses() (*WarehouseResponse, error) {
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetWarehousesEndpoint())

	var result struct {
		api.APIResponse
		Info WarehouseResponse `json:"info"`
	}
	if err := a.APIRequest(http.MethodPost, url, nil, &result); err != nil {
		return nil, err
	}

	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("获取仓库信息失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info, nil
}
