package productdata

import (
	"fmt"
	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/infra/rabbitmq"
	shein "task-processor/internal/platforms/shein"

	"github.com/sirupsen/logrus"
)

// SubmitRawJsonDataHandler 提交原始JSON数据到服务器缓存处理器
type SubmitRawJsonDataHandler struct {
	logger  *logrus.Entry
	fetcher appProduct.ProductFetcher
}

// NewSubmitRawJsonDataHandler 创建新的提交原始JSON数据处理器
func NewSubmitRawJsonDataHandler(
	rawJsonDataClient product.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor any,
	rabbitmqClient *rabbitmq.Client,
) *SubmitRawJsonDataHandler {
	logger := logrus.WithField("handler", "SubmitRawJsonDataHandler")

	var ap *amazon.AmazonProcessor
	if amazonProcessor != nil {
		if processor, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
			ap = processor
		}
	}

	factory := appProduct.NewFetcherFactory()
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, ap, rabbitmqClient)
	if err != nil {
		logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
		fetcher = product.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, ap)
	}

	return &SubmitRawJsonDataHandler{logger: logger, fetcher: fetcher}
}

// Name 返回处理器名称
func (h *SubmitRawJsonDataHandler) Name() string {
	return "提交原始JSON数据到服务器缓存"
}

// Handle 执行提交原始JSON数据处理
func (h *SubmitRawJsonDataHandler) Handle(ctx *shein.TaskContext) error {
	if ctx.AmazonProduct == nil {
		h.logger.Warn("产品数据未获取，跳过提交原始JSON数据步骤")
		return nil
	}
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}
	if ctx.Task.ProductID == "" {
		return fmt.Errorf("产品ID为空")
	}

	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.Platform,
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	if err := h.fetcher.CacheProduct(req, ctx.AmazonProduct); err != nil {
		h.logger.Warnf("⚠️ 缓存产品数据失败: %v", err)
		return nil
	}

	h.logger.Infof("✅ 产品数据已缓存: ProductID=%s", ctx.Task.ProductID)
	if ctx.Variants != nil && len(*ctx.Variants) > 0 {
		h.logger.Infof("产品包含 %d 个变体，变体数据将在后续处理中提交", len(*ctx.Variants))
	}
	return nil
}

// SubmitVariantRawJsonDataHandler 提交变体原始JSON数据到服务器缓存处理器
type SubmitVariantRawJsonDataHandler struct {
	logger  *logrus.Entry
	fetcher appProduct.ProductFetcher
}

// NewSubmitVariantRawJsonDataHandler 创建新的提交变体原始JSON数据处理器
func NewSubmitVariantRawJsonDataHandler(
	rawJsonDataClient product.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor any,
	rabbitmqClient *rabbitmq.Client,
) *SubmitVariantRawJsonDataHandler {
	logger := logrus.WithField("handler", "SubmitVariantRawJsonDataHandler")

	var ap *amazon.AmazonProcessor
	if amazonProcessor != nil {
		if processor, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
			ap = processor
		}
	}

	factory := appProduct.NewFetcherFactory()
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, ap, rabbitmqClient)
	if err != nil {
		logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
		fetcher = product.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, ap)
	}

	return &SubmitVariantRawJsonDataHandler{logger: logger, fetcher: fetcher}
}

// Name 返回处理器名称
func (h *SubmitVariantRawJsonDataHandler) Name() string {
	return "提交变体原始JSON数据到服务器缓存"
}

// Handle 执行提交变体原始JSON数据处理
func (h *SubmitVariantRawJsonDataHandler) Handle(ctx *shein.TaskContext) error {
	if ctx.Variants == nil || len(*ctx.Variants) == 0 {
		h.logger.Debug("没有变体数据，跳过提交变体原始JSON数据步骤")
		return nil
	}
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.Platform,
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	variants := make([]*model.Product, len(*ctx.Variants))
	for i := range *ctx.Variants {
		variants[i] = &(*ctx.Variants)[i]
	}

	if err := h.fetcher.CacheVariants(req, variants); err != nil {
		h.logger.Warnf("⚠️ 缓存变体数据失败: %v", err)
		return nil
	}

	h.logger.Infof("✅ 变体数据已缓存: 数量=%d", len(*ctx.Variants))
	return nil
}
