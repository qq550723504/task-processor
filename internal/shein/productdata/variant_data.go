package productdata

import (
	"context"
	"fmt"
	"strings"

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

// VariantJsonDataHandler 获取所有变体原始Json数据处理器
type VariantJsonDataHandler struct {
	logger         *logrus.Entry
	productFetcher appProduct.ProductFetcher
	amazonConfig   *config.AmazonConfig
}

// NewVariantJsonDataHandler 创建新的获取变体原始Json数据处理器
func NewVariantJsonDataHandler(
	rawJsonDataClient product.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor product.AmazonScraper,
	rabbitmqClient *rabbitmq.Client,
) *VariantJsonDataHandler {
	logger := coreLogger.GetGlobalLogger("VariantJsonDataHandler")

	factory := appProduct.NewFetcherFactory()

	var fetcher appProduct.ProductFetcher
	var err error

	if amazonProcessor != nil {
		fetcher, err = factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, amazonProcessor, rabbitmqClient)
		if err != nil {
			logger.Errorf("创建产品获取器失败，降级到本地获取器: %v", err)
			fetcher = product.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, amazonProcessor)
		}
		logger.Info("[SHEIN] 变体数据使用分布式爬虫获取器")
	} else {
		logger.Warn("变体数据处理器：没有提供Amazon处理器实例，Amazon功能将被禁用")
		fetcher = product.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, nil)
	}

	return &VariantJsonDataHandler{
		logger:         logger,
		productFetcher: fetcher,
		amazonConfig:   &cfg.Amazon,
	}
}

// Name 返回处理器名称
func (h *VariantJsonDataHandler) Name() string {
	return "并行变体JSON数据处理器"
}

// Handle 执行获取所有变体的Json数据处理
func (h *VariantJsonDataHandler) Handle(ctx *shein.TaskContext) error {
	tracker := perf.NewTracker("并行变体数据处理", h.logger)
	defer tracker.Finish()

	tracker.StartStep("初始化和验证")

	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	mainProductAsin := ctx.Task.ProductID
	variantAsins := h.getAsinListFromContext(ctx, mainProductAsin)

	if len(variantAsins) == 0 {
		h.logger.Infof("✅ 产品 %s 没有变体（单品），跳过变体数据获取", mainProductAsin)
		emptyVariants := make([]model.Product, 0)
		ctx.Variants = &emptyVariants
		tracker.EndStep()
		return nil
	}

	h.logger.Infof("找到 %d 个变体ASIN（包含所有可售卖的SKU）", len(variantAsins))

	if len(variantAsins) > 100 {
		h.logger.Warnf("变体ASIN数量过多（%d），可能会导致处理时间过长", len(variantAsins))
		return shein.NewNonRetryableError("变体ASIN数量过多，停止处理", nil)
	}

	tracker.EndStep()
	tracker.StartStep("并行获取变体数据")

	variants, err := h.fetchVariantsParallel(ctx, variantAsins)
	if err != nil {
		h.logger.Errorf("并行获取变体数据失败: %v", err)
		return fmt.Errorf("并行获取变体数据失败: %w", err)
	}

	tracker.EndStep()
	tracker.StartStep("处理变体数据")

	variantList := make([]model.Product, 0, len(variants))
	for _, v := range variants {
		if v != nil {
			variantList = append(variantList, *v)
		}
	}

	ctx.Variants = &variantList

	tracker.EndStep()

	h.logger.Infof("✅ 最终获取到 %d/%d 个变体数据", len(variantList), len(variantAsins))
	return nil
}

// fetchVariantsParallel 批量获取变体数据（调用 FetchVariants 一次性提交所有任务）
func (h *VariantJsonDataHandler) fetchVariantsParallel(ctx *shein.TaskContext, variantAsins []string) ([]*model.Product, error) {
	if ctx.Task == nil {
		return nil, fmt.Errorf("任务信息为空")
	}

	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.SourcePlatform,
		Region:     ctx.Task.Region,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	variants, err := h.productFetcher.FetchVariants(context.Background(), req, variantAsins)
	if err != nil {
		return nil, err
	}

	h.logger.Infof("🎉 批量获取完成: 成功 %d/%d 个变体数据", len(variants), len(variantAsins))
	return variants, nil
}

// getAsinListFromContext 从上下文中获取ASIN列表
func (h *VariantJsonDataHandler) getAsinListFromContext(ctx *shein.TaskContext, mainProductAsin string) []string {
	h.logger.Infof("🔍 [变体ASIN提取] 主产品ASIN: %s", mainProductAsin)

	if len(ctx.AsinSkuMap) > 0 {
		h.logger.Infof("🔍 [变体ASIN提取] 从AsinSkuMap获取，总数: %d", len(ctx.AsinSkuMap))
		return h.getAsinListFromMap(ctx.AsinSkuMap, mainProductAsin)
	}

	if ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Variations) > 0 {
		h.logger.Infof("🔍 [变体ASIN提取] 从Variations获取，总数: %d", len(ctx.AmazonProduct.Variations))
		asins := make([]string, 0, len(ctx.AmazonProduct.Variations))
		for _, variation := range ctx.AmazonProduct.Variations {
			if variation.Asin != "" {
				asins = append(asins, variation.Asin)
			}
		}
		return asins
	}

	h.logger.Info("🔍 [变体ASIN提取] 未找到任何变体ASIN数据源")
	return []string{}
}

// getAsinListFromMap 从AsinSkuMap中提取所有ASIN（包括主产品ASIN）
func (h *VariantJsonDataHandler) getAsinListFromMap(asinSkuMap map[string]string, mainProductAsin string) []string {
	if len(asinSkuMap) == 0 {
		return []string{}
	}

	asinList := make([]string, 0, len(asinSkuMap))
	mainProductCount := 0

	for asin := range asinSkuMap {
		asinList = append(asinList, asin)

		normalizedAsin := strings.TrimSpace(strings.ToUpper(asin))
		normalizedMainAsin := strings.TrimSpace(strings.ToUpper(mainProductAsin))
		if normalizedAsin == normalizedMainAsin {
			mainProductCount++
		}
	}

	h.logger.Infof("🔍 [SHEIN变体] 从AsinSkuMap获取完成: 总变体数=%d (包含主产品=%d)",
		len(asinList), mainProductCount)

	return asinList
}

// Shutdown 关闭处理器，释放资源
func (h *VariantJsonDataHandler) Shutdown() {
	h.logger.Debug("[SHEIN] VariantJsonDataHandler 关闭")
}

// GetVariantByAsinFromVariants 通过ASIN从Variants中获取变体
func GetVariantByAsinFromVariants(variants *[]model.Product, asin string) *model.Product {
	if variants == nil {
		return nil
	}
	for _, variant := range *variants {
		if variant.Asin == asin {
			return &variant
		}
	}
	return nil
}
