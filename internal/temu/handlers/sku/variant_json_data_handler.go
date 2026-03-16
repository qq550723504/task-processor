// Package sku 提供TEMU平台的变体JSON数据处理功能
package sku

import (
	"context"
	"fmt"
	"strings"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/timeout"
	"task-processor/internal/pkg/recovery"
	"task-processor/internal/pkg/strx"
	temucontext "task-processor/internal/temu/context"

	"github.com/sirupsen/logrus"
)

// VariantJsonDataHandler 变体JSON数据处理器（使用公共组件）
type VariantJsonDataHandler struct {
	logger         *logrus.Entry
	productFetcher *product.ProductFetcher
	amazonConfig   *config.AmazonConfig
}

// NewVariantJsonDataHandler 创建新的变体JSON数据处理器
func NewVariantJsonDataHandler(
	rawJsonDataClient product.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	amazonProcessor *amazon.AmazonProcessor,
) *VariantJsonDataHandler {
	return &VariantJsonDataHandler{
		logger:         logrus.WithField("handler", "VariantJsonDataHandler"),
		productFetcher: product.NewProductFetcher(rawJsonDataClient, amazonConfig, amazonProcessor),
		amazonConfig:   amazonConfig,
	}
}

// Name 返回处理器名称
func (h *VariantJsonDataHandler) Name() string {
	return "变体JSON数据处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *VariantJsonDataHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *VariantJsonDataHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始处理变体JSON数据")

	// 检查任务上下文中的必要数据
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 获取变体ASIN列表
	variantAsins := h.getAsinListFromContext(temuCtx)
	if len(variantAsins) == 0 {
		h.logger.Info("未发现变体ASIN列表，使用单一产品模式")
		return h.processSingleProduct(temuCtx)
	}

	h.logger.Infof("找到 %d 个变体ASIN", len(variantAsins))

	// 检查变体数量限制
	if len(variantAsins) > 100 {
		h.logger.Warnf("变体ASIN数量过多（%d），可能会导致处理时间过长或请求失败", len(variantAsins))
		return fmt.Errorf("NONRETRYABLE: 变体ASIN数量过多，停止处理")
	}

	// 使用公共ProductFetcher批量获取变体数据
	variants, err := h.fetchAllVariants(temuCtx, variantAsins)
	if err != nil {
		h.logger.Errorf("获取变体数据失败: %v", err)
		return fmt.Errorf("获取变体数据失败: %w", err)
	}

	// 将变体数据存储到上下文中
	temuCtx.SetVariants(variants)

	// 处理变体数据
	err = h.processVariantData(temuCtx, variants)
	if err != nil {
		h.logger.Errorf("处理变体数据失败: %v", err)
		return fmt.Errorf("处理变体数据失败: %w", err)
	}

	h.logger.Info("变体JSON数据处理完成")
	return nil
}

// fetchAllVariants 批量获取所有变体数据（使用公共ProductFetcher）
func (h *VariantJsonDataHandler) fetchAllVariants(temuCtx *temucontext.TemuTaskContext, variantAsins []string) ([]*model.Product, error) {
	variants := make([]*model.Product, 0, len(variantAsins))

	task := temuCtx.GetTask()
	if task == nil {
		return nil, fmt.Errorf("任务信息为空")
	}

	h.logger.Infof("🚀 开始批量获取 %d 个变体数据", len(variantAsins))

	successCount := 0
	for i, asin := range variantAsins {
		// 显示进度
		h.logger.Infof("📦 获取变体 [%d/%d]: %s", i+1, len(variantAsins), asin)

		// 为每个变体请求设置2分钟超时
		ctx, cancel := timeout.WithTaskTimeout(context.Background())

		// 构建获取请求
		req := &product.FetchRequest{
			TenantID:   task.TenantID,
			Platform:   task.Platform,
			Region:     task.Region,
			ProductID:  asin,
			StoreID:    task.StoreID,
			CategoryID: task.CategoryID,
			Creator:    task.Creator,
		}

		// 使用公共ProductFetcher获取变体数据（带超时控制）
		variant, err := h.fetchVariantWithTimeout(ctx, req)
		cancel() // 及时释放资源

		if err != nil {
			h.logger.Warnf("❌ 变体 [%d/%d] %s 获取失败: %v", i+1, len(variantAsins), asin, err)
			continue
		}

		variants = append(variants, variant)
		successCount++
		h.logger.Infof("✅ 变体 [%d/%d] %s 获取成功", i+1, len(variantAsins), asin)
	}

	h.logger.Infof("🎉 批量获取完成: 成功 %d/%d 个变体数据", successCount, len(variantAsins))
	return variants, nil
}

// fetchVariantWithTimeout 带超时控制的变体获取
func (h *VariantJsonDataHandler) fetchVariantWithTimeout(ctx context.Context, req *product.FetchRequest) (*model.Product, error) {
	// 创建结果通道
	resultChan := make(chan *model.Product, 1)
	errorChan := make(chan error, 1)

	// 在goroutine中执行获取操作
	go func() {
		var err error
		defer recovery.RecoverWithError("变体获取", h.logger, &err)
		defer func() {
			if err != nil {
				errorChan <- err
			}
		}()

		variant, err := h.productFetcher.FetchProduct(req)
		if err != nil {
			return
		}
		resultChan <- variant
	}()

	// 等待结果或超时
	select {
	case variant := <-resultChan:
		return variant, nil
	case err := <-errorChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("变体获取超时: %s", req.ProductID)
	}
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

	// 1. 从AsinSkuMap中获取
	if len(temuCtx.AsinSkuMap) > 0 {
		h.logger.Infof("🔍 [变体ASIN提取] 从AsinSkuMap获取，总数: %d", len(temuCtx.AsinSkuMap))
		return h.getAsinListFromMap(temuCtx.AsinSkuMap)
	}

	// 2. 从Amazon产品的变体中获取
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
func (h *VariantJsonDataHandler) getAsinListFromMap(asinSkuMap map[string]string) []string {
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
func (h *VariantJsonDataHandler) processSingleProduct(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("处理单一产品模式")

	var productName, description string

	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct != nil {
		productName = amazonProduct.Title
		description = amazonProduct.Description
	}

	// 处理产品信息
	if temuCtx.TemuProduct != nil {
		if productName != "" {
			cleanedTitle := strx.CleanProductTitle(productName)
			temuCtx.CleanedTitle = cleanedTitle
			h.logger.Debugf("产品标题已清理: %s -> %s", productName, cleanedTitle)
		}
		if description != "" {
			temuCtx.ProductDescription = description
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

	// 处理每个变体
	for i, variant := range variants {
		if variant == nil {
			continue
		}

		// 清理变体标题
		if variant.Title != "" {
			originalTitle := variant.Title
			variant.Title = strx.CleanProductTitle(variant.Title)
			if originalTitle != variant.Title {
				h.logger.Debugf("变体 %d 标题已清理: %s -> %s", i+1, originalTitle, variant.Title)
			}
		}

		h.logger.Infof("处理变体 %d: %s (ASIN: %s)", i+1, variant.Title, variant.Asin)
	}

	// 设置主产品信息（使用第一个变体的信息）
	if len(variants) > 0 && variants[0] != nil {
		mainVariant := variants[0]
		if mainVariant.Title != "" {
			cleanedTitle := strx.CleanProductTitle(mainVariant.Title)
			temuCtx.CleanedTitle = cleanedTitle
			h.logger.Debugf("主变体标题已清理: %s -> %s", mainVariant.Title, cleanedTitle)
		}
		if mainVariant.Description != "" {
			temuCtx.ProductDescription = mainVariant.Description
		}
	}

	h.logger.Info("变体数据处理完成")
	return nil
}

// GetVariantByAsinFromVariants 通过ASIN从变体列表中获取变体
func (h *VariantJsonDataHandler) GetVariantByAsinFromVariants(variants []*model.Product, asin string) *model.Product {
	if variants == nil {
		return nil
	}
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
