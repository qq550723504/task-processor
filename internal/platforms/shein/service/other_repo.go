package service

import (
	"fmt"
	"net/http"
	"task-processor/internal/platforms/shein/api"
	"task-processor/internal/platforms/shein/api/other"
	"task-processor/internal/platforms/shein/client"
)

// OtherAPI 其他相关API实现
type OtherAPI struct {
	*client.BaseAPIClient
}

// NewOtherAPI 创建新的其他API实现
func NewOtherAPI(baseClient *client.BaseAPIClient) *OtherAPI {
	return &OtherAPI{
		BaseAPIClient: baseClient,
	}
}

// GetUser 获取用户信息
func (o *OtherAPI) GetUser(uuid int64) (*other.UserInfo, error) {
	url := fmt.Sprintf("%s%s?uuid=%d", o.GetBaseURL(), client.GetUserEndpoint(), uuid)

	var result struct {
		api.APIResponse
		Info *other.UserInfo `json:"info"`
	}

	if err := o.APIRequest(http.MethodGet, url, nil, &result); err != nil {
		return nil, err
	}

	// 统一错误处理 - 认证过期错误直接返回，其他错误包装为 APIError
	if err := o.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回不包装
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		// 其他错误包装为 APIError
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("获取用户信息失败: %s", result.Msg),
			URL:        url,
		}
	}

	return result.Info, nil
}

// BatchCheckOnWay 批量检查在途商品
func (o *OtherAPI) BatchCheckOnWay(spuNameList []string) (*other.BatchCheckOnWayResponse, error) {
	url := fmt.Sprintf("%s%s", o.GetBaseURL(), client.GetBatchCheckOnWayEndpoint())

	reqBody := map[string]any{
		"spu_name_list": spuNameList,
	}

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

	// 统一错误处理 - 认证过期错误直接返回，其他错误包装为 APIError
	if err := o.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回不包装
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		// 其他错误包装为 APIError
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("批量检查在途商品失败: %s", result.Msg),
			URL:        url,
		}
	}

	// 构造返回结果
	response := &other.BatchCheckOnWayResponse{
		Code: result.Code,
		Msg:  result.Msg,
		Info: result.Info,
		BBL:  result.BBL,
	}

	return response, nil
}

// GetSupplierOperateInfo 获取供应商操作信息
func (o *OtherAPI) GetSupplierOperateInfo() (*other.SupplierOperateInfoResponse, error) {
	url := fmt.Sprintf("%s%s", o.GetBaseURL(), client.GetSupplierOperateInfoEndpoint())

	var result struct {
		api.APIResponse
		Info other.SupplierOperateInfo `json:"info"`
		BBL  *string                   `json:"bbl"`
	}

	if err := o.APIRequest(http.MethodPost, url, nil, &result); err != nil {
		return nil, err
	}

	// 统一错误处理 - 认证过期错误直接返回，其他错误包装为 APIError
	if err := o.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回不包装
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		// 其他错误包装为 APIError
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("获取供应商操作信息失败: %s", result.Msg),
			URL:        url,
		}
	}

	// 构造返回结果
	response := &other.SupplierOperateInfoResponse{
		Code: result.Code,
		Msg:  result.Msg,
		Info: result.Info,
		BBL:  result.BBL,
	}

	return response, nil
}

// GetSpuLimitCount 获取SPU限制数量
func (o *OtherAPI) GetSpuLimitCount() (*other.SpuLimitCountInfo, error) {
	url := fmt.Sprintf("%s%s", o.GetBaseURL(), client.GetSpuLimitCountEndpoint())

	var result struct {
		api.APIResponse
		Info struct {
			Data other.SpuLimitCountInfo `json:"data"`
		} `json:"info"`
		Bbl *string `json:"bbl"`
	}

	if err := o.APIRequest(http.MethodGet, url, nil, &result); err != nil {
		return nil, err
	}

	// 统一错误处理 - 认证过期错误直接返回，其他错误包装为 APIError
	if err := o.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回不包装
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		// 其他错误包装为 APIError
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("获取SPU限制数量失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info.Data, nil
}

// QueryShelfQuota 查询商品上架配额
func (o *OtherAPI) QueryShelfQuota() (*other.ShelfQuotaResponse, error) {
	url := fmt.Sprintf("%s%s", o.GetBaseURL(), client.GetQueryShelfQuotaEndpoint())

	// 请求体为空对象
	reqBody := map[string]any{}

	var result struct {
		api.APIResponse
		Info other.ShelfQuotaInfo `json:"info"`
		Bbl  *string              `json:"bbl"`
	}

	if err := o.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	// 统一错误处理 - 认证过期错误直接返回，其他错误包装为 APIError
	if err := o.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回不包装
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		// 其他错误包装为 APIError
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("查询商品上架配额失败: %s", result.Msg),
			URL:        url,
		}
	}

	// 构造返回结果
	response := &other.ShelfQuotaResponse{
		Code: result.Code,
		Msg:  result.Msg,
		Info: result.Info,
		Bbl:  result.Bbl,
	}

	return response, nil
}
