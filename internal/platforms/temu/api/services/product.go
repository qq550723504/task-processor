// Package services 提供TEMU平台产品API功能
package services

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

// 搜索类型常量
const (
	// SKU搜索类型
	SkuSearchTypeOnSale  = 2 // 在售
	SkuSearchTypeOffSale = 3 // 不在售

	// 商品搜索类型
	GoodsSearchTypeAll = 1 // 全部

	// 状态过滤类型
	StatusFilterTypeDefault = 2001 // 默认状态过滤

	// 排序类型
	OrderTypeDesc = 0 // 降序
	OrderTypeAsc  = 1 // 升序
)

// ProductAPI 产品API管理器
type ProductAPI struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewProductAPI 创建新的产品API管理器
func NewProductAPI(client client.APIClientInterface, logger *logrus.Entry) *ProductAPI {
	return &ProductAPI{
		client: client,
		logger: logger,
	}
}

// ProductListOptions 产品列表查询选项
type ProductListOptions struct {
	PageNo        int  // 页码
	PageSize      int  // 每页数量
	SkuSearchType *int // SKU搜索类型：2=在售，3=不在售（可选）
}

// NewProductListOptions 创建产品列表查询选项
func NewProductListOptions(pageNo, pageSize int) ProductListOptions {
	return ProductListOptions{
		PageNo:   pageNo,
		PageSize: pageSize,
	}
}

// WithSkuSearchType 设置SKU搜索类型
func (opts ProductListOptions) WithSkuSearchType(searchType int) ProductListOptions {
	opts.SkuSearchType = &searchType
	return opts
}

// GoodsSearchOptions 商品搜索查询选项
type GoodsSearchOptions struct {
	PageNo                   int    // 页码
	PageSize                 int    // 每页数量
	OrderType                int    // 排序类型：0=降序，1=升序
	OrderField               string // 排序字段，默认为 "gmt_create"
	StatusFilterType         int    // 状态过滤类型，默认为 2001
	GoodsSearchType          int    // 商品搜索类型，默认为 1（全部）
	GoodsSubStatusFilterType int    // 商品子状态过滤类型，默认为 2001
}

// NewGoodsSearchOptions 创建商品搜索查询选项
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

// WithOrderType 设置排序类型
func (opts GoodsSearchOptions) WithOrderType(orderType int) GoodsSearchOptions {
	opts.OrderType = orderType
	return opts
}

// WithOrderField 设置排序字段
func (opts GoodsSearchOptions) WithOrderField(orderField string) GoodsSearchOptions {
	opts.OrderField = orderField
	return opts
}

// WithStatusFilterType 设置状态过滤类型
func (opts GoodsSearchOptions) WithStatusFilterType(statusFilterType int) GoodsSearchOptions {
	opts.StatusFilterType = statusFilterType
	return opts
}

// SearchGoods 搜索商品列表
func (p *ProductAPI) SearchGoods(options GoodsSearchOptions) (*models.GoodsSearchResponse, error) {
	p.logger.Info("开始搜索 TEMU 商品列表")

	// 构建请求体
	requestBody := models.GoodsSearchRequest{
		PageSize:                 options.PageSize,
		PageNo:                   options.PageNo,
		OrderType:                options.OrderType,
		OrderField:               options.OrderField,
		EnableBatchSearchText:    true,
		StatusFilterType:         options.StatusFilterType,
		GoodsSearchType:          options.GoodsSearchType,
		GoodsSubStatusFilterType: options.GoodsSubStatusFilterType,
	}

	request := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/goods/v2/search",
		"headers": map[string]string{
			"content-type":       "application/json;charset=UTF-8",
			"x-document-referer": "https://seller.temu.com/products.html",
		},
		"body": requestBody,
	}

	var result models.GoodsSearchResponse
	authManager := client.NewAuthManager(p.logger)
	if err := authManager.SendRequestWithAuth(p.client, request, &result); err != nil {
		return nil, fmt.Errorf("调用商品搜索 API 失败: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API 返回错误: error_code=%d", result.ErrorCode)
	}

	p.logger.WithFields(logrus.Fields{
		"page_no":                      options.PageNo,
		"page_size":                    options.PageSize,
		"order_type":                   options.OrderType,
		"order_field":                  options.OrderField,
		"status_filter_type":           options.StatusFilterType,
		"goods_search_type":            options.GoodsSearchType,
		"goods_sub_status_filter_type": options.GoodsSubStatusFilterType,
		"total":                        result.Result.Total,
		"count":                        len(result.Result.GoodsList),
	}).Info("成功获取 TEMU 商品搜索结果")

	return &result, nil
}

// SaveProduct 保存产品到草稿箱
func (p *ProductAPI) SaveProduct(request *models.ProductSaveRequest) (*models.ProductSaveResponse, error) {
	p.logger.Info("开始保存产品到TEMU草稿箱")

	if request == nil {
		return nil, fmt.Errorf("保存请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["accept"] = "application/json, text/plain, */*"
	headers["accept-language"] = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["priority"] = "u=1, i"
	headers["sec-ch-ua"] = "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\""
	headers["sec-ch-ua-mobile"] = "?0"
	headers["sec-ch-ua-platform"] = "\"Windows\""
	headers["sec-fetch-dest"] = "empty"
	headers["sec-fetch-mode"] = "cors"
	headers["sec-fetch-site"] = "same-origin"

	apiReq := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/edit/commit/save",
		"headers": headers,
		"body":    request,
	}

	var result models.ProductSaveResponse
	if err := p.client.SendTEMURequest(apiReq, &result); err != nil {
		p.logger.WithError(err).Error("发送产品保存请求失败")
		return nil, fmt.Errorf("发送产品保存请求失败: %w", err)
	}

	if !result.Success {
		p.logger.Errorf("产品保存失败: errorCode=%d, message=%s", result.ErrorCode, result.Message)
		return nil, fmt.Errorf("产品保存失败: errorCode=%d, message=%s", result.ErrorCode, result.Message)
	}

	p.logger.Infof("产品保存成功: ListingCommitID=%s",
		func() string {
			if result.Result != nil {
				return result.Result.ListingCommitID
			}
			return "未返回"
		}())

	return &result, nil
}
