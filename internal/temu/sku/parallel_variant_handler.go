// Package sku 提供并行变体数据处理功能
package sku

import (
	"context"
	"fmt"
	"time"

	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/model"
	domainProduct "task-processor/internal/product"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/goroutine"
	"task-processor/internal/pkg/perf"
	temucontext "task-processor/internal/temu/context"

	"github.com/sirupsen/logrus"
)

// ParallelVariantHandler 并行变体数据处理器
type ParallelVariantHandler struct {
	logger         *logrus.Entry
	productFetcher appProduct.ProductFetcher
	amazonConfig   *config.AmazonConfig
	maxWorkers     int
	timeout        time.Duration
}

// NewParallelVariantHandler 创建并行变体数据处理器（支持分布式获取器）
func NewParallelVariantHandler(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor domainProduct.AmazonScraper,
	rabbitmqClient *rabbitmq.Client,
) *ParallelVariantHandler {
	logger := logrus.WithField("handler", "ParallelVariantHandler")

	// 直接使用浏览器池大小作为并发数，确保资源利用最优
	maxWorkers := cfg.Amazon.PoolSize
	if maxWorkers <= 0 {
		maxWorkers = 3 // 默认3个并发
	}

	// 使用工厂模式创建获取器
	factory := appProduct.NewFetcherFactory()

	// 根据配置创建获取器
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, amazonProcessor, rabbitmqClient)
	if err != nil {
		logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
		// 降级到本地获取器
		fetcher = domainProduct.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, amazonProcessor)
	}

	return &ParallelVariantHandler{
		logger:         logger,
		productFetcher: fetcher,
		amazonConfig:   &cfg.Amazon,
		maxWorkers:     maxWorkers,
		timeout:        2 * time.Minute, // 每个变体2分钟超时
	}
}

// Name 返回处理器名称
func (h *ParallelVariantHandler) Name() string {
	return "并行变体JSON数据处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *ParallelVariantHandler) Handle(ctx pipeline.TaskContext) error {
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *ParallelVariantHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	// 创建性能跟踪器
	tracker := perf.NewTracker("并行变体数据处理", h.logger)
	defer tracker.Finish()

	tracker.StartStep("初始化和验证")

	// 检查任务上下文
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 获取变体ASIN列表
	variantAsins := h.getAsinListFromContext(temuCtx)
	if len(variantAsins) == 0 {
		h.logger.Info("未发现变体ASIN列表，使用单一产品模式")
		tracker.EndStep()
		return h.processSingleProduct(temuCtx)
	}

	h.logger.Infof("找到 %d 个变体ASIN，准备并行处理", len(variantAsins))

	// 检查变体数量限制
	if len(variantAsins) > 100 {
		h.logger.Warnf("变体ASIN数量过多（%d），可能会导致处理时间过长", len(variantAsins))
		return fmt.Errorf("NONRETRYABLE: 变体ASIN数量过多，停止处理")
	}

	tracker.EndStep()
	tracker.StartStep("并行获取变体数据")

	// 并行获取变体数据
	variants, err := h.fetchVariantsParallel(temuCtx, variantAsins)
	if err != nil {
		h.logger.Errorf("并行获取变体数据失败: %v", err)
		return fmt.Errorf("并行获取变体数据失败: %w", err)
	}

	tracker.EndStep()
	tracker.StartStep("处理变体数据")

	// 将变体数据存储到上下文中
	temuCtx.SetVariants(variants)

	// 处理变体数据
	err = h.processVariantData(temuCtx, variants)
	if err != nil {
		h.logger.Errorf("处理变体数据失败: %v", err)
		return fmt.Errorf("处理变体数据失败: %w", err)
	}

	tracker.EndStep()

	h.logger.Info("并行变体JSON数据处理完成")
	return nil
}

// fetchVariantsParallel 并行获取变体数据
func (h *ParallelVariantHandler) fetchVariantsParallel(temuCtx *temucontext.TemuTaskContext, variantAsins []string) ([]*model.Product, error) {
	task := temuCtx.GetTask()
	if task == nil {
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
			Data: &domainProduct.FetchRequest{
				TenantID:   task.TenantID,
				Platform:   task.Platform,
				Region:     task.Region,
				ProductID:  asin,
				StoreID:    task.StoreID,
				CategoryID: task.CategoryID,
				Creator:    task.Creator,
			},
		}
	}

	// 定义处理函数
	processFunc := func(ctx context.Context, task *goroutine.Task) (any, error) {
		req, ok := task.Data.(*domainProduct.FetchRequest)
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

// getAsinListFromContext 从上下文中获取ASIN列表（复用原有逻辑）
func (h *ParallelVariantHandler) getAsinListFromContext(temuCtx *temucontext.TemuTaskContext) []string {
	task := temuCtx.GetTask()
	if task == nil {
		return []string{}
	}

	h.logger.Infof("🔍 [变体ASIN提取] 主产品ASIN: %s", task.ProductID)

	// 1. 从AsinSkuMap中获取
	if len(temuCtx.AsinSkuMap) > 0 {
		h.logger.Infof("🔍 [变体ASIN提取] 从AsinSkuMap获取，总数: %d", len(temuCtx.AsinSkuMap))
		return h.getAsinListFromMap(temuCtx.AsinSkuMap)
	}

	// 2. 从Amazon产品的变体中获取
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct != nil && len(amazonProduct.Variations) > 0 {
		h.logger.Infof("🔍 [变体ASIN提取] 从Variations获取，总数: %d", len(amazonProduct.Variations))
		asins := make([]string, 0, len(amazonProduct.Variations))
		for _, variation := range amazonProduct.Variations {
			if variation.Asin != "" {
				asins = append(asins, variation.Asin)
			}
		}
		return asins
	}

	// 3. 从其他数据源获取
	if len(temuCtx.VariantAsins) > 0 {
		h.logger.Infof("🔍 [变体ASIN提取] 从VariantAsins获取，总数: %d", len(temuCtx.VariantAsins))
		return temuCtx.VariantAsins
	}

	h.logger.Info("🔍 [变体ASIN提取] 未找到任何变体ASIN数据源")
	return []string{}
}

// getAsinListFromMap 从AsinSkuMap中提取所有ASIN
func (h *ParallelVariantHandler) getAsinListFromMap(asinSkuMap map[string]string) []string {
	if len(asinSkuMap) == 0 {
		return []string{}
	}

	asinList := make([]string, 0, len(asinSkuMap))
	for asin := range asinSkuMap {
		asinList = append(asinList, asin)
	}
	return asinList
}

// processSingleProduct 处理单一产品（无变体）
func (h *ParallelVariantHandler) processSingleProduct(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("处理单一产品模式")

	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct != nil && temuCtx.TemuProduct != nil {
		if amazonProduct.Title != "" {
			temuCtx.CleanedTitle = amazonProduct.Title
		}
		if amazonProduct.Description != "" {
			temuCtx.ProductDescription = amazonProduct.Description
		}
	}

	return nil
}

// processVariantData 处理变体数据
func (h *ParallelVariantHandler) processVariantData(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) error {
	h.logger.Info("开始处理产品变体数据")

	if len(variants) == 0 {
		h.logger.Info("未发现变体数据，使用单一产品模式")
		return h.processSingleProduct(temuCtx)
	}

	h.logger.Infof("发现 %d 个变体", len(variants))

	// 设置主产品信息（使用第一个变体的信息）
	if len(variants) > 0 && variants[0] != nil {
		mainVariant := variants[0]
		if mainVariant.Title != "" {
			temuCtx.CleanedTitle = mainVariant.Title
		}
		if mainVariant.Description != "" {
			temuCtx.ProductDescription = mainVariant.Description
		}
	}

	h.logger.Info("变体数据处理完成")
	return nil
}

// Shutdown 关闭处理器，释放资源
func (h *ParallelVariantHandler) Shutdown() {
	h.logger.Debug("ParallelVariantHandler 关闭")
}


