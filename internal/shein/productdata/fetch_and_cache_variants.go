package productdata

import (
	"context"
	"fmt"

	coreLogger "task-processor/internal/core/logger"
	appProduct "task-processor/internal/crawler/fetcher"
	"task-processor/internal/model"
	"task-processor/internal/pkg/perf"
	"task-processor/internal/product"
	shein "task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

type FetchAndCacheVariantsHandler struct {
	fetcher appProduct.ProductFetcher
	logger  *logrus.Entry
}

func NewFetchAndCacheVariantsHandler(fetcher appProduct.ProductFetcher) *FetchAndCacheVariantsHandler {
	logger := coreLogger.GetGlobalLogger("FetchAndCacheVariantsHandler")
	return &FetchAndCacheVariantsHandler{fetcher: fetcher, logger: logger}
}

func (h *FetchAndCacheVariantsHandler) Name() string {
	return "fetch_and_cache_variants"
}

func (h *FetchAndCacheVariantsHandler) Handle(ctx *shein.TaskContext) error {
	tracker := perf.NewTracker("fetch_and_cache_variants", h.logger)
	defer tracker.Finish()

	if ctx.Task == nil {
		return fmt.Errorf("task is nil")
	}

	mainProductAsin := ctx.Task.ProductID
	variantAsins := getAsinListFromContext(ctx, mainProductAsin, h.logger)
	if len(variantAsins) == 0 {
		h.logger.Infof("no variants found for product %s", mainProductAsin)
		ctx.SetVariants([]model.Product{})
		return nil
	}
	if len(variantAsins) > 100 {
		return shein.NewNonRetryableError("too many variant ASINs", nil)
	}

	tracker.StartStep("fetch_variants")
	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.GetSourcePlatformOrDefault(),
		Region:     ctx.Task.Region,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}
	variants, err := h.fetcher.FetchVariants(context.Background(), req, variantAsins)
	if err != nil {
		return fmt.Errorf("fetch variants failed: %w", err)
	}
	tracker.EndStep()

	variantList := make([]model.Product, 0, len(variants))
	for _, v := range variants {
		if v != nil {
			variantList = append(variantList, *v)
		}
	}
	ctx.SetVariants(variantList)

	cacheReq := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.GetSourcePlatformOrDefault(),
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}
	variantPtrs := make([]*model.Product, len(variantList))
	for i := range variantList {
		variantPtrs[i] = &variantList[i]
	}
	if err := h.fetcher.CacheVariants(cacheReq, variantPtrs); err != nil {
		h.logger.Warnf("cache variants failed: %v", err)
	}

	h.logger.Infof("loaded variants: %d/%d", len(variantList), len(variantAsins))
	return nil
}
