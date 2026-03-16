// Package product 提供TEMU平台产品相关的API和数据结构
package product

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"

	"github.com/sirupsen/logrus"
)

// 搜索类型常量
const (
	SkuSearchTypeOnSale     = 2    // 在售
	SkuSearchTypeOffSale    = 3    // 不在售
	GoodsSearchTypeAll      = 1    // 全部
	StatusFilterTypeDefault = 2001 // 默认状态过滤
	OrderTypeDesc           = 0    // 降序
	OrderTypeAsc            = 1    // 升序
)

// API 产品API管理器（搜索、保存、提交、创建提交）
type API struct {
	client client.ClientAPI
	logger *logrus.Entry
}

// NewAPI 创建产品API管理器
func NewAPI(c client.ClientAPI, logger *logrus.Entry) *API {
	return &API{client: c, logger: logger}
}

// SearchGoods 搜索商品列表
func (a *API) SearchGoods(pageNo, pageSize int) (*GoodsSearchResponse, error) {
	body := GoodsSearchRequest{
		PageSize:                 pageSize,
		PageNo:                   pageNo,
		OrderType:                OrderTypeDesc,
		OrderField:               "gmt_create",
		EnableBatchSearchText:    true,
		StatusFilterType:         StatusFilterTypeDefault,
		GoodsSearchType:          GoodsSearchTypeAll,
		GoodsSubStatusFilterType: StatusFilterTypeDefault,
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/goods/v2/search",
		"headers": map[string]string{
			"content-type":       "application/json;charset=UTF-8",
			"x-document-referer": "https://seller.temu.com/products.html",
		},
		"body": body,
	}

	var result GoodsSearchResponse
	authManager := client.NewAuthManager(a.logger)
	if err := authManager.SendRequestWithAuth(a.client, req, &result); err != nil {
		return nil, fmt.Errorf("调用商品搜索API失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("API返回错误: error_code=%d", result.ErrorCode)
	}
	return &result, nil
}

// Save 保存产品到草稿箱
func (a *API) Save(request *SaveRequest) (*SaveResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("保存请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/edit/commit/save",
		"headers": headers,
		"body":    request,
	}

	var result SaveResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("发送产品保存请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("产品保存失败: errorCode=%d, message=%s", result.ErrorCode, result.Message)
	}
	return &result, nil
}

// Submit 提交产品
func (a *API) Submit(request *SubmitRequest) (*SubmitResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("提交请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/edit/commit/submit",
		"headers": headers,
		"body":    request,
	}

	var result SubmitResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("产品提交请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("产品提交失败: errorCode=%d, message=%s", result.ErrorCode, result.Message)
	}
	return &result, nil
}

// CreateCommit 创建新的提交
func (a *API) CreateCommit(request *CreateCommitRequest) (*CreateCommitResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("创建提交请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/edit/commit/create_new",
		"headers": headers,
		"body":    request,
	}

	var result CreateCommitResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("创建提交请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("创建提交失败: errorCode=%d, message=%s", result.ErrorCode, result.Message)
	}
	return &result, nil
}

// QueryMaxRetailPrice 查询最大零售价格
func (a *API) QueryMaxRetailPrice(request *PriceQueryRequest) (*PriceQueryResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/price/retail/max/info",
		"body":   request,
	}

	var result PriceQueryResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("价格查询请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("价格查询失败: errorCode=%d, message=%s", result.ErrorCode, result.ErrorMsg)
	}
	return &result, nil
}

// GoodsSearchOptions 商品搜索查询选项
type GoodsSearchOptions struct {
	PageNo                   int
	PageSize                 int
	OrderType                int
	OrderField               string
	StatusFilterType         int
	GoodsSearchType          int
	GoodsSubStatusFilterType int
}

// NewGoodsSearchOptions 创建商品搜索查询选项（带默认值）
func NewGoodsSearchOptions(pageNo, pageSize int) GoodsSearchOptions {
	return GoodsSearchOptions{
		PageNo:                   pageNo,
		PageSize:                 pageSize,
		OrderType:                OrderTypeDesc,
		OrderField:               "gmt_create",
		StatusFilterType:         StatusFilterTypeDefault,
		GoodsSearchType:          GoodsSearchTypeAll,
		GoodsSubStatusFilterType: StatusFilterTypeDefault,
	}
}

// SearchGoodsWithOptions 使用选项搜索商品列表
func (a *API) SearchGoodsWithOptions(options GoodsSearchOptions) (*GoodsSearchResponse, error) {
	return a.SearchGoods(options.PageNo, options.PageSize)
}
