package productdata

import (
	"context"
	"fmt"

	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	coreLogger "task-processor/internal/core/logger"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/model"
	"task-processor/internal/pkg/perf"
	"task-processor/internal/product"
	shein "task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

// FetchAndCacheVariantsHandler 获取并缓存变体数据处理器
// 合并原 VariantJsonDataHandler + SubmitVariantRawJsonDataHandler 两步为一步，
// 缓存失败仅记录警告，不阻断上架流程（仅影响后续任务的缓存命中率）
type FetchAndCacheVariantsHandler struct {
	fetcher      appProduct.ProductFetcher
	amazonConfig *config.AmazonConfig
	logger       *logrus.Entry
}

// NewFetchAndCacheVariantsHandler 创建获取并缓存变体数据处理器
func NewFetchAndCacheVariantsHandler(
	rawJsonDataClient product.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor product.AmazonScraper,
	rabbitmqClient *rabbitmq.Client,
) *FetchAndCacheVariantsHandler {
	logger := coreLogger.GetGlobalLogger("FetchAndCacheVariantsHandler")

	factory := appProduct.NewFetcherFactory()
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, amazonProcessor, rabbitmqClient)
	if err != nil {
		logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
		fetcher = product.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, amazonProcessor)
	}

	return &FetchAndCacheVariantsHandler{
		fetcher:      fetcher,
		amazonConfig: &cfg.Amazon,
		logger:       logger,
	}
}

// Name 返回处理器名称
func (h *FetchAndCacheVariantsHandler) Name() string {
	return "获取并缓存变体数据"
}

// Handle 获取变体数据，成功后尝试缓存
func (h *FetchAndCacheVariantsHandler) Handle(ctx *shein.TaskContext) error {
	tracker := perf.NewTracker("获取并缓存变体数据", h.logger)
	defer tracker.Finish()

	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	mainProductAsin := ctx.Task.ProductID
	variantAsins := getAsinListFromContext(ctx, mainProductAsin, h.logger)

	if len(variantAsins) == 0 {
		h.logger.Infof("✅ 产品 %s 没有变体（单品），跳过变体数据获取", mainProductAsin)
		emptyVariants := make([]model.Product, 0)
		ctx.Variants = &emptyVariants
		return nil
	}

	h.logger.Infof("找到 %d 个变体ASIN", len(variantAsins))

	if len(variantAsins) > 100 {
		h.logger.Warnf("变体ASIN数量过多（%d），停止处理", len(variantAsins))
		return shein.NewNonRetryableError("变体ASIN数量过多，停止处理", nil)
	}

	tracker.StartStep("并行获取变体数据")
	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.SourcePlatform,
		Region:     ctx.Task.Region,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	variants, err := h.fetcher.FetchVariants(context.Background(), req, variantAsins)
	if err != nil {
		return fmt.Errorf("并行获取变体数据失败: %w", err)
	}
	tracker.EndStep()

	variantList := make([]model.Product, 0, len(variants))
	for _, v := range variants {
		if v != nil {
			variantList = append(variantList, *v)
		}
	}
	ctx.Variants = &variantList
	h.logger.Infof("✅ 获取到 %d/%d 个变体数据", len(variantList), len(variantAsins))

	// 缓存失败不阻断本次上架，仅影响后续任务的缓存命中率
	cacheReq := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.Platform,
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
		h.logger.Warnf("⚠️ 缓存变体数据失败（不影响本次上架）: %v", err)
	} else {
		h.logger.Infof("✅ 变体数据已缓存: 数量=%d", len(variantList))
	}

	return nil
}
