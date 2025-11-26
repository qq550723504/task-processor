package handlers

import (
	"fmt"

	"task-processor/common/amazon"
	"task-processor/common/pipeline"
	"task-processor/common/utils"
	"task-processor/platforms/temu/types"
)

// buildVariantSkcsDefault 默认变体SKC构建（备用方案）
func (sb *SkuBuilder) buildVariantSkcsDefault(ctx *pipeline.TaskContext, variants []*amazon.Product) error {
	sb.logger.Warn("⚠️ 使用默认SKC构建器（备用方案）")
	sb.logger.Warn("⚠️ 注意：默认构建器不生成规格信息，可能导致TEMU提交失败")

	var skcList []types.Skc

	for i, variant := range variants {
		skc := types.Skc{
			//CarouselGallery: sb.imageProcessor.BuildVariantImagesWithUpload(ctx, variant),
			SkuList: []types.Sku{func() types.Sku {
				var _ int = i
				return sb.buildSkuFromVariant(ctx, variant)
			}()},
		}

		skcList = append(skcList, skc)
	}

	ctx.TemuProduct.SkcList = skcList

	// 默认构建器不处理spec解析，因为没有足够的信息
	// 如果需要spec，应该使用AI生成正确的映射
	return nil
}

// buildSkuFromVariant 从变体构建SKU（默认方式）
func (sb *SkuBuilder) buildSkuFromVariant(ctx *pipeline.TaskContext, variant *amazon.Product) types.Sku {
	supplierPrice := sb.priceHandler.CalculateVariantPrice(ctx, variant)

	// 使用SKU生成器生成OutSkuSN，使用variant的ASIN作为基础
	asin := variant.Asin

	// 根据店铺配置生成SKU
	outSkuSN := sb.generateSkuFromStoreConfig(ctx, asin)

	// 保存ASIN到SKU的映射关系到上下文，供后续SavePublishResultHandler使用
	sb.saveAsinSkuMapping(ctx, outSkuSN, asin)

	// 从店铺配置读取库存设置（使用统一的方法）
	quantity := sb.priceHandler.GetDefaultStock(ctx)

	// 计算市场价：供货价 * 2
	marketPrice := supplierPrice * 2                                    // 市场价（分）
	marketPriceStr := fmt.Sprintf("%.2f", float64(supplierPrice)*2/100) // 市场价字符串（元）

	sku := types.Sku{}
	sku.Currency = sb.regionHandler.GetCurrencyByRegion(ctx.Task.Region)
	sku.OutSkuSN = outSkuSN
	//sku.Price = finalPrice
	sku.Quantity = fmt.Sprintf("%d", quantity)
	sku.SupplierPrice = supplierPrice
	sku.MarketPrice = marketPrice       // 市场价（分），供货价*2
	sku.MarketPriceStr = marketPriceStr // 市场价字符串（元），供货价*2
	sku.UseEstimateSupplierPrice = false
	sku.CarouselGallery = sb.imageProcessor.BuildVariantImagesWithUpload(ctx, variant)

	// 注意：默认构建器不生成spec，因为没有足够的信息
	// spec应该由AI生成或从TEMU模板中获取
	// 这里留空，让后续的验证逻辑检测到问题
	sb.logger.Warn("⚠️ 默认构建器无法生成spec，SKU将没有规格信息（可能导致提交失败）")
	sb.logger.Warn("⚠️ 建议：修复AI映射生成逻辑，确保生成正确数量的SKU映射")

	return sku
}

// generateSkuFromStoreConfig 根据店铺配置生成SKU
func (sb *SkuBuilder) generateSkuFromStoreConfig(ctx *pipeline.TaskContext, asin string) string {
	// 默认策略和参数
	strategy := utils.StrategyASINOnly
	prefix := ""
	suffix := ""

	// 如果有店铺配置，使用店铺配置的策略
	if ctx.StoreInfo != nil {
		// 设置前缀和后缀
		prefix = ctx.StoreInfo.Prefix
		suffix = ctx.StoreInfo.Suffix

		// 根据店铺配置的策略字符串转换为对应的常量
		switch ctx.StoreInfo.SkuGenerateStrategy {
		case "asin_only":
			strategy = utils.StrategyASINOnly
		case "random":
			strategy = utils.StrategyRandom
		case "timestamp":
			strategy = utils.StrategyTimestamp
		case "hash":
			strategy = utils.StrategyHash
		default:
			strategy = utils.StrategyASINOnly
		}

		sb.logger.Debugf("使用店铺配置生成SKU: 策略=%s, 前缀=%s, 后缀=%s",
			ctx.StoreInfo.SkuGenerateStrategy, prefix, suffix)
	} else {
		sb.logger.Debug("未找到店铺配置，使用默认SKU生成策略")
	}

	return utils.GenerateSKU(asin, strategy, prefix, suffix)
}

// saveAsinSkuMapping 保存ASIN到SKU的映射关系到上下文
func (sb *SkuBuilder) saveAsinSkuMapping(ctx *pipeline.TaskContext, outSkuSN string, asin string) {
	// 获取或创建映射表
	var asinSkuMap map[string]string
	if mapData, exists := ctx.GetData("asin_sku_map"); exists {
		if existingMap, ok := mapData.(map[string]string); ok {
			asinSkuMap = existingMap
		} else {
			sb.logger.Warn("asin_sku_map类型不正确，重新创建")
			asinSkuMap = make(map[string]string)
		}
	} else {
		asinSkuMap = make(map[string]string)
	}

	// 保存映射关系：SKU -> ASIN
	asinSkuMap[outSkuSN] = asin
	ctx.SetData("asin_sku_map", asinSkuMap)

	sb.logger.Debugf("保存ASIN到SKU映射: SKU=%s -> ASIN=%s", outSkuSN, asin)
}
