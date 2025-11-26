package modules

import (
	"fmt"
	"task-processor/common/shein/api/product"

	"github.com/sirupsen/logrus"
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
func (h *HasSpuRecordHandler) Handle(ctx *TaskContext) error {

	// 提取ASIN列表
	asins := h.extractAsinsFromContext(ctx)

	// 生成SKU列表
	skuList := h.generateSkuList(asins, ctx)

	// 存储Asin与SKU的对应关系
	h.storeAsinSkuMap(ctx, asins, skuList)

	// 构建请求参数
	request := &product.ProductRecordRequest{
		Language:                  "en",
		OnlyCurrentMonthRecommend: false,
		OnlySpmbCopyProduct:       false,
		QueryTimeOut:              false,
		SearchDiyCustom:           false,
		SupplierCodeList:          &skuList,
		SupplierCodeSearchType:    1,
	}

	// 调用API检查SPU发布记录
	response, err := ctx.ShopClient.Record(request)
	if err != nil {
		return fmt.Errorf("检查SPU发布记录失败: %w", err)
	}

	// 检查API调用是否成功
	if response.Code != "0" {
		return fmt.Errorf("检查SPU发布记录失败: %s", response.Msg)
	}

	// 检查是否已存在发布记录
	if len(response.Info.Data) > 0 {
		// 记录警告信息，并返回不可重试错误以终止任务
		logrus.Warnf("检测到已存在发布记录，任务将被终止: %v", response.Info.Data)
		// 返回不可重试错误，终止任务且不重试
		return NewNonRetryableError("已存在发布记录", nil)
	}

	return nil
}

// storeAsinSkuMap 存储ASIN与SKU的对应关系
func (h *HasSpuRecordHandler) storeAsinSkuMap(ctx *TaskContext, asins []string, skus []string) {
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
func (h *HasSpuRecordHandler) extractAsinsFromContext(ctx *TaskContext) []string {
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
func (h *HasSpuRecordHandler) generateSkuList(asins []string, ctx *TaskContext) []string {
	skuList := make([]string, 0, len(asins))

	// 获取店铺配置
	skuStrategy := 0 // 默认策略
	prefix := ""
	suffix := ""

	if ctx.StoreInfo != nil {
		// 解析SKU生成策略
		switch ctx.StoreInfo.SkuGenerateStrategy {
		case "ASIN_ONLY", "0":
			skuStrategy = 0
		case "RANDOM", "1":
			skuStrategy = 1
		case "TIMESTAMP", "2":
			skuStrategy = 2
		case "HASH", "3":
			skuStrategy = 3
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
		sku := GenerateSKU(asin, skuStrategy, prefix, suffix)
		skuList = append(skuList, sku)
	}

	return skuList
}

// GetSkuByAsin 根据ASIN获取对应的SKU
func GetSkuByAsin(ctx *TaskContext, asin string) string {
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
func GetAsinBySku(ctx *TaskContext, sku string) string {
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
