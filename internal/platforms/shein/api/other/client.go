package other

import (
	"fmt"
	"net/http"
	"task-processor/internal/platforms/shein/api"
	"task-processor/internal/platforms/shein/client"
)

// Client 其他相关API实现
type Client struct {
	*client.BaseAPIClient
}

// NewClient 创建新的 other API 客户端
func NewClient(baseClient *client.BaseAPIClient) *Client {
	return &Client{BaseAPIClient: baseClient}
}

// GetUser 获取用户信息
func (o *Client) GetUser(uuid int64) (*UserInfo, error) {
	url := fmt.Sprintf("%s%s?uuid=%d", o.GetBaseURL(), client.GetUserEndpoint(), uuid)

	var result struct {
		api.APIResponse
		Info *UserInfo `json:"info"`
	}

	if err := o.APIRequest(http.MethodGet, url, nil, &result); err != nil {
		return nil, err
	}

	if err := o.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("获取用户信息失败: %s", result.Msg),
			URL:        url,
		}
	}

	return result.Info, nil
}

// BatchCheckOnWay 批量检查在途商品
func (o *Client) BatchCheckOnWay(spuNameList []string) (*BatchCheckOnWayResponse, error) {
	url := fmt.Sprintf("%s%s", o.GetBaseURL(), client.GetBatchCheckOnWayEndpoint())

	reqBody := map[string]interface{}{"spu_name_list": spuNameList}

	var result struct {
		api.APIResponse
		Info []struct {
			SpuName    string `json:"spu_name"`
			SkcName    string `json:"skc_name"`
			DocumentSn string `json:"document_sn"`
		} `json:"info"`
		BBL any `json:"bbl"`
	}

	if err := o.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	if err := o.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("批量检查在途商品失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &BatchCheckOnWayResponse{
		Code: result.Code,
		Msg:  result.Msg,
		Info: result.Info,
		BBL:  result.BBL,
	}, nil
}

// GetSupplierOperateInfo 获取供应商操作信息
func (o *Client) GetSupplierOperateInfo() (*SupplierOperateInfoResponse, error) {
	url := fmt.Sprintf("%s%s", o.GetBaseURL(), client.GetSupplierOperateInfoEndpoint())

	var result struct {
		api.APIResponse
		Info SupplierOperateInfo `json:"info"`
		BBL  *string             `json:"bbl"`
	}

	if err := o.APIRequest(http.MethodPost, url, nil, &result); err != nil {
		return nil, err
	}

	if err := o.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("获取供应商操作信息失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &SupplierOperateInfoResponse{
		Code: result.Code,
		Msg:  result.Msg,
		Info: result.Info,
		BBL:  result.BBL,
	}, nil
}

// GetSpuLimitCount 获取SPU限制数量
func (o *Client) GetSpuLimitCount() (*SpuLimitCountInfo, error) {
	url := fmt.Sprintf("%s%s", o.GetBaseURL(), client.GetSpuLimitCountEndpoint())

	var result struct {
		api.APIResponse
		Info struct {
			Data SpuLimitCountInfo `json:"data"`
		} `json:"info"`
		Bbl *string `json:"bbl"`
	}

	if err := o.APIRequest(http.MethodGet, url, nil, &result); err != nil {
		return nil, err
	}

	if err := o.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("获取SPU限制数量失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info.Data, nil
}

// QueryShelfQuota 查询商品上架配额
func (o *Client) QueryShelfQuota() (*ShelfQuotaResponse, error) {
	url := fmt.Sprintf("%s%s", o.GetBaseURL(), client.GetQueryShelfQuotaEndpoint())

	var result struct {
		api.APIResponse
		Info ShelfQuotaInfo `json:"info"`
		Bbl  *string        `json:"bbl"`
	}

	if err := o.APIRequest(http.MethodPost, url, map[string]interface{}{}, &result); err != nil {
		return nil, err
	}

	if err := o.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("查询商品上架配额失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &ShelfQuotaResponse{
		Code: result.Code,
		Msg:  result.Msg,
		Info: result.Info,
		Bbl:  result.Bbl,
	}, nil
}
