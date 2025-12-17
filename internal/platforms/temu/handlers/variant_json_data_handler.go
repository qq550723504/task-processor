package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"task-processor/internal/common/amazon"
	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/management/api"
	"task-processor/internal/common/pipeline"
	"task-processor/internal/common/product"
	"task-processor/internal/common/utils"
	"task-processor/internal/config"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// VariantJsonDataHandler 变体JSON数据处理器
type VariantJsonDataHandler struct {
	logger *logrus.Entry
	// 管理系统API客户端
	rawJsonDataClient api.RawJsonDataAPI
	// Amazon爬虫处理器
	amazonProcessor *amazon.AmazonProcessor
	// Amazon配置
	amazonConfig *config.AmazonConfig
}

// NewVariantJsonDataHandler 创建新的变体JSON数据处理器
func NewVariantJsonDataHandler(rawJsonDataClient api.RawJsonDataAPI, amazonConfig *config.AmazonConfig, amazonProcessor interface{}) *VariantJsonDataHandler {
	handler := &VariantJsonDataHandler{
		logger:            logrus.WithField("handler", "VariantJsonDataHandler"),
		rawJsonDataClient: rawJsonDataClient,
		amazonConfig:      amazonConfig,
	}

	// 使用共享的Amazon处理器（如果提供）
	if amazonProcessor != nil {
		if ap, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
			handler.amazonProcessor = ap
			logrus.Info("变体数据使用共享的Amazon爬虫实例")
		}
	} else if amazonConfig != nil && amazonConfig.Enabled {
		// 如果没有提供共享实例，则创建新的（向后兼容）
		handler.amazonProcessor = amazon.NewAmazonProcessor(amazonConfig)
		logrus.Info("变体数据Amazon爬虫已启用")
	}

	return handler
}

// Name 返回处理器名称
func (h *VariantJsonDataHandler) Name() string {
	return "变体JSON数据处理器"
}

// Handle 处理任务
func (h *VariantJsonDataHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始处理变体JSON数据")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 获取变体ASIN列表
	variantAsins := h.getAsinListFromContext(ctx)
	if len(variantAsins) == 0 {
		h.logger.Info("未发现变体ASIN列表，使用单一产品模式")
		return h.processSingleProduct(ctx)
	}

	h.logger.Infof("找到 %d 个变体ASIN", len(variantAsins))

	// 检查变体数量限制
	if len(variantAsins) > 100 {
		h.logger.Warnf("变体ASIN数量过多（%d），可能会导致处理时间过长或请求失败", len(variantAsins))
		return types.NewNonRetryableError("变体ASIN数量过多，停止处理", nil)
	}

	// 获取所有变体数据
	variants, err := h.fetchAllVariants(ctx, variantAsins)
	if err != nil {
		h.logger.Errorf("获取变体数据失败: %v", err)
		return fmt.Errorf("获取变体数据失败: %w", err)
	}

	// 将变体数据存储到上下文中（使用强类型）
	ctx.SetAmazonVariants(variants)

	// 处理变体数据
	err = h.processVariantData(ctx, variants)
	if err != nil {
		h.logger.Errorf("处理变体数据失败: %v", err)
		return fmt.Errorf("处理变体数据失败: %w", err)
	}

	h.logger.Info("变体JSON数据处理完成")
	return nil
}

// fetchAllVariants 获取所有变体数据
func (h *VariantJsonDataHandler) fetchAllVariants(ctx *pipeline.TaskContext, variantAsins []string) ([]*model.Product, error) {

	variants := make([]*model.Product, 0, len(variantAsins))
	missingAsins := make([]string, 0)

	for _, asin := range variantAsins {
		task := &api.RawJsonDataReqDTO{
			TenantID:   ctx.Task.TenantID,
			Platform:   ctx.Task.Platform,
			ProductID:  asin,
			Region:     ctx.Task.Region,
			StoreID:    ctx.Task.StoreID,
			CategoryID: ctx.Task.CategoryID,
			Creator:    ctx.Task.Creator,
		}

		rawJsonData, err := h.rawJsonDataClient.GetRawJsonData(task)
		if err == nil && rawJsonData != nil && rawJsonData.RawJSONData != "" {
			// 服务器有历史数据，直接使用
			variant, parseErr := h.parseAmazonProduct(rawJsonData.RawJSONData)
			if parseErr == nil {
				variants = append(variants, variant)
				h.logger.Infof("变体 %s 使用服务器历史数据", asin)
				continue
			}
			h.logger.Warnf("变体 %s 服务器数据解析失败: %v", asin, parseErr)
		}

		// 服务器没有数据或解析失败，记录需要抓取的ASIN
		missingAsins = append(missingAsins, asin)
	}

	h.logger.Infof("服务器有 %d/%d 个变体的历史数据，需要抓取 %d 个",
		len(variants), len(variantAsins), len(missingAsins))

	// 第二步：对于服务器没有数据的变体，判断是否使用爬虫抓取
	if len(missingAsins) > 0 {
		if h.shouldUseAmazonCrawler(ctx) {
			h.logger.Infof("使用Amazon爬虫批量抓取 %d 个缺失的变体", len(missingAsins))
			crawledVariants := h.fetchVariantsBatchFromAmazonCrawler(ctx, missingAsins)
			variants = append(variants, crawledVariants...)
		} else {
			h.logger.Warnf("服务器缺少 %d 个变体数据且无法抓取（非Amazon平台或未启用爬虫）", len(missingAsins))
		}
	}

	h.logger.Infof("最终获取到 %d/%d 个变体数据", len(variants), len(variantAsins))

	return variants, nil
}

// processSingleProduct 处理单一产品（无变体）
func (h *VariantJsonDataHandler) processSingleProduct(ctx *pipeline.TaskContext) error {
	h.logger.Info("处理单一产品模式")

	var productName, description string

	if ctx.AmazonProduct != nil {
		productName = ctx.AmazonProduct.Title
		description = ctx.AmazonProduct.Description
	}

	// 设置产品基本信息
	if ctx.TemuProduct != nil {
		if productName != "" {
			// 清理产品标题，移除特殊符号和表情符号
			cleanedTitle := utils.CleanProductTitle(productName)
			ctx.TemuProduct.GoodsBasic.GoodsName = cleanedTitle
			h.logger.Debugf("产品标题已清理: %s -> %s", productName, cleanedTitle)
		}
		if description != "" {
			ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc = description
		}
	}

	return nil
}

// processVariantData 处理变体数据
func (h *VariantJsonDataHandler) processVariantData(ctx *pipeline.TaskContext, variants []*model.Product) error {
	h.logger.Info("开始处理产品变体数据")

	if len(variants) == 0 {
		h.logger.Info("未发现变体数据，使用单一产品模式")
		return h.processSingleProduct(ctx)
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
			variant.Title = utils.CleanProductTitle(variant.Title)
			if originalTitle != variant.Title {
				h.logger.Debugf("变体 %d 标题已清理: %s -> %s", i+1, originalTitle, variant.Title)
			}
		}

		h.logger.Infof("处理变体 %d: %s (ASIN: %s)", i+1, variant.Title, variant.Asin)
	}

	// 设置主产品信息（使用第一个变体的信息）
	if len(variants) > 0 && variants[0] != nil && ctx.TemuProduct != nil {
		mainVariant := variants[0]
		if mainVariant.Title != "" {
			// 清理产品标题，移除特殊符号和表情符号
			cleanedTitle := utils.CleanProductTitle(mainVariant.Title)
			ctx.TemuProduct.GoodsBasic.GoodsName = cleanedTitle
			h.logger.Debugf("主变体标题已清理: %s -> %s", mainVariant.Title, cleanedTitle)
		}
		if mainVariant.Description != "" {
			ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc = mainVariant.Description
		}
	}

	h.logger.Info("变体数据处理完成")
	return nil
}

// getAsinListFromContext 从上下文中获取ASIN列表
// 注意：包含所有可售卖的ASIN，包括主产品ASIN（因为主产品也可能是可售卖的SKU）
func (h *VariantJsonDataHandler) getAsinListFromContext(ctx *pipeline.TaskContext) []string {
	// 获取主产品ASIN（仅用于日志记录）
	mainProductAsin := strings.TrimSpace(strings.ToUpper(ctx.Task.ProductID))

	// 如果有Amazon产品数据，优先使用其ASIN（更准确）
	if ctx.AmazonProduct != nil && ctx.AmazonProduct.Asin != "" {
		mainProductAsin = strings.TrimSpace(strings.ToUpper(ctx.AmazonProduct.Asin))
	}

	h.logger.Infof("🔍 [变体ASIN提取] 主产品ASIN: %s", mainProductAsin)

	// 1. 从AsinSkuMap中获取
	if asinSkuMapData, exists := ctx.GetData("AsinSkuMap"); exists {
		if asinSkuMap, ok := asinSkuMapData.(map[string]string); ok {
			h.logger.Infof("🔍 [变体ASIN提取] 从AsinSkuMap获取，总数: %d", len(asinSkuMap))
			return h.getAsinListFromMap(asinSkuMap, mainProductAsin)
		}
	}

	// 2. 从Amazon产品的变体中获取（包含所有ASIN）
	if ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Variations) > 0 {
		h.logger.Infof("🔍 [变体ASIN提取] 从Variations获取，总数: %d", len(ctx.AmazonProduct.Variations))

		asins := make([]string, 0, len(ctx.AmazonProduct.Variations))
		mainProductCount := 0

		for _, variation := range ctx.AmazonProduct.Variations {
			if variation.Asin == "" {
				continue
			}

			h.logger.Infof("✅ [变体ASIN提取] 添加变体: %s", variation.Asin)
			asins = append(asins, variation.Asin)

			// 统计主产品（仅用于日志）
			variantAsin := strings.TrimSpace(strings.ToUpper(variation.Asin))
			if variantAsin == mainProductAsin {
				mainProductCount++
			}
		}

		h.logger.Infof("🔍 [变体ASIN提取] 从Variations获取完成: 总变体数=%d (包含主产品=%d)",
			len(asins), mainProductCount)
		return asins
	}

	// 3. 从其他可能的数据源获取（包含所有ASIN）
	if variantAsinsData, exists := ctx.GetData("VariantAsins"); exists {
		if variantAsins, ok := variantAsinsData.([]string); ok {
			h.logger.Infof("🔍 [变体ASIN提取] 从VariantAsins获取，总数: %d", len(variantAsins))

			// 包含所有ASIN
			asins := make([]string, 0, len(variantAsins))
			mainProductCount := 0

			for _, asin := range variantAsins {
				h.logger.Infof("✅ [变体ASIN提取] 添加变体: %s", asin)
				asins = append(asins, asin)

				// 统计主产品（仅用于日志）
				normalizedAsin := strings.TrimSpace(strings.ToUpper(asin))
				if normalizedAsin == mainProductAsin {
					mainProductCount++
				}
			}

			h.logger.Infof("🔍 [变体ASIN提取] 从VariantAsins获取完成: 总变体数=%d (包含主产品=%d)",
				len(asins), mainProductCount)
			return asins
		}
	}

	h.logger.Info("🔍 [变体ASIN提取] 未找到任何变体ASIN数据源")
	return []string{}
}

// getAsinListFromMap 从AsinSkuMap中提取所有ASIN（包括主产品ASIN）
// 注意：主产品ASIN也可能是一个可售卖的SKU，不应该被排除
func (h *VariantJsonDataHandler) getAsinListFromMap(asinSkuMap map[string]string, mainProductAsin string) []string {
	if len(asinSkuMap) == 0 {
		return []string{}
	}

	// 创建ASIN列表，包含所有ASIN（包括主产品）
	asinList := make([]string, 0, len(asinSkuMap))
	mainProductCount := 0

	for asin := range asinSkuMap {
		h.logger.Infof("✅ [变体ASIN提取] 从AsinSkuMap添加变体: %s", asin)
		asinList = append(asinList, asin)

		// 统计主产品（仅用于日志）
		normalizedAsin := strings.TrimSpace(strings.ToUpper(asin))
		if normalizedAsin == mainProductAsin {
			mainProductCount++
		}
	}

	h.logger.Infof("🔍 [变体ASIN提取] 从AsinSkuMap获取完成: 总变体数=%d (包含主产品=%d)",
		len(asinList), mainProductCount)

	return asinList
}

// shouldUseAmazonCrawler 判断是否应该使用Amazon爬虫
func (h *VariantJsonDataHandler) shouldUseAmazonCrawler(ctx *pipeline.TaskContext) bool {
	if h.amazonProcessor == nil || h.amazonConfig == nil || !h.amazonConfig.Enabled {
		return false
	}

	platform := strings.ToLower(ctx.Task.Platform)
	return platform == "amazon"
}

// fetchVariantsBatchFromAmazonCrawler 批量抓取变体数据
func (h *VariantJsonDataHandler) fetchVariantsBatchFromAmazonCrawler(ctx *pipeline.TaskContext, asins []string) []*model.Product {
	if h.amazonProcessor == nil {
		h.logger.Error("Amazon爬虫未初始化")
		return []*model.Product{}
	}

	// 使用公共函数获取地区信息
	domain := product.GetAmazonDomainByRegion(ctx.Task.Region)
	zipcode := product.GetZipcodeForRegion(ctx.Task.Region, h.amazonConfig.Zipcodes)

	// 准备批量请求
	requests := make([]model.ProductRequest, 0, len(asins))
	for i, asin := range asins {
		url := fmt.Sprintf("https://www.%s/dp/%s?th=1&psc=1", domain, asin)
		requests = append(requests, model.ProductRequest{
			URL:     url,
			Zipcode: zipcode,
		})
		h.logger.Infof("🌐 [变体爬取] 准备请求 [%d/%d]: ASIN=%s, URL=%s", i+1, len(asins), asin, url)
	}

	h.logger.Infof("🚀 [变体爬取] 开始批量抓取 %d 个变体: Region=%s, Domain=%s, Zipcode=%s",
		len(requests), ctx.Task.Region, domain, zipcode)

	// 批量处理（使用同一个浏览器实例）
	results := h.amazonProcessor.ProcessBatch(requests)

	h.logger.Infof("✅ [变体爬取] 批量抓取完成，收到 %d 个结果", len(results))

	// 转换结果并保存到服务器
	variants := make([]*model.Product, 0, len(results))
	successCount := 0
	savedCount := 0

	for i, result := range results {
		if result.Error != nil {
			h.logger.Warnf("变体 %s 抓取失败: %v", asins[i], result.Error)
			continue
		}

		if result.Product != nil {
			variants = append(variants, result.Product)
			successCount++

			// 保存抓取到的变体数据到服务器缓存
			if err := h.saveVariantToServer(ctx, asins[i], result.Product); err != nil {
				h.logger.Warnf("保存变体 %s 到服务器失败: %v", asins[i], err)
			} else {
				savedCount++
				h.logger.Debugf("变体 %s 已保存到服务器缓存", asins[i])
			}
		}
	}

	h.logger.Infof("批量抓取完成: 成功抓取 %d/%d，成功保存 %d/%d",
		successCount, len(asins), savedCount, successCount)

	// 如果有保存失败的情况，记录警告
	if savedCount < successCount {
		h.logger.Warnf("有 %d 个变体数据保存失败，下次仍需重新抓取", successCount-savedCount)
	}

	return variants
}

// parseAmazonProduct 解析Amazon产品JSON数据
func (h *VariantJsonDataHandler) parseAmazonProduct(jsonData string) (*model.Product, error) {
	var product model.Product
	if err := json.Unmarshal([]byte(jsonData), &product); err != nil {
		return nil, fmt.Errorf("解析Amazon产品数据失败: %w", err)
	}

	// 重新计算 IsAvailable 字段（修复历史数据中的错误）
	product.IsAvailable = h.recalculateIsAvailable(&product)

	return &product, nil
}

// recalculateIsAvailable 重新计算产品是否可用
func (h *VariantJsonDataHandler) recalculateIsAvailable(product *model.Product) bool {
	lowerText := strings.ToLower(strings.TrimSpace(product.Availability))

	// 不可用的关键词（优先检查）
	unavailableKeywords := []string{
		"currently unavailable", "unavailable", "out of stock",
		"temporarily out of stock", "not available", "discontinued", "sold out",
		"no disponible", "agotado", "sin stock", "temporalmente agotado",
		"actualmente no disponible",
	}

	for _, keyword := range unavailableKeywords {
		if strings.Contains(lowerText, keyword) {
			h.logger.WithFields(logrus.Fields{
				"asin":         product.Asin,
				"availability": product.Availability,
				"keyword":      keyword,
			}).Info("❌ recalculate: 匹配到不可用关键词")
			return false
		}
	}

	// 可用的关键词
	availableKeywords := []string{
		"in stock", "available", "ships", "delivery", "arrives",
		"left in stock", "more on the way", "usually ships", "in stock soon",
		"disponible", "en stock", "envío", "entrega", "llega",
	}

	for _, keyword := range availableKeywords {
		if strings.Contains(lowerText, keyword) {
			h.logger.WithFields(logrus.Fields{
				"asin":         product.Asin,
				"availability": product.Availability,
				"keyword":      keyword,
			}).Info("✅ recalculate: 匹配到可用关键词")
			return true
		}
	}

	// 无法明确判断时，保持原有值
	h.logger.WithFields(logrus.Fields{
		"asin":           product.Asin,
		"availability":   product.Availability,
		"original_value": product.IsAvailable,
		"lower_text":     lowerText,
	}).Warn("⚠️ recalculate: 无法明确判断可用性，保持原有值")
	return product.IsAvailable
}

// saveVariantToServer 保存变体数据到服务器缓存
func (h *VariantJsonDataHandler) saveVariantToServer(ctx *pipeline.TaskContext, asin string, variant *model.Product) error {
	// 将变体数据序列化为JSON
	jsonData, err := json.Marshal(variant)
	if err != nil {
		return fmt.Errorf("序列化变体数据失败: %w", err)
	}

	// 构造保存请求
	saveReq := &api.RawJsonDataCreateReqDTO{
		Platform:    ctx.Task.Platform,
		Region:      ctx.Task.Region,
		ProductID:   asin,
		RawJsonData: string(jsonData),
		Creator:     ctx.Task.Creator,
	}

	// 调用管理系统API保存数据
	id, err := h.rawJsonDataClient.CreateRawJsonData(saveReq)
	if err != nil {
		return fmt.Errorf("调用保存API失败: %w", err)
	}

	h.logger.Debugf("变体 %s 保存成功，ID: %d", asin, id)
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

// Shutdown 关闭处理器，释放资源（现在由共享的Amazon处理器管理）
func (h *VariantJsonDataHandler) Shutdown() {
	// Amazon处理器由外部管理，不需要在这里关闭
	h.logger.Debug("VariantJsonDataHandler 关闭（Amazon处理器由外部管理）")
}
