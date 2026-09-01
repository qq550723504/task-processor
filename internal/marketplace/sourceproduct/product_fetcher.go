// Package sourceproduct owns marketplace source fetch/cache execution.
package sourceproduct

import (
	"context"
	"fmt"
	"io"
	"strings"

	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// ProductFetcher 产品获取器
type ProductFetcher struct {
	cacheManager  *CacheManager
	options       ProductFetcherOptions
	sourceFetcher SourceFetcher
	logger        *logrus.Entry
}

// NewProductFetcher 创建产品获取器
func NewProductFetcher(
	rawJsonDataClient RawJsonDataClient,
	options ProductFetcherOptions,
	sourceFetcher SourceFetcher,
) *ProductFetcher {
	return NewProductFetcherWithLogger(rawJsonDataClient, options, sourceFetcher, nil)
}

// NewProductFetcherWithLogger 创建产品获取器，支持传入自定义 logger。
func NewProductFetcherWithLogger(
	rawJsonDataClient RawJsonDataClient,
	options ProductFetcherOptions,
	sourceFetcher SourceFetcher,
	log *logrus.Entry,
) *ProductFetcher {
	if log == nil {
		localLogger := logrus.New()
		localLogger.SetOutput(io.Discard)
		log = localLogger.WithField("component", "product.fetcher")
	}
	return &ProductFetcher{
		cacheManager:  NewCacheManagerWithFreshness(rawJsonDataClient, log, options.DataFreshnessDays),
		options:       options,
		sourceFetcher: sourceFetcher,
		logger:        log,
	}
}

// FetchProduct 获取产品
func (f *ProductFetcher) FetchProduct(ctx context.Context, req *FetchRequest) (*model.Product, error) {
	if f.cacheManager != nil && f.shouldUseCache(req) {
		if product, err := f.cacheManager.GetFromCache(req); err == nil {
			f.logger.Debugf("got product from cache: %s", req.ProductID)
			return product, nil
		}
	}

	configured := sourceFetcherConfigured(f.sourceFetcher)
	if configured && f.options.Enabled {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		f.logger.Debugf("fetching product via crawler: %s", req.ProductID)
		product, err := f.sourceFetcher.Fetch(ctx, SourceFetchRequest{
			Region:    req.Region,
			ProductID: req.ProductID,
			Zipcode:   req.Zipcode,
		})
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return product, err
	}

	return nil, f.crawlerUnavailableError(req, configured)
}

// FetchProductWithRetry 带重试的产品获取
func (f *ProductFetcher) FetchProductWithRetry(productID, region string, storeID int64, maxRetries int) (*model.Product, error) {
	req := &FetchRequest{ProductID: productID, Region: region, StoreID: storeID}
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		product, err := f.FetchProduct(context.Background(), req)
		if err == nil {
			return product, nil
		}
		lastErr = err
		f.logger.Warnf("retry %d fetch product failed: %v", i+1, err)
	}
	return nil, lastErr
}

// CacheProduct 缓存产品数据到服务端
func (f *ProductFetcher) CacheProduct(req *FetchRequest, product *model.Product) error {
	if !f.shouldUseCache(req) {
		f.logger.Debug("skip cache because request uses explicit zipcode")
		return nil
	}
	if product == nil {
		f.logger.Warn("product is nil, skipping cache")
		return nil
	}
	if f.cacheManager == nil {
		f.logger.Warn("cacheManager is nil, cannot cache product")
		return nil
	}
	return f.cacheManager.CacheProduct(req, product)
}

// CacheVariants 批量缓存变体数据到服务端
func (f *ProductFetcher) CacheVariants(req *FetchRequest, variants []*model.Product) error {
	if len(variants) == 0 {
		return nil
	}
	if f.cacheManager == nil {
		f.logger.Warn("cacheManager is nil, cannot cache variants")
		return nil
	}
	return f.cacheManager.CacheVariants(req, variants)
}

// FetchVariants 批量获取变体数据
func (f *ProductFetcher) FetchVariants(ctx context.Context, req *FetchRequest, variantASINs []string) ([]*model.Product, error) {
	if len(variantASINs) == 0 {
		return []*model.Product{}, nil
	}

	variants := make([]*model.Product, 0, len(variantASINs))
	for _, asin := range variantASINs {
		variantReq := VariantFetchRequest(req, asin)
		product, err := f.FetchProduct(ctx, variantReq)
		if err != nil {
			f.logger.Warnf("fetch variant failed: ASIN=%s, err=%v", asin, err)
			continue
		}
		if product != nil {
			if strings.TrimSpace(product.Asin) != "" && product.Asin != asin {
				f.logger.Infof("variant fetch redirected ASIN: requested=%s actual=%s; preserving requested ASIN for downstream mapping", asin, product.Asin)
				normalized := *product
				normalized.Asin = asin
				product = &normalized
			}
			if cacheErr := f.CacheProduct(variantReq, product); cacheErr != nil {
				f.logger.Warnf("cache variant immediately failed: ASIN=%s, err=%v", asin, cacheErr)
			}
			variants = append(variants, product)
		}
	}
	return variants, nil
}

// GetStats 获取统计信息
func (f *ProductFetcher) GetStats() map[string]any {
	return map[string]any{"type": "local"}
}

func (f *ProductFetcher) shouldUseCache(req *FetchRequest) bool {
	return req == nil || strings.TrimSpace(req.Zipcode) == ""
}

func (f *ProductFetcher) crawlerUnavailableError(req *FetchRequest, configured bool) error {
	productID := ""
	region := ""
	if req != nil {
		productID = req.ProductID
		region = req.Region
	}

	switch {
	case !configured:
		return fmt.Errorf("crawler source is not configured for product fetch: product_id=%s region=%s", productID, region)
	case !f.options.Enabled:
		return fmt.Errorf("amazon crawler is disabled for product fetch: product_id=%s region=%s", productID, region)
	default:
		return fmt.Errorf("crawler fetch is unavailable for product fetch: product_id=%s region=%s", productID, region)
	}
}
