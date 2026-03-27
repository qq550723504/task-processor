// Package sku 提供并行变体数据处理功能
package sku

import (
	"context"
	"fmt"

	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/app/ports"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/perf"
	domainProduct "task-processor/internal/product"
	temucontext "task-processor/internal/temu/context"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ParallelVariantHandler 并行变体数据处理器
type ParallelVariantHandler struct {
	logger         *logrus.Entry
	productFetcher appProduct.ProductFetcher
	amazonConfig   *config.AmazonConfig
}

// NewParallelVariantHandler 创建并行变体数据处理器（支持分布式获取器）
func NewParallelVariantHandler(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor ports.ProductSource,
	rabbitmqClient *rabbitmq.Client,
) *ParallelVariantHandler {
	logger := logger.GetGlobalLogger("ParallelVariantHandler")

	factory := appProduct.NewFetcherFactory()
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, amazonProcessor, rabbitmqClient)
	if err != nil {
		logger.Errorf("创建产品获取器失败，降级到本地获取器: %v", err)
		fetcher = domainProduct.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, amazonProcessor)
	}

	return &ParallelVariantHandler{
		logger:         logger,
		productFetcher: fetcher,
		amazonConfig:   &cfg.Amazon,
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

// fetchVariantsParallel 批量获取变体数据（调用 FetchVariants 一次性提交所有任务）
func (h *ParallelVariantHandler) fetchVariantsParallel(temuCtx *temucontext.TemuTaskContext, variantAsins []string) ([]*model.Product, error) {
	task := temuCtx.GetTask()
	if task == nil {
		return nil, fmt.Errorf("任务信息为空")
	}

	req := &domainProduct.FetchRequest{
		TenantID:   task.TenantID,
		Platform:   task.Platform,
		Region:     task.Region,
		StoreID:    task.StoreID,
		CategoryID: task.CategoryID,
		Creator:    task.Creator,
	}

	variants, err := h.productFetcher.FetchVariants(context.Background(), req, variantAsins)
	if err != nil {
		return nil, err
	}

	h.logger.Infof("🎉 批量获取完成: 成功 %d/%d 个变体数据", len(variants), len(variantAsins))
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
	return extractAsinListFromMap(asinSkuMap)
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
