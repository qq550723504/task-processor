// Package product 提供产品领域服务
package product

import (
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"

	"github.com/sirupsen/logrus"
)

// ProductFetcher 产品获取器
type ProductFetcher struct {
	rawJsonDataClient RawJsonDataClient
	amazonConfig      *config.AmazonConfig
	amazonProcessor   *amazon.AmazonProcessor
	logger            *logrus.Entry
}

// NewProductFetcher 创建产品获取器
func NewProductFetcher(
	rawJsonDataClient RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	amazonProcessor *amazon.AmazonProcessor,
) *ProductFetcher {
	return &ProductFetcher{
		rawJsonDataClient: rawJsonDataClient,
		amazonConfig:      amazonConfig,
		amazonProcessor:   amazonProcessor,
		logger:            logrus.New().WithField("component", "ProductFetcher"),
	}
}

// FetchProduct 获取产品
func (f *ProductFetcher) FetchProduct(req *FetchRequest) (*model.Product, error) {
	if f.rawJsonDataClient != nil {
		resp, err := f.rawJsonDataClient.GetRawJsonData(&RawJsonReq{
			TenantID:   req.TenantID,
			Platform:   req.Platform,
			ProductID:  req.ProductID,
			Region:     req.Region,
			StoreID:    req.StoreID,
			CategoryID: req.CategoryID,
			Creator:    req.Creator,
		})
		if err == nil && resp != nil && resp.RawJSONData != "" {
			f.logger.Debugf("从缓存获取产品成功: %s", req.ProductID)
			// TODO: 解析缓存数据
			return nil, nil
		}
	}

	if f.amazonProcessor != nil && f.amazonConfig != nil && f.amazonConfig.Enabled {
		f.logger.Debugf("使用爬虫获取产品: %s", req.ProductID)
		return f.amazonProcessor.Process("", "")
	}

	return nil, nil
}

// FetchProductWithRetry 带重试的产品获取
func (f *ProductFetcher) FetchProductWithRetry(productID, region string, storeID int64, maxRetries int) (*model.Product, error) {
	req := &FetchRequest{ProductID: productID, Region: region, StoreID: storeID}
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		product, err := f.FetchProduct(req)
		if err == nil {
			return product, nil
		}
		lastErr = err
		f.logger.Warnf("第%d次尝试获取产品失败: %v", i+1, err)
	}
	return nil, lastErr
}

// CacheProduct 缓存产品数据到服务器
func (f *ProductFetcher) CacheProduct(req *FetchRequest, product *model.Product) error {
	if product == nil {
		f.logger.Warn("产品数据为空，跳过缓存")
		return nil
	}
	if f.rawJsonDataClient == nil {
		f.logger.Warn("rawJsonDataClient未初始化，无法缓存")
		return nil
	}
	// TODO: 实现缓存逻辑
	return nil
}

// CacheVariants 批量缓存变体数据到服务器
func (f *ProductFetcher) CacheVariants(req *FetchRequest, variants []*model.Product) error {
	if len(variants) == 0 {
		return nil
	}
	// TODO: 实现批量缓存逻辑
	return nil
}

// GetStats 获取统计信息
func (f *ProductFetcher) GetStats() map[string]any {
	return map[string]any{"type": "local"}
}
