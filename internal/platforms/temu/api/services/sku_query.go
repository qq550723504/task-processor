// Package services 提供TEMU平台SKU查询相关功能
package services

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

// SkuQueryAPI SKU查询API管理器
type SkuQueryAPI struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewSkuQueryAPI 创建新的SKU查询API管理器
func NewSkuQueryAPI(client client.APIClientInterface, logger *logrus.Entry) *SkuQueryAPI {
	return &SkuQueryAPI{
		client: client,
		logger: logger,
	}
}

// SkuQueryOptions SKU查询选项
type SkuQueryOptions struct {
	CommitID             string // 提交ID
	GoodsID              string // 商品ID
	SourceTypeOfSkuQuery int    // SKU查询来源类型，默认为1
	Source               int    // 来源，默认为0
}

// NewSkuQueryOptions 创建SKU查询选项
func NewSkuQueryOptions(commitID, goodsID string) SkuQueryOptions {
	return SkuQueryOptions{
		CommitID:             commitID,
		GoodsID:              goodsID,
		SourceTypeOfSkuQuery: 1,
		Source:               0,
	}
}

// WithSourceType 设置来源类型
func (opts SkuQueryOptions) WithSourceType(sourceType int) SkuQueryOptions {
	opts.SourceTypeOfSkuQuery = sourceType
	return opts
}

// WithSource 设置来源
func (opts SkuQueryOptions) WithSource(source int) SkuQueryOptions {
	opts.Source = source
	return opts
}

// QuerySkuPriceAndStock 查询SKU价格与库存
func (s *SkuQueryAPI) QuerySkuPriceAndStock(commitID, goodsID string) (*models.SkuQueryResponse, error) {

	if commitID == "" || goodsID == "" {
		return nil, fmt.Errorf("commitID 和 goodsID 不能为空")
	}

	// 构建请求体
	requestBody := models.SkuQueryRequest{
		CommitID:             commitID,
		GoodsID:              goodsID,
		SourceTypeOfSkuQuery: 1, // 默认来源类型
		Source:               0, // 默认来源
	}

	request := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/sku/query",
		"headers": map[string]string{
			"accept-language":    "en-US,en;q=0.9",
			"x-document-referer": "https://seller.temu.com/products.html",
		},
		"body": requestBody,
	}

	var result models.SkuQueryResponse
	authManager := client.NewAuthManager(s.logger)
	if err := authManager.SendRequestWithAuth(s.client, request, &result); err != nil {
		return nil, fmt.Errorf("调用 SKU 查询 API 失败: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API 返回错误: error_code=%d", result.ErrorCode)
	}

	return &result, nil
}

// QuerySkuPriceAndStockWithOptions 使用选项查询SKU价格与库存
func (s *SkuQueryAPI) QuerySkuPriceAndStockWithOptions(options SkuQueryOptions) (*models.SkuQueryResponse, error) {
	// 构建请求体
	requestBody := models.SkuQueryRequest{
		CommitID:             options.CommitID,
		GoodsID:              options.GoodsID,
		SourceTypeOfSkuQuery: options.SourceTypeOfSkuQuery,
		Source:               options.Source,
	}

	request := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/sku/query",
		"headers": map[string]string{
			"content-type":       "application/json;charset=UTF-8",
			"x-document-referer": "https://seller.temu.com/products.html",
		},
		"body": requestBody,
	}

	var result models.SkuQueryResponse
	authManager := client.NewAuthManager(s.logger)
	if err := authManager.SendRequestWithAuth(s.client, request, &result); err != nil {
		return nil, fmt.Errorf("调用 SKU 查询 API 失败: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API 返回错误: error_code=%d", result.ErrorCode)
	}

	s.logger.WithFields(logrus.Fields{
		"commit_id":                options.CommitID,
		"goods_id":                 options.GoodsID,
		"source_type_of_sku_query": options.SourceTypeOfSkuQuery,
		"source":                   options.Source,
		"total":                    result.Result.Total,
		"sku_count":                len(result.Result.SkuList),
	}).Info("成功获取 TEMU SKU 价格与库存信息")

	return &result, nil
}
