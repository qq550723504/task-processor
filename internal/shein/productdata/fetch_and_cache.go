package productdata

import (
	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	coreLogger "task-processor/internal/core/logger"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/product"
	shein "task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

// FetchAndCacheProductHandler 获取并缓存主产品数据处理器
// 合并原 RawJsonDataHandler + SubmitRawJsonDataHandler 两步为一步，
// 缓存失败仅记录警告，不阻断上架流程（仅影响后续任务的缓存命中率）
type FetchAndCacheProductHandler struct {
	fetcher appProduct.ProductFetcher
	logger  *logrus.Entry
}

// NewFetchAndCacheProductHandler 创建获取并缓存主产品数据处理器
func NewFetchAndCacheProductHandler(
	rawJsonDataClient product.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor product.AmazonScraper,
	rabbitmqClient *rabbitmq.Client,
) *FetchAndCacheProductHandler {
	logger := coreLogger.GetGlobalLogger("FetchAndCacheProductHandler")

	factory := appProduct.NewFetcherFactory()
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, amazonProcessor, rabbitmqClient)
	if err != nil {
		logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
		fetcher = product.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, amazonProcessor)
	}

	return &FetchAndCacheProductHandler{fetcher: fetcher, logger: logger}
}

// Name 返回处理器名称
func (h *FetchAndCacheProductHandler) Name() string {
	return "获取并缓存主产品数据"
}

// Handle 获取产品数据，成功后尝试缓存
func (h *FetchAndCacheProductHandler) Handle(ctx *shein.TaskContext) error {
	h.logger.Infof("开始获取原始JSON数据: ProductID=%s, Region=%s", ctx.Task.ProductID, ctx.Task.Region)

	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.SourcePlatform,
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	amazonProduct, err := h.fetcher.FetchProduct(ctx.Context, req)
	if err != nil {
		if isProductNotFoundError(err) {
			h.logger.Warnf("产品不存在，不需要重试: ProductID=%s, Error=%v", ctx.Task.ProductID, err)
			return shein.NewNonRetryableError("Amazon产品不存在", err)
		}
		return shein.NewRetryableError("获取产品数据失败", err)
	}

	ctx.AmazonProduct = amazonProduct

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
	if err := h.fetcher.CacheProduct(cacheReq, amazonProduct); err != nil {
		h.logger.Warnf("⚠️ 缓存产品数据失败（不影响本次上架）: %v", err)
	} else {
		h.logger.Infof("✅ 产品数据已缓存: ProductID=%s", ctx.Task.ProductID)
	}

	return nil
}
