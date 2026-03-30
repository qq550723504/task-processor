package productdata

import (
	appProduct "task-processor/internal/app/crawler/fetcher"
	coreLogger "task-processor/internal/core/logger"
	"task-processor/internal/product"
	shein "task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

type FetchAndCacheProductHandler struct {
	fetcher appProduct.ProductFetcher
	logger  *logrus.Entry
}

func NewFetchAndCacheProductHandler(fetcher appProduct.ProductFetcher) *FetchAndCacheProductHandler {
	return &FetchAndCacheProductHandler{fetcher: fetcher, logger: coreLogger.GetGlobalLogger("FetchAndCacheProductHandler")}
}

func (h *FetchAndCacheProductHandler) Name() string {
	return "fetch_and_cache_product"
}

func (h *FetchAndCacheProductHandler) Handle(ctx *shein.TaskContext) error {
	h.logger.Infof("fetch product data: product_id=%s region=%s", ctx.Task.ProductID, ctx.Task.Region)

	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.GetSourcePlatformOrDefault(),
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	amazonProduct, err := h.fetcher.FetchProduct(ctx.Context, req)
	if err != nil {
		if isProductNotFoundError(err) {
			h.logger.Warnf("product not found: product_id=%s err=%v", ctx.Task.ProductID, err)
			return shein.NewNonRetryableError("Amazon product not found", err)
		}
		return shein.NewRetryableError("fetch product data failed", err)
	}

	ctx.SetAmazonProduct(amazonProduct)

	cacheReq := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.GetSourcePlatformOrDefault(),
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}
	if err := h.fetcher.CacheProduct(cacheReq, amazonProduct); err != nil {
		h.logger.Warnf("cache product data failed: %v", err)
	} else {
		h.logger.Infof("cached product data: product_id=%s", ctx.Task.ProductID)
	}

	return nil
}
