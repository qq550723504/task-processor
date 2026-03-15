package productdata

import (
	"context"
	"fmt"
	"strings"
	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/core/config/types"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/pkg/goroutine"
	"task-processor/internal/pkg/perfutil"
	shein_model "task-processor/internal/platforms/shein/model"
	"time"

	"github.com/sirupsen/logrus"
)

// VariantJsonDataHandler 获取所有变体原始Json数据处理器（并行处理版本）
type VariantJsonDataHandler struct {
	logger         *logrus.Entry
	productFetcher appProduct.ProductFetcherInterface
	amazonConfig   *config.AmazonConfig
	maxWorkers     int
	timeout        time.Duration
}

// NewVariantJsonDataHandler 创建新的获取变体原始Json数据处理器
func NewVariantJsonDataHandler(
	rawJsonDataClient product.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	amazonProcessor any,
	rabbitmqClient *rabbitmq.Client,
) *VariantJsonDataHandler {
	logger := logrus.WithField("handler", "VariantJsonDataHandler")

	// 直接使用浏览器池大小作为并发数，确保资源利用最优
	maxWorkers := amazonConfig.PoolSize
	if maxWorkers <= 0 {
		maxWorkers = 3 // 默认3个并发
	}

	// 使用工厂模式创建获取器
	factory := appProduct.NewFetcherFactory()

	// 创建配置对象（用于工厂方法）
	cfg := &config.Config{
		Config: &types.Config{
			Amazon: *amazonConfig,
		},
	}

	// 根据配置创建获取器
	var fetcher appProduct.ProductFetcherInterface
	var err error

	if ap, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
		fetcher, err = factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, ap, rabbitmqClient)
		if err != nil {
			logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
			// 降级到本地获取器
			fetcher = product.NewProductFetcher(rawJsonDataClient, amazonConfig, ap)
		}
		logger.Info("[SHEIN] 变体数据使用共享的Amazon爬虫实例")
	} else {
		logger.Warn("变体数据处理器：没有提供Amazon处理器实例，Amazon功能将被禁用")
		// 创建一个基础的获取器（仅支持缓存）
		fetcher = product.NewProductFetcher(rawJsonDataClient, amazonConfig, nil)
	}

	return &VariantJsonDataHandler{
		logger:         logger,
		productFetcher: fetcher,
		amazonConfig:   amazonConfig,
		maxWorkers:     maxWorkers,
		timeout:        2 * time.Minute, // 每个变体2分钟超时
	}
}

// Name 返回处理器名称
func (h *VariantJsonDataHandler) Name() string {
	return "并行变体JSON数据处理器"
}

// Handle 执行获取所有变体的Json数据处理
func (h *VariantJsonDataHandler) Handle(ctx *shein_model.TaskContext) error {
	// 创建性能跟踪器
	tracker := perfutil.NewTracker("并行变体数据处理", h.logger)
	defer tracker.Finish()

	tracker.StartStep("初始化和验证")

	// 检查任务上下文
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 从上下文中获取所有变体ASIN列表
	mainProductAsin := ctx.Task.ProductID
	variantAsins := h.getAsinListFromContext(ctx, mainProductAsin)

	// 如果没有变体（单品情况），初始化空列表并继续
	if len(variantAsins) == 0 {
		h.logger.Infof("✅ 产品 %s 没有变体（单品），跳过变体数据获取", mainProductAsin)
		emptyVariants := make([]model.Product, 0)
		ctx.Variants = &emptyVariants
		tracker.EndStep()
		return nil
	}

	h.logger.Infof("找到 %d 个变体ASIN（包含所有可售卖的SKU）", len(variantAsins))

	// 检查变体数量限制
	if len(variantAsins) > 100 {
		h.logger.Warnf("变体ASIN数量过多（%d），可能会导致处理时间过长", len(variantAsins))
		return shein_model.NewNonRetryableError("变体ASIN数量过多，停止处理", nil)
	}

	tracker.EndStep()
	tracker.StartStep("并行获取变体数据")

	// 并行获取变体数据
	variants, err := h.fetchVariantsParallel(ctx, variantAsins)
	if err != nil {
		h.logger.Errorf("并行获取变体数据失败: %v", err)
		return fmt.Errorf("并行获取变体数据失败: %w", err)
	}

	tracker.EndStep()
	tracker.StartStep("处理变体数据")

	// 转换为 []model.Product 类型（去指针）
	variantList := make([]model.Product, 0, len(variants))
	for _, v := range variants {
		if v != nil {
			variantList = append(variantList, *v)
		}
	}

	// 将变体数据存储到上下文中
	ctx.Variants = &variantList

	tracker.EndStep()

	h.logger.Infof("✅ 最终获取到 %d/%d 个变体数据", len(variantList), len(variantAsins))
	return nil
}

// fetchVariantsParallel 并行获取变体数据
func (h *VariantJsonDataHandler) fetchVariantsParallel(ctx *shein_model.TaskContext, variantAsins []string) ([]*model.Product, error) {
	if ctx.Task == nil {
		return nil, fmt.Errorf("任务信息为空")
	}

	// 创建并行处理器
	processor := goroutine.NewProcessor(h.maxWorkers, h.timeout, h.logger)

	// 创建处理任务
	tasks := make([]*goroutine.Task, len(variantAsins))
	for i, asin := range variantAsins {
		tasks[i] = &goroutine.Task{
			Index: i,
			ID:    asin,
			Data: &product.FetchRequest{
				TenantID:   ctx.Task.TenantID,
				Platform:   ctx.Task.SourcePlatform,
				Region:     ctx.Task.Region,
				ProductID:  asin,
				StoreID:    ctx.Task.StoreID,
				CategoryID: ctx.Task.CategoryID,
				Creator:    ctx.Task.Creator,
			},
		}
	}

	// 定义处理函数
	processFunc := func(taskCtx context.Context, task *goroutine.Task) (interface{}, error) {
		req, ok := task.Data.(*product.FetchRequest)
		if !ok {
			return nil, fmt.Errorf("任务数据类型错误")
		}

		return h.productFetcher.FetchProduct(req)
	}

	// 并行执行处理
	results := processor.ProcessParallel(context.Background(), tasks, processFunc)

	// 处理结果
	variants := make([]*model.Product, 0, len(results))
	successCount := 0

	for _, result := range results {
		if result.Success && result.Data != nil {
			if variant, ok := result.Data.(*model.Product); ok {
				variants = append(variants, variant)
				successCount++
			}
		}
	}

	h.logger.Infof("🎉 并行获取完成: 成功 %d/%d 个变体数据", successCount, len(variantAsins))

	return variants, nil
}

// getAsinListFromContext 从上下文中获取ASIN列表
func (h *VariantJsonDataHandler) getAsinListFromContext(ctx *shein_model.TaskContext, mainProductAsin string) []string {
	h.logger.Infof("🔍 [变体ASIN提取] 主产品ASIN: %s", mainProductAsin)

	// 1. 从AsinSkuMap中获取
	if len(ctx.AsinSkuMap) > 0 {
		h.logger.Infof("🔍 [变体ASIN提取] 从AsinSkuMap获取，总数: %d", len(ctx.AsinSkuMap))
		return h.getAsinListFromMap(ctx.AsinSkuMap, mainProductAsin)
	}

	// 2. 从Amazon产品的变体中获取
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

	// 创建ASIN列表，包含所有ASIN（包括主产品）
	asinList := make([]string, 0, len(asinSkuMap))
	mainProductCount := 0

	for asin := range asinSkuMap {
		asinList = append(asinList, asin)

		// 统计主产品（仅用于日志）
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

// getVariantByAsinFromVariants 通过ASIN从Variants中获取变体
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
