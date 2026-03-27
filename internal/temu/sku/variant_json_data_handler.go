// Package sku 提供TEMU平台的变体JSON数据处理功能
package sku

import (
	"context"
	"fmt"
	"strings"

	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/app/ports"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/strx"
	domainProduct "task-processor/internal/product"
	temucontext "task-processor/internal/temu/context"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// VariantJsonDataHandler 变体JSON数据处理器
type VariantJsonDataHandler struct {
	logger         *logrus.Entry
	productFetcher appProduct.ProductFetcher
	amazonConfig   *config.AmazonConfig
}

// NewVariantJsonDataHandler 创建新的变体JSON数据处理器
func NewVariantJsonDataHandler(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor ports.ProductSource,
	rabbitmqClient *rabbitmq.Client,
) *VariantJsonDataHandler {
	log := logger.GetGlobalLogger("VariantJsonDataHandler")

	factory := appProduct.NewFetcherFactory()
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, amazonProcessor, rabbitmqClient)
	if err != nil {
		log.Errorf("创建产品获取器失败，降级到本地获取器: %v", err)
		fetcher = domainProduct.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, amazonProcessor)
	}

	return &VariantJsonDataHandler{
		logger:         log,
		productFetcher: fetcher,
		amazonConfig:   &cfg.Amazon,
	}
}

// Name 返回处理器名称
func (h *VariantJsonDataHandler) Name() string {
	return "变体JSON数据处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *VariantJsonDataHandler) Handle(ctx pipeline.TaskContext) error {
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *VariantJsonDataHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始处理变体JSON数据")

	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	variantAsins := h.getAsinListFromContext(temuCtx)
	if len(variantAsins) == 0 {
		h.logger.Info("未发现变体ASIN列表，使用单一产品模式")
		return h.processSingleProduct(temuCtx)
	}

	h.logger.Infof("找到 %d 个变体ASIN", len(variantAsins))

	if len(variantAsins) > 100 {
		h.logger.Warnf("变体ASIN数量过多（%d），停止处理", len(variantAsins))
		return fmt.Errorf("NONRETRYABLE: 变体ASIN数量过多，停止处理")
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
		h.logger.Errorf("批量获取变体数据失败: %v", err)
		return fmt.Errorf("批量获取变体数据失败: %w", err)
	}

	temuCtx.SetVariants(variants)

	return h.processVariantData(temuCtx, variants)
}

// getAsinListFromContext 从上下文中获取ASIN列表
func (h *VariantJsonDataHandler) getAsinListFromContext(temuCtx *temucontext.TemuTaskContext) []string {
	task := temuCtx.GetTask()
	if task == nil {
		return []string{}
	}

	mainProductAsin := strings.TrimSpace(strings.ToUpper(task.ProductID))

	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct != nil && amazonProduct.Asin != "" {
		mainProductAsin = strings.TrimSpace(strings.ToUpper(amazonProduct.Asin))
	}

	h.logger.Infof("🔍 [变体ASIN提取] 主产品ASIN: %s", mainProductAsin)

	if len(temuCtx.AsinSkuMap) > 0 {
		h.logger.Infof("🔍 [变体ASIN提取] 从AsinSkuMap获取，总数: %d", len(temuCtx.AsinSkuMap))
		return h.getAsinListFromMap(temuCtx.AsinSkuMap)
	}

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

	if len(temuCtx.VariantAsins) > 0 {
		h.logger.Infof("🔍 [变体ASIN提取] 从VariantAsins获取，总数: %d", len(temuCtx.VariantAsins))
		return temuCtx.VariantAsins
	}

	h.logger.Info("🔍 [变体ASIN提取] 未找到任何变体ASIN数据源")
	return []string{}
}

// getAsinListFromMap 从AsinSkuMap中提取所有ASIN
func (h *VariantJsonDataHandler) getAsinListFromMap(asinSkuMap map[string]string) []string {
	return extractAsinListFromMap(asinSkuMap)
}

// processSingleProduct 处理单一产品（无变体）
func (h *VariantJsonDataHandler) processSingleProduct(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("处理单一产品模式")

	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct != nil && temuCtx.TemuProduct != nil {
		if amazonProduct.Title != "" {
			temuCtx.CleanedTitle = strx.CleanProductTitle(amazonProduct.Title)
		}
		if amazonProduct.Description != "" {
			temuCtx.ProductDescription = amazonProduct.Description
		}
	}

	return nil
}

// processVariantData 处理变体数据
func (h *VariantJsonDataHandler) processVariantData(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) error {
	h.logger.Info("开始处理产品变体数据")

	if len(variants) == 0 {
		h.logger.Info("未发现变体数据，使用单一产品模式")
		return h.processSingleProduct(temuCtx)
	}

	h.logger.Infof("发现 %d 个变体", len(variants))

	for i, variant := range variants {
		if variant == nil {
			continue
		}
		if variant.Title != "" {
			original := variant.Title
			variant.Title = strx.CleanProductTitle(variant.Title)
			if original != variant.Title {
				h.logger.Debugf("变体 %d 标题已清理: %s -> %s", i+1, original, variant.Title)
			}
		}
		h.logger.Infof("处理变体 %d: %s (ASIN: %s)", i+1, variant.Title, variant.Asin)
	}

	if variants[0] != nil {
		if variants[0].Title != "" {
			temuCtx.CleanedTitle = strx.CleanProductTitle(variants[0].Title)
		}
		if variants[0].Description != "" {
			temuCtx.ProductDescription = variants[0].Description
		}
	}

	h.logger.Info("变体数据处理完成")
	return nil
}

// GetVariantByAsinFromVariants 通过ASIN从变体列表中获取变体
func (h *VariantJsonDataHandler) GetVariantByAsinFromVariants(variants []*model.Product, asin string) *model.Product {
	for _, variant := range variants {
		if variant != nil && variant.Asin == asin {
			return variant
		}
	}
	return nil
}

// Shutdown 关闭处理器，释放资源
func (h *VariantJsonDataHandler) Shutdown() {
	h.logger.Debug("VariantJsonDataHandler 关闭")
}
