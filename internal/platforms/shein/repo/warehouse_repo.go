package repo

import (
	"fmt"
	"net/http"
	"task-processor/internal/platforms/shein/api"
	"task-processor/internal/platforms/shein/api/warehouse"
	"task-processor/internal/platforms/shein/repo/client"
)

type WarehouseAPI struct {
	*client.BaseAPIClient
}

func NewWarehouseAPI(baseClient *client.BaseAPIClient) *WarehouseAPI {
	return &WarehouseAPI{BaseAPIClient: baseClient}
}

func (a *WarehouseAPI) GetWarehouses() (*warehouse.WarehouseResponse, error) {
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetWarehousesEndpoint())

	var result struct {
		api.APIResponse
		Info warehouse.WarehouseResponse `json:"info"`
	}
	if err := a.APIRequest(http.MethodPost, url, nil, &result); err != nil {
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
			Message:    fmt.Sprintf("获取仓库信息失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info, nil
}
