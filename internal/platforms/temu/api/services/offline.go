package services

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

const (
	// 已下架产品搜索接口
	offlineProductSearchEndpoint = "/mms/marigold/sku/v2/search"
)

// OfflineAPI 下架产品API管理器
type OfflineAPI struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewOfflineAPI 创建新的下架产品API管理器
func NewOfflineAPI(client client.APIClientInterface, logger *logrus.Entry) *OfflineAPI {
	return &OfflineAPI{
		client: client,
		logger: logger,
	}
}

// GetOfflineProducts 获取已下架产品列表
func (o *OfflineAPI) GetOfflineProducts(pageNo, pageSize int) (*models.OfflineProductSearchResponse, error) {
	o.logger.Infof("获取已下架产品列表: pageNo=%d, pageSize=%d", pageNo, pageSize)

	// 参数校验
	if pageSize > 200 {
		pageSize = 200 // 最大200
	}
	if pageSize <= 0 {
		pageSize = 50 // 默认50
	}
	if pageNo <= 0 {
		pageNo = 1 // 默认第1页
	}

	req := &models.OfflineProductSearchRequest{
		PageSize:              pageSize,
		PageNo:                pageNo,
		OrderType:             0,            // 0-降序
		OrderField:            "gmt_create", // 按创建时间排序
		EnableBatchSearchText: true,         // 启用批量搜索文本
		SkuSearchType:         3,            // 3-已下架产品
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/"

	request := map[string]any{
		"method":  "POST",
		"url":     offlineProductSearchEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result models.OfflineProductSearchResponse
	if err := o.client.SendTEMURequest(request, &result); err != nil {
		o.logger.WithError(err).Error("获取已下架产品列表失败")
		return nil, fmt.Errorf("获取已下架产品列表失败: %w", err)
	}

	if !result.Success {
		o.logger.Errorf("获取已下架产品列表失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("获取已下架产品列表失败: errorCode=%d", result.ErrorCode)
	}

	o.logger.Infof("成功获取已下架产品列表: 总数=%d, 当前页商品数=%d",
		result.Result.Total, len(result.Result.SkuList))
	return &result, nil
}
