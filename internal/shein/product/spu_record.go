package product

import (
	"task-processor/internal/core/logger"
	"fmt"
	"task-processor/internal/pkg/skugen"
	"task-processor/internal/shein"
	"task-processor/internal/shein/api/product"

)

// HasSpuRecordHandler 检查SPU发布记录处理器
type HasSpuRecordHandler struct {
}

// NewHasSpuRecordHandler 创建新的检查SPU发布记录处理器
func NewHasSpuRecordHandler() *HasSpuRecordHandler {
	return &HasSpuRecordHandler{}
}

// Name 返回处理器名称
func (h *HasSpuRecordHandler) Name() string {
	return "检查SPU发布记录"
}

// Handle 执行检查SPU发布记录处理
func (h *HasSpuRecordHandler) Handle(ctx *shein.TaskContext) error {

	// 提取ASIN列表
	asins := h.extractAsinsFromContext(ctx)

	// 生成SKU列表
	skuList := h.generateSkuList(asins, ctx)

	// 存储Asin与SKU的对应关系
	h.storeAsinSkuMap(ctx, asins, skuList)

	// SHEIN API 限制：供方货号最多支持1000条数据，需要分批查询
	const batchSize = 1000
	totalBatches := (len(skuList) + batchSize - 1) / batchSize

	logger.GetGlobalLogger("shein/product").Infof("开始检查SPU发布记录，共 %d 个SKU，分 %d 批处理", len(skuList), totalBatches)

	// 分批查询
	for i := 0; i < len(skuList); i += batchSize {
		end := i + batchSize
		if end > len(skuList) {
			end = len(skuList)
		}

		batchSkuList := skuList[i:end]
		batchNum := i/batchSize + 1

		logger.GetGlobalLogger("shein/product").Infof("处理第 %d/%d 批，SKU数量: %d", batchNum, totalBatches, len(batchSkuList))

		// 构建请求参数
		request := &product.ProductRecordRequest{
			Language:                  "en",
			OnlyCurrentMonthRecommend: false,
			OnlySpmbCopyProduct:       false,
			QueryTimeOut:              false,
			SearchDiyCustom:           false,
			SupplierCodeList:          &batchSkuList,
			SupplierCodeSearchType:    1,
		}

		// 调用API检查SPU发布记录
		response, err := ctx.ProductAPI.Record(request)
		if err != nil {
			return fmt.Errorf("检查SPU发布记录失败 (批次 %d/%d): %w", batchNum, totalBatches, err)
		}

		// 检查API调用是否成功
		if response.Code != "0" {
			return fmt.Errorf("检查SPU发布记录失败 (批次 %d/%d): %s", batchNum, totalBatches, response.Msg)
		}

		// 检查是否已存在发布记录
		if len(response.Info.Data) > 0 {
			// 记录警告信息，并返回不可重试错误以终止任务
			logger.GetGlobalLogger("shein/product").Warnf("检测到已存在发布记录 (批次 %d/%d)，任务将被终止: %v", batchNum, totalBatches, response.Info.Data)
			// 返回不可重试错误，终止任务且不重试
			return shein.NewNonRetryableError("已存在发布记录", nil)
		}
	}

	logger.GetGlobalLogger("shein/product").Infof("SPU发布记录检查完成，未发现重复记录")
	return nil
}

// storeAsinSkuMap 存储ASIN与SKU的对应关系
func (h *HasSpuRecordHandler) storeAsinSkuMap(ctx *shein.TaskContext, asins []string, skus []string) {
	// 初始化映射
	if ctx.AsinSkuMap == nil {
		ctx.AsinSkuMap = make(map[string]string)
	}

	// 建立ASIN到SKU的映射
	for i, asin := range asins {
		if i < len(skus) {
			ctx.AsinSkuMap[asin] = skus[i]
		}
	}
}

// extractAsinsFromContext 从任务上下文中提取ASIN列表
func (h *HasSpuRecordHandler) extractAsinsFromContext(ctx *shein.TaskContext) []string {
	asins := []string{}

	// 添加主产品ID
	if ctx.Task != nil && ctx.Task.ProductID != "" {
		asins = append(asins, ctx.Task.ProductID)
	}

	// 从AmazonProduct的Variations中提取ASIN
	if ctx.AmazonProduct != nil {
		for _, variant := range ctx.AmazonProduct.Variations {
			// 直接访问ASIN字段
			if variant.Asin != "" {
				// 避免重复添加
				found := false
				for _, existingAsin := range asins {
					if existingAsin == variant.Asin {
						found = true
						break
					}
				}
				if !found {
					asins = append(asins, variant.Asin)
				}
			}
		}
	}

	return asins
}

// generateSkuList 根据店铺配置生成SKU列表
func (h *HasSpuRecordHandler) generateSkuList(asins []string, ctx *shein.TaskContext) []string {
	skuList := make([]string, 0, len(asins))

	// 获取店铺配置
	skuStrategy := 0 // 默认策略
	prefix := ""
	suffix := ""

	if ctx.StoreInfo != nil {
		// 解析SKU生成策略
		switch ctx.StoreInfo.SkuGenerateStrategy {
		case "ASIN_ONLY", "0":
			skuStrategy = skugen.StrategyASINOnly
		case "RANDOM", "1":
			skuStrategy = skugen.StrategyRandom
		case "TIMESTAMP", "2":
			skuStrategy = skugen.StrategyTimestamp
		case "HASH", "3":
			skuStrategy = skugen.StrategyHash
		default:
			// 尝试解析为数字
			var strategyNum int
			if _, err := fmt.Sscanf(ctx.StoreInfo.SkuGenerateStrategy, "%d", &strategyNum); err == nil {
				skuStrategy = strategyNum
			}
		}

		prefix = ctx.StoreInfo.Prefix
		suffix = ctx.StoreInfo.Suffix
	}

	// 为每个ASIN生成SKU
	for _, asin := range asins {
		sku := skugen.Generate(asin, skuStrategy, prefix, suffix)
		skuList = append(skuList, sku)
	}

	return skuList
}

// GetSkuByAsin 根据ASIN获取对应的SKU
func GetSkuByAsin(ctx *shein.TaskContext, asin string) string {
	// 检查上下文和映射是否存在
	if ctx == nil || ctx.AsinSkuMap == nil {
		return ""
	}

	// 直接从映射中查找SKU
	if sku, exists := ctx.AsinSkuMap[asin]; exists {
		return sku
	}

	return ""
}

// GetAsinBySku 根据SKU获取对应的ASIN
func GetAsinBySku(ctx *shein.TaskContext, sku string) string {
	// 检查上下文和映射是否存在
	if ctx == nil || ctx.AsinSkuMap == nil {
		return ""
	}

	// 遍历映射查找匹配的ASIN
	for asin, s := range ctx.AsinSkuMap {
		if s == sku {
			return asin
		}
	}

	return ""
}
