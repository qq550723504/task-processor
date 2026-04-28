package warehouse

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api"
	"task-processor/internal/shein/client"
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
		return a.getWarehousesFromStoreAddress(err)
	}

	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return a.getWarehousesFromStoreAddress(&api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("获取仓库信息失败: %s", result.Msg),
			URL:        url,
		})
	}

	if len(result.Info.Data) == 0 {
		return a.getWarehousesFromStoreAddress(&api.APIError{
			StatusCode: 0,
			Message:    "获取仓库信息失败: empty warehouse data",
			URL:        url,
		})
	}

	return &result.Info, nil
}

func (a *Client) AddStoreAddress(request *StoreAddressAddRequest) error {
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetStoreAddressAddEndpoint())

	var result api.APIResponse
	if err := a.APIRequest(http.MethodPost, url, request, &result); err != nil {
		return err
	}

	if err := a.ProcessAPIResponse(&result, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return err
		}
		return &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("新增店铺地址仓库失败: %s", result.Msg),
			URL:        url,
		}
	}

	return nil
}

func (a *Client) ListStoreAddresses(addressType int) (*StoreAddressListInfo, error) {
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetStoreAddressListEndpoint())

	var result struct {
		api.APIResponse
		Info StoreAddressListInfo `json:"info"`
	}

	reqBody := StoreAddressListRequest{AddressType: addressType}
	if err := a.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("获取店铺地址仓库信息失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info, nil
}

func (a *Client) CheckStoreAddress(request *StoreAddressCheckRequest) (*StoreAddressCheckInfo, error) {
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetStoreAddressCheckEndpoint())

	var result struct {
		api.APIResponse
		Info StoreAddressCheckInfo `json:"info"`
	}

	if err := a.APIRequest(http.MethodPost, url, request, &result); err != nil {
		return nil, err
	}

	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("校验店铺地址仓库失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info, nil
}

func (a *Client) getWarehousesFromStoreAddress(cause error) (*WarehouseResponse, error) {
	storeAddressInfo, err := a.ListStoreAddresses(2)
	if err != nil {
		if cause != nil {
			return nil, cause
		}
		return nil, err
	}
	warehouseInfo := convertStoreAddresses(*storeAddressInfo)
	if len(warehouseInfo.Data) == 0 {
		if cause != nil {
			return nil, cause
		}
		url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetStoreAddressListEndpoint())
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    "获取店铺地址仓库信息失败: empty warehouse data",
			URL:        url,
		}
	}

	return warehouseInfo, nil
}

func convertStoreAddresses(info StoreAddressListInfo) *WarehouseResponse {
	resp := &WarehouseResponse{
		Data: make([]Warehouse, 0, len(info.Addresses)),
	}

	for _, address := range info.Addresses {
		if address.WarehouseCode == "" {
			continue
		}

		resp.Data = append(resp.Data, Warehouse{
			WarehouseName:   address.WarehouseName,
			WarehouseCode:   address.WarehouseCode,
			SaleCountryList: collectSaleCountries(address.StoreSiteInfos),
			WarehouseType:   address.WarehouseType,
		})
	}

	resp.Meta.Count = len(resp.Data)
	return resp
}

func collectSaleCountries(siteInfos []StoreSiteInfo) []string {
	if len(siteInfos) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(siteInfos))
	countries := make([]string, 0, len(siteInfos))
	for _, siteInfo := range siteInfos {
		for _, country := range siteInfo.SaleCountries {
			if country == "" {
				continue
			}
			if _, ok := seen[country]; ok {
				continue
			}
			seen[country] = struct{}{}
			countries = append(countries, country)
		}
	}

	return countries
}
