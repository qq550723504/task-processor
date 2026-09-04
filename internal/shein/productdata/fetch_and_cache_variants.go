package productdata

import (
	"context"
	"fmt"

	coreLogger "task-processor/internal/core/logger"
	appProduct "task-processor/internal/crawler/fetcher"
	"task-processor/internal/marketplace/sourceproduct"
	"task-processor/internal/model"
	"task-processor/internal/pkg/perf"
	shein "task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

type FetchAndCacheVariantsHandler struct {
	reader appProduct.ProductReader
	cache  appProduct.ProductCache
	logger *logrus.Entry
}

func NewFetchAndCacheVariantsHandler(reader appProduct.ProductReader, cache appProduct.ProductCache) *FetchAndCacheVariantsHandler {
	logger := coreLogger.GetGlobalLogger("FetchAndCacheVariantsHandler")
	return &FetchAndCacheVariantsHandler{reader: reader, cache: cache, logger: logger}
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
	if err := validateVariantASINCount(variantAsins); err != nil {
		return err
	}

	tracker.StartStep("fetch_variants")
	req := &sourceproduct.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.GetSourcePlatformOrDefault(),
		Region:     ctx.Task.Region,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}
	variants, err := h.reader.FetchVariants(context.Background(), req, variantAsins)
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

	cacheReq := &sourceproduct.FetchRequest{
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
	if err := h.cache.CacheVariants(cacheReq, variantPtrs); err != nil {
		h.logger.Warnf("cache variants failed: %v", err)
	}

	h.logger.Infof("loaded variants: %d/%d", len(variantList), len(variantAsins))
	return nil
}
