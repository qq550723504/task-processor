package marketing

import (
	"fmt"
	"net/http"
	"task-processor/internal/platforms/shein/api"
	"task-processor/internal/platforms/shein/client"
)

// Client 营销活动相关API实现
type Client struct {
	*client.BaseAPIClient
}

// NewClient 创建新的营销API客户端
func NewClient(baseClient *client.BaseAPIClient) *Client {
	return &Client{BaseAPIClient: baseClient}
}

func (m *Client) paginatedQuery(endpoint string, pageNum, pageSize int, result any, errorMsg string) error {
	url := fmt.Sprintf("%s%s", m.GetBaseURL(), endpoint)
	reqBody := map[string]any{"page_num": pageNum, "page_size": pageSize}
	if err := m.APIRequest(http.MethodPost, url, reqBody, result); err != nil {
		return fmt.Errorf("%s请求失败: %w", errorMsg, err)
	}
	return nil
}

// GetAvailableSkcList 获取可报名活动的产品列表
func (m *Client) GetAvailableSkcList(req *GetAvailableSkcListRequest) (*GetAvailableSkcListResponse, error) {
	var result struct {
		api.APIResponse
		Info *AvailableSkcListInfo `json:"info"`
		BBL  any                   `json:"bbl"`
	}

	endpoint := client.GetAvailableSkcListEndpoint()
	if err := m.paginatedQuery(endpoint, req.PageNum, req.PageSize, &result, "获取可报名活动产品列表"); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s%s", m.GetBaseURL(), endpoint)
	if result.Code != "0" {
		return nil, &api.APIError{StatusCode: 0, Message: fmt.Sprintf("获取可报名活动产品列表失败: %s", result.Msg), URL: url}
	}

	return &GetAvailableSkcListResponse{Code: result.Code, Msg: result.Msg, Info: result.Info, BBL: result.BBL}, nil
}

// SaveConfig 保存活动配置
func (m *Client) SaveConfig(req *SaveConfigRequest) (*SaveConfigResponse, error) {
	url := fmt.Sprintf("%s%s", m.GetBaseURL(), client.GetSaveConfigEndpoint())

	reqBody := map[string]any{"config_list": req.ConfigList}

	var result struct {
		api.APIResponse
		Info any `json:"info"`
		BBL  any `json:"bbl"`
	}

	if err := m.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("保存活动配置请求失败: %w", err)
	}

	if result.Code != "0" {
		return nil, &api.APIError{StatusCode: 0, Message: fmt.Sprintf("保存活动配置失败: %s", result.Msg), URL: url}
	}

	return &SaveConfigResponse{Code: result.Code, Msg: result.Msg, Info: result.Info, BBL: result.BBL}, nil
}

// GetConfigList 获取已报名活动的产品列表
func (m *Client) GetConfigList(req *GetConfigListRequest) (*GetConfigListResponse, error) {
	var result struct {
		api.APIResponse
		Info *ConfigListInfo `json:"info"`
		BBL  any             `json:"bbl"`
	}

	endpoint := client.GetConfigListEndpoint()
	if err := m.paginatedQuery(endpoint, req.PageNum, req.PageSize, &result, "获取已报名活动产品列表"); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s%s", m.GetBaseURL(), endpoint)
	if result.Code != "0" {
		return nil, &api.APIError{StatusCode: 0, Message: fmt.Sprintf("获取已报名活动产品列表失败: %s", result.Msg), URL: url}
	}

	return &GetConfigListResponse{Code: result.Code, Msg: result.Msg, Info: result.Info, BBL: result.BBL}, nil
}

// QueryPromotionGoods 查询促销活动商品列表
func (m *Client) QueryPromotionGoods(req *QueryPromotionGoodsRequest) (*QueryPromotionGoodsResponse, error) {
	url := fmt.Sprintf("%s%s", m.GetBaseURL(), client.GetQueryPromotionGoodsEndpoint())

	reqBody := map[string]any{
		"activity_base_info_request": map[string]any{
			"act_name":        req.ActivityBaseInfoRequest.ActName,
			"ref_tool_id":     req.ActivityBaseInfoRequest.RefToolID,
			"time_zone":       req.ActivityBaseInfoRequest.TimeZone,
			"zone_end_time":   req.ActivityBaseInfoRequest.ZoneEndTime,
			"zone_start_time": req.ActivityBaseInfoRequest.ZoneStartTime,
			"sub_type_id":     req.ActivityBaseInfoRequest.SubTypeID,
		},
		"effective_center_list": req.EffectiveCenterList,
		"is_shelf":              req.IsShelf,
		"page_num":              req.PageNum,
		"page_size":             req.PageSize,
	}

	var result struct {
		api.APIResponse
		Info *PromotionGoodsInfo `json:"info"`
		BBL  any                 `json:"bbl"`
	}

	if err := m.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("查询促销活动商品列表请求失败: %w", err)
	}

	if result.Code != "0" {
		return nil, &api.APIError{StatusCode: 0, Message: fmt.Sprintf("查询促销活动商品列表失败: %s", result.Msg), URL: url}
	}

	return &QueryPromotionGoodsResponse{Code: result.Code, Msg: result.Msg, Info: result.Info, BBL: result.BBL}, nil
}

// CalculateSupplyPrice 计算供货价格和利润
func (m *Client) CalculateSupplyPrice(req *CalculateSupplyPriceRequest) (*CalculateSupplyPriceResponse, error) {
	url := fmt.Sprintf("%s%s", m.GetBaseURL(), client.GetCalculateSupplyPriceEndpoint())

	skcInfoList := make([]map[string]any, 0, len(req.SkcInfoList))
	for _, skc := range req.SkcInfoList {
		skuInfoList := make([]map[string]any, 0, len(skc.SkuInfoList))
		for _, sku := range skc.SkuInfoList {
			skuInfoList = append(skuInfoList, map[string]any{
				"discount_value": sku.DiscountValue,
				"product_price":  sku.ProductPrice,
				"sku_code":       sku.SkuCode,
			})
		}
		skcInfoList = append(skcInfoList, map[string]any{
			"skc_name":      skc.SkcName,
			"sku_info_list": skuInfoList,
		})
	}

	reqBody := map[string]any{
		"currency":        req.Currency,
		"ref_tool_id":     req.RefToolID,
		"scene_id":        req.SceneID,
		"skc_info_list":   skcInfoList,
		"time_zone":       req.TimeZone,
		"zone_end_time":   req.ZoneEndTime,
		"zone_start_time": req.ZoneStartTime,
	}

	var result struct {
		api.APIResponse
		Info []SkcCalculationResult `json:"info"`
		BBL  any                    `json:"bbl"`
	}

	if err := m.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("计算供货价格请求失败: %w", err)
	}

	if result.Code != "0" {
		return nil, &api.APIError{StatusCode: 0, Message: fmt.Sprintf("计算供货价格失败: %s", result.Msg), URL: url}
	}

	return &CalculateSupplyPriceResponse{Code: result.Code, Msg: result.Msg, Info: result.Info, BBL: result.BBL}, nil
}

// CreateActivity 创建促销活动
func (m *Client) CreateActivity(req *CreateActivityRequest) (*CreateActivityResponse, error) {
	url := fmt.Sprintf("%s%s", m.GetBaseURL(), client.GetCreateActivityEndpoint())

	addCostAndStockInfoList := make([]map[string]any, 0, len(req.AddCostAndStockInfoList))
	for _, info := range req.AddCostAndStockInfoList {
		addSkuList := make([]map[string]any, 0, len(info.AddSkuList))
		for _, sku := range info.AddSkuList {
			addSkuList = append(addSkuList, map[string]any{
				"cost_price":            sku.CostPrice,
				"sku":                   sku.Sku,
				"max_product_act_price": sku.MaxProductActPrice,
				"product_act_price":     sku.ProductActPrice,
			})
		}
		addCostAndStockInfoList = append(addCostAndStockInfoList, map[string]any{
			"attend_num":            info.AttendNum,
			"center_list":           info.CenterList,
			"is_sale_attribute":     info.IsSaleAttribute,
			"promotion_id_list":     info.PromotionIDList,
			"skc":                   info.Skc,
			"stock_num":             info.StockNum,
			"cost_price":            info.CostPrice,
			"max_product_act_price": info.MaxProductActPrice,
			"product_act_price":     info.ProductActPrice,
			"add_sku_list":          addSkuList,
		})
	}

	reqBody := map[string]any{
		"activity_base_info_request": map[string]any{
			"act_name":  req.ActivityBaseInfoRequest.ActName,
			"time_zone": req.ActivityBaseInfoRequest.TimeZone,
			"activity_rule": map[string]any{
				"goods_limit":     req.ActivityBaseInfoRequest.ActivityRule.GoodsLimit,
				"goods_limit_num": req.ActivityBaseInfoRequest.ActivityRule.GoodsLimitNum,
			},
			"zone_start_time": req.ActivityBaseInfoRequest.ZoneStartTime,
			"zone_end_time":   req.ActivityBaseInfoRequest.ZoneEndTime,
			"ref_tool_id":     req.ActivityBaseInfoRequest.RefToolID,
			"notify_flag":     req.ActivityBaseInfoRequest.NotifyFlag,
			"sub_type_id":     req.ActivityBaseInfoRequest.SubTypeID,
		},
		"add_cost_and_stock_info_list": addCostAndStockInfoList,
		"pricing_type":                 req.PricingType,
	}

	var result struct {
		api.APIResponse
		Info *ActivityCreateInfo `json:"info"`
		BBL  any                 `json:"bbl"`
	}

	if err := m.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("创建促销活动请求失败: %w", err)
	}

	if result.Code != "0" {
		return nil, &api.APIError{StatusCode: 0, Message: fmt.Sprintf("创建促销活动失败: %s", result.Msg), URL: url}
	}

	return &CreateActivityResponse{Code: result.Code, Msg: result.Msg, Info: result.Info, BBL: result.BBL}, nil
}
