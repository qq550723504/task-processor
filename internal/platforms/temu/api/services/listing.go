package services

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

const (
	// 重新上架接口
	relistProductEndpoint = "/mms/marigold/sku/online"
	// 下架产品接口
	delistProductEndpoint = "/mms/marigold/sku/offline"
)

// ListingAPI 商品上架API管理器
type ListingAPI struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewListingAPI 创建新的商品上架API管理器
func NewListingAPI(client client.APIClientInterface, logger *logrus.Entry) *ListingAPI {
	return &ListingAPI{
		client: client,
		logger: logger,
	}
}

// RelistProduct 重新上架产品
func (l *ListingAPI) RelistProduct(goodsID string, skuIDs []string) (*models.RelistProductResponse, error) {
	l.logger.Infof("重新上架产品: goodsID=%s, skuIDs数量=%d, skuIDs=%v", goodsID, len(skuIDs), skuIDs)

	// 参数校验
	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	if len(skuIDs) == 0 {
		return nil, fmt.Errorf("SKU ID列表不能为空")
	}

	req := &models.RelistProductRequest{
		GoodsID: goodsID,
		SkuIDs:  skuIDs,
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]any{
		"method":  "POST",
		"url":     relistProductEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result models.RelistProductResponse
	if err := l.client.SendTEMURequest(request, &result); err != nil {
		l.logger.WithError(err).Error("重新上架产品失败")
		return nil, fmt.Errorf("重新上架产品失败: %w", err)
	}

	if !result.Success {
		l.logger.Errorf("重新上架产品失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("重新上架产品失败: errorCode=%d", result.ErrorCode)
	}

	l.logger.Infof("成功重新上架产品: goodsID=%s", goodsID)
	return &result, nil
}
