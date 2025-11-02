package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"task-processor/common/amazon"
	"task-processor/common/config"
	"task-processor/common/management/api"
	"task-processor/common/pipeline"

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
func NewVariantJsonDataHandler(rawJsonDataClient api.RawJsonDataAPI, amazonConfig *config.AmazonConfig) *VariantJsonDataHandler {
	handler := &VariantJsonDataHandler{
		logger:            logrus.WithField("handler", "VariantJsonDataHandler"),
		rawJsonDataClient: rawJsonDataClient,
		amazonConfig:      amazonConfig,
	}

	// 如果启用Amazon爬虫，初始化处理器
	if amazonConfig != nil && amazonConfig.Enabled {
		handler.amazonProcessor = amazon.NewAmazonProcessor(amazonConfig)
		logrus.Info("变体数据Amazon爬虫已启用")
	}

	return handler
}

// NonRetryableError 不可重试的错误类型
type NonRetryableError struct {
	Message string
	Cause   error
}

func (e *NonRetryableError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// NewNonRetryableError 创建不可重试错误
func NewNonRetryableError(message string, cause error) *NonRetryableError {
	return &NonRetryableError{
		Message: message,
		Cause:   cause,
	}
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
		return NewNonRetryableError("变体ASIN数量过多，停止处理", nil)
	}

	// 获取所有变体数据
	variants, err := h.fetchAllVariants(ctx, variantAsins)
	if err != nil {
		h.logger.Errorf("获取变体数据失败: %v", err)
		return fmt.Errorf("获取变体数据失败: %w", err)
	}

	// 将变体数据存储到上下文中
	ctx.SetData("variants", variants)

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
func (h *VariantJsonDataHandler) fetchAllVariants(ctx *pipeline.TaskContext, variantAsins []string) ([]*amazon.Product, error) {
	h.logger.Infof("开始获取 %d 个变体的数据", len(variantAsins))

	variants := make([]*amazon.Product, 0, len(variantAsins))
	missingAsins := make([]string, 0)

	// 第一步：检查服务器是否有所有变体的历史数据
	h.logger.Infof("检查服务器是否有 %d 个变体的历史数据", len(variantAsins))

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
			ctx.TemuProduct.GoodsBasic.GoodsName = productName
		}
		if description != "" {
			ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc = description
		}
	}

	return nil
}

// processVariantData 处理变体数据
func (h *VariantJsonDataHandler) processVariantData(ctx *pipeline.TaskContext, variants []*amazon.Product) error {
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

		h.logger.Infof("处理变体 %d: %s (ASIN: %s)", i+1, variant.Title, variant.Asin)

		// 这里可以添加具体的变体处理逻辑
		// 例如：创建SKC、处理规格、设置价格等
		h.processVariantSKU(ctx, variant, i)
	}

	// 设置主产品信息（使用第一个变体的信息）
	if len(variants) > 0 && variants[0] != nil && ctx.TemuProduct != nil {
		mainVariant := variants[0]
		if mainVariant.Title != "" {
			ctx.TemuProduct.GoodsBasic.GoodsName = mainVariant.Title
		}
		if mainVariant.Description != "" {
			ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc = mainVariant.Description
		}
	}

	h.logger.Info("变体数据处理完成")
	return nil
}

// processVariantSKU 处理变体SKU
func (h *VariantJsonDataHandler) processVariantSKU(ctx *pipeline.TaskContext, variant *amazon.Product, index int) {
	// 这里可以实现具体的SKU处理逻辑
	// 例如：
	// 1. 生成SKU编号
	// 2. 设置价格
	// 3. 处理规格属性
	// 4. 设置库存

	h.logger.Debugf("处理变体SKU: ASIN=%s, Price=%.2f %s",
		variant.Asin, variant.FinalPrice, variant.Currency)
}

// getAsinListFromContext 从上下文中获取ASIN列表
func (h *VariantJsonDataHandler) getAsinListFromContext(ctx *pipeline.TaskContext) []string {
	// 尝试从不同的数据源获取ASIN列表

	// 1. 从AsinSkuMap中获取
	if asinSkuMapData, exists := ctx.GetData("AsinSkuMap"); exists {
		if asinSkuMap, ok := asinSkuMapData.(map[string]string); ok {
			return h.getAsinListFromMap(asinSkuMap)
		}
	}

	// 2. 从Amazon产品的变体中获取
	if ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Variations) > 0 {
		asins := make([]string, 0, len(ctx.AmazonProduct.Variations))
		for _, variation := range ctx.AmazonProduct.Variations {
			if variation.Asin != "" {
				asins = append(asins, variation.Asin)
			}
		}
		return asins
	}

	// 3. 从其他可能的数据源获取
	if variantAsinsData, exists := ctx.GetData("VariantAsins"); exists {
		if variantAsins, ok := variantAsinsData.([]string); ok {
			return variantAsins
		}
	}

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

// shouldUseAmazonCrawler 判断是否应该使用Amazon爬虫
func (h *VariantJsonDataHandler) shouldUseAmazonCrawler(ctx *pipeline.TaskContext) bool {
	if h.amazonProcessor == nil || h.amazonConfig == nil || !h.amazonConfig.Enabled {
		return false
	}

	platform := strings.ToLower(ctx.Task.Platform)
	return platform == "amazon"
}

// fetchVariantsBatchFromAmazonCrawler 批量抓取变体数据
func (h *VariantJsonDataHandler) fetchVariantsBatchFromAmazonCrawler(ctx *pipeline.TaskContext, asins []string) []*amazon.Product {
	if h.amazonProcessor == nil {
		h.logger.Error("Amazon爬虫未初始化")
		return []*amazon.Product{}
	}

	// 根据地区获取邮编
	zipcode := h.getZipcodeForRegion(ctx.Task.Region)

	// 根据地区获取正确的Amazon域名
	domain := h.getAmazonDomainByRegion(ctx.Task.Region)

	// 准备批量请求
	requests := make([]amazon.ProductRequest, 0, len(asins))
	for _, asin := range asins {
		url := fmt.Sprintf("https://www.%s/dp/%s", domain, asin)
		requests = append(requests, amazon.ProductRequest{
			URL:     url,
			Zipcode: zipcode,
		})
	}

	h.logger.Infof("开始批量抓取 %d 个变体: Region=%s, Domain=%s, Zipcode=%s",
		len(requests), ctx.Task.Region, domain, zipcode)

	// 批量处理
	results := h.amazonProcessor.ProcessBatch(requests)

	// 转换结果
	variants := make([]*amazon.Product, 0, len(results))
	successCount := 0
	for i, result := range results {
		if result.Error != nil {
			h.logger.Warnf("变体 %s 抓取失败: %v", asins[i], result.Error)
			continue
		}

		if result.Product != nil {
			variants = append(variants, result.Product)
			successCount++
		}
	}

	h.logger.Infof("批量抓取完成: 成功 %d/%d", successCount, len(asins))

	return variants
}

// parseAmazonProduct 解析Amazon产品JSON数据
func (h *VariantJsonDataHandler) parseAmazonProduct(jsonData string) (*amazon.Product, error) {
	var product amazon.Product
	if err := json.Unmarshal([]byte(jsonData), &product); err != nil {
		return nil, fmt.Errorf("解析Amazon产品数据失败: %w", err)
	}
	return &product, nil
}

// getZipcodeForRegion 根据地区获取邮编
func (h *VariantJsonDataHandler) getZipcodeForRegion(region string) string {
	if h.amazonConfig != nil && h.amazonConfig.Zipcodes != nil {
		if zipcode, exists := h.amazonConfig.Zipcodes[region]; exists {
			return zipcode
		}
	}

	// 默认邮编映射
	defaultZipcodes := map[string]string{
		"US": "10001",    // New York
		"UK": "SW1A 1AA", // London
		"DE": "10115",    // Berlin
		"FR": "75001",    // Paris
		"IT": "00118",    // Rome
		"ES": "28001",    // Madrid
		"JP": "100-0001", // Tokyo
		"CA": "K1A 0A6",  // Ottawa
		"AU": "2000",     // Sydney
	}

	if zipcode, exists := defaultZipcodes[region]; exists {
		return zipcode
	}

	return "10001" // 默认使用美国邮编
}

// getAmazonDomainByRegion 根据地区获取Amazon域名
func (h *VariantJsonDataHandler) getAmazonDomainByRegion(region string) string {
	domainMap := map[string]string{
		"US": "amazon.com",
		"UK": "amazon.co.uk",
		"DE": "amazon.de",
		"FR": "amazon.fr",
		"IT": "amazon.it",
		"ES": "amazon.es",
		"JP": "amazon.co.jp",
		"CA": "amazon.ca",
		"AU": "amazon.com.au",
	}

	if domain, exists := domainMap[region]; exists {
		return domain
	}

	return "amazon.com" // 默认使用美国站点
}

// GetVariantByAsinFromVariants 通过ASIN从变体列表中获取变体
func (h *VariantJsonDataHandler) GetVariantByAsinFromVariants(variants []*amazon.Product, asin string) *amazon.Product {
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
	if h.amazonProcessor != nil {
		h.logger.Info("关闭变体Amazon爬虫处理器...")
		h.amazonProcessor.Shutdown()
	}
}
