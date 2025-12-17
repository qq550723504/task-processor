package handlers

import (
	"fmt"
	"strings"

	"task-processor/internal/common/management/api"
	"task-processor/internal/common/pipeline"
	"task-processor/internal/common/utils"
	openaiClient "task-processor/internal/clients/openai"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// =============================================================================
// 核心构建器
// =============================================================================

// SpuBuilder SPU构建器
type SpuBuilder struct {
	logger         *logrus.Entry
	textProcessor  *TextProcessor
	priceHandler   *PriceHandler
	regionHandler  *RegionHandler
	skuBuilder     *SkuBuilder
	propertyMapper *AIPropertyMapper
	openaiClient   *openaiClient.Client
}

// NewSpuBuilder 创建新的SPU构建器
func NewSpuBuilder(logger *logrus.Entry, openaiConfig *openaiClient.ClientConfig, profitRuleClient api.ProfitRuleAPI) *SpuBuilder {
	var aiClient *openaiClient.Client
	if openaiConfig != nil {
		aiClient = openaiClient.NewClient(openaiConfig)
	}

	return &SpuBuilder{
		logger:         logger,
		textProcessor:  NewTextProcessor(),
		priceHandler:   NewPriceHandler(profitRuleClient),
		regionHandler:  NewRegionHandler(),
		skuBuilder:     NewSkuBuilder(logger, aiClient, profitRuleClient),
		propertyMapper: NewAIPropertyMapper(logger, aiClient),
		openaiClient:   aiClient,
	}
}

// =============================================================================
// 主要构建方法
// =============================================================================

// BuildBasicInfo 构建基本信息
func (b *SpuBuilder) BuildBasicInfo(ctx *pipeline.TaskContext) error {
	b.logger.Info("构建产品基本信息")

	basic := &ctx.TemuProduct.GoodsBasic

	// 设置商品名称
	b.setProductName(ctx, basic)

	// 设置外部商品编号
	b.setOutGoodsSN(ctx, basic)

	// 设置地区和语言信息
	basic.Lang = b.regionHandler.GetLanguageByRegion(ctx.Task.Region)
	basic.AllowSite = b.regionHandler.GetAllowSitesByRegion(ctx.Task.Region)

	// 记录品牌信息
	if ctx.AmazonProduct != nil && ctx.AmazonProduct.Brand != "" {
		b.logger.Infof("Amazon品牌信息: %s", ctx.AmazonProduct.Brand)
	}

	b.logger.Info("产品基本信息构建完成")
	return nil
}

// BuildExtensionInfo 构建扩展信息
func (b *SpuBuilder) BuildExtensionInfo(ctx *pipeline.TaskContext) error {
	b.logger.Info("构建产品扩展信息")

	ext := &ctx.TemuProduct.GoodsExtensionInfo

	// 构建商品属性（使用AI智能映射）
	if err := b.propertyMapper.BuildGoodsProperties(ctx, ext); err != nil {
		b.logger.WithError(err).Warn("构建商品属性失败，使用默认属性")
	}

	// 设置产品描述
	b.setProductDescription(ctx, ext)

	// 设置要点描述
	b.setBulletPoints(ctx, ext)

	// 设置原产地信息
	if ext.GoodsOriginInfo.OriginRegionName1 == "" {
		ext.GoodsOriginInfo.OriginRegionName1 = b.regionHandler.GetOriginByRegion(ctx.Task.Region)
	}

	b.logger.Info("产品扩展信息构建完成")
	return nil
}

// BuildSkcAndSku 构建SKC和SKU
func (b *SpuBuilder) BuildSkcAndSku(ctx *pipeline.TaskContext) error {
	b.logger.Info("构建SKC和SKU信息")

	// 创建SKC列表
	if len(ctx.TemuProduct.SkcList) == 0 {
		// 创建SKC
		if err := b.createSkcList(ctx); err != nil {
			return fmt.Errorf("创建SKC列表失败: %w", err)
		}
	} else {
		b.logger.Infof("使用现有的 %d 个SKC", len(ctx.TemuProduct.SkcList))
	}

	// 处理每个SKC
	for i := range ctx.TemuProduct.SkcList {
		if err := b.skuBuilder.ProcessSkcItem(ctx, i); err != nil {
			return fmt.Errorf("处理SKC[%d]失败: %w", i, err)
		}
	}

	// 构建商品规格属性（基于SKU中使用的规格）
	if err := b.buildGoodsSpecProperties(ctx); err != nil {
		b.logger.WithError(err).Warn("构建商品规格属性失败")
	}

	b.logger.Infof("SKC和SKU构建完成: %d个SKC, 总计%d个SKU",
		len(ctx.TemuProduct.SkcList), b.skuBuilder.GetTotalSkuCount(ctx.TemuProduct.SkcList))
	return nil
}

// BuildServicePromise 构建服务承诺
func (b *SpuBuilder) BuildServicePromise(ctx *pipeline.TaskContext) error {
	b.logger.Info("构建服务承诺信息")

	// 验证CostTemplateID是否已设置
	if ctx.TemuProduct.GoodsServicePromise.CostTemplateID == "" {
		b.logger.Warn("运费模板ID未设置，这可能导致后续处理失败")
	} else {
		b.logger.Infof("运费模板ID已设置: %s", ctx.TemuProduct.GoodsServicePromise.CostTemplateID)
	}

	// 只设置其他必要的服务承诺字段
	ctx.TemuProduct.GoodsServicePromise.ShipmentLimitSecond = 2 // 2天发货
	ctx.TemuProduct.GoodsServicePromise.FulfillmentType = 1     // 自发货

	b.logger.Info("服务承诺信息构建完成")
	return nil
}

// BuildSaleInfo 构建销售信息
func (b *SpuBuilder) BuildSaleInfo(ctx *pipeline.TaskContext) error {
	b.logger.Info("验证销售信息")

	// 验证销售信息是否已在初始化时设置
	if ctx.TemuProduct.GoodsSaleInfo.GoodsPattern == 0 {
		b.logger.Warn("商品模式未设置，使用默认值")
		ctx.TemuProduct.GoodsSaleInfo.GoodsPattern = 0 // 修改为11，与成功提交的JSON一致
	} else {
		b.logger.Infof("商品模式已设置: %d", ctx.TemuProduct.GoodsSaleInfo.GoodsPattern)
	}

	b.logger.Info("销售信息验证完成")
	return nil
}

// =============================================================================
// 辅助构建方法
// =============================================================================

// setProductName 设置商品名称
func (b *SpuBuilder) setProductName(ctx *pipeline.TaskContext, basic *types.GoodsBasicInfo) {
	if basic.GoodsName == "" && ctx.AmazonProduct != nil {
		basic.GoodsName = b.textProcessor.ProcessProductTitle(ctx.AmazonProduct.Title)
		b.logger.Infof("从Amazon设置商品名称: %s", basic.GoodsName)
	}
}

// setOutGoodsSN 设置外部商品编号
func (b *SpuBuilder) setOutGoodsSN(ctx *pipeline.TaskContext, basic *types.GoodsBasicInfo) {
	if basic.OutGoodsSN != "" || ctx.AmazonProduct == nil {
		return
	}

	strategy := utils.StrategyASINOnly
	prefix := ""
	suffix := ""

	// 从店铺信息中获取SKU生成配置
	if ctx.StoreInfo != nil {
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

		prefix = ctx.StoreInfo.Prefix
		suffix = ctx.StoreInfo.Suffix

		b.logger.Infof("使用店铺SKU配置: 策略=%s, 前缀=%s, 后缀=%s",
			ctx.StoreInfo.SkuGenerateStrategy, prefix, suffix)
	} else {
		b.logger.Warn("店铺信息为空，使用默认SKU生成配置")
	}

	basic.OutGoodsSN = utils.GenerateSKU(ctx.AmazonProduct.ParentAsin, strategy, prefix, suffix)
	b.logger.Infof("设置外部商品编号: %s (基于ASIN: %s)", basic.OutGoodsSN, ctx.AmazonProduct.Asin)
}

// setProductDescription 设置产品描述
func (b *SpuBuilder) setProductDescription(ctx *pipeline.TaskContext, ext *types.ExtensionInfo) {
	if ext.GoodsDesc != "" {
		return
	}

	if ctx.AmazonProduct != nil && ctx.AmazonProduct.Description != "" {
		ext.GoodsDesc = b.textProcessor.ProcessDescription(ctx.AmazonProduct.Description)
		b.logger.Infof("从Amazon设置产品描述，长度: %d", len(ext.GoodsDesc))
	} else {
		ext.GoodsDesc = "High quality product with excellent features."
	}
}

// setBulletPoints 设置要点描述
func (b *SpuBuilder) setBulletPoints(ctx *pipeline.TaskContext, ext *types.ExtensionInfo) {
	if len(ext.BulletPoints) > 0 {
		return
	}

	if ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Features) > 0 {
		ext.BulletPoints = b.textProcessor.ProcessBulletPoints(ctx.AmazonProduct.Features)
		b.logger.Infof("从Amazon设置要点描述，数量: %d", len(ext.BulletPoints))
	} else {
		ext.BulletPoints = b.textProcessor.GetDefaultBulletPoints()
	}
}

// createSkcList 创建SKC列表
func (b *SpuBuilder) createSkcList(ctx *pipeline.TaskContext) error {
	if variants := ctx.GetAmazonVariants(); len(variants) > 0 {
		// 有变体，创建变体SKC
		b.logger.Infof("发现 %d 个变体数据，尝试创建变体SKC", len(variants))
		if err := b.skuBuilder.BuildVariantSkcs(ctx, variants); err != nil {
			b.logger.Errorf("❌ 创建变体SKC失败: %v", err)
			return fmt.Errorf("创建变体SKC失败: %w", err)
		}
		return nil
	} else {
		// 没有变体，创建默认SKC（从模板中选择规格）
		b.logger.Info("没有找到Amazon变体数据，创建默认SKC")
		skc, err := b.skuBuilder.CreateDefaultSkc(ctx)
		if err != nil {
			b.logger.Errorf("❌ 创建默认SKC失败: %v", err)
			return fmt.Errorf("创建默认SKC失败: %w", err)
		}
		ctx.TemuProduct.SkcList = []types.Skc{skc}
		b.logger.Info("✅ 成功创建默认SKC")
		return nil
	}
}

// buildGoodsSpecProperties 构建商品规格属性（基于SKU中使用的规格）
func (b *SpuBuilder) buildGoodsSpecProperties(ctx *pipeline.TaskContext) error {
	b.logger.Info("开始构建商品规格属性")

	// 收集所有SKU中使用的规格
	specMap := make(map[string]*types.GoodsSpecProperty) // key: parent_spec_id_spec_id

	for _, skc := range ctx.TemuProduct.SkcList {
		for _, sku := range skc.SkuList {
			for _, spec := range sku.Spec {
				// 检查是否还有临时ID（不应该出现）
				if strings.HasPrefix(spec.SpecID, "TEMP_") {
					b.logger.Errorf("❌ 发现未解析的临时规格ID: %s (parent: %s, name: %s)",
						spec.SpecID, spec.ParentSpecID, spec.SpecName)
					b.logger.Error("❌ 这表明resolveTemporarySpecIDs没有正确工作")
					return fmt.Errorf("发现未解析的临时规格ID: %s，请检查规格解析逻辑", spec.SpecID)
				}

				key := fmt.Sprintf("%s_%s", spec.ParentSpecID, spec.SpecID)
				if _, exists := specMap[key]; !exists {
					// 根据规格类型设置feature值
					feature := 0
					if b.skuBuilder.isSizeSpec(strings.ToLower(spec.ParentSpecName)) {
						feature = 2 // 尺码规格
					}

					specMap[key] = &types.GoodsSpecProperty{
						Value:          spec.SpecName,
						SpecID:         spec.SpecID,
						ParentSpecID:   spec.ParentSpecID,
						ParentSpecName: spec.ParentSpecName,
						Feature:        feature,
						Checked:        false,
						ControlType:    0,
						Disabled:       false,
						Name:           spec.ParentSpecName,
						IsCustomized:   1, // 1表示用户自定义规格
					}
				}
			}
		}
	}

	// 转换为切片
	var goodsSpecProperties []types.GoodsSpecProperty
	for _, specProp := range specMap {
		goodsSpecProperties = append(goodsSpecProperties, *specProp)
	}

	// 设置到产品扩展信息中
	ctx.TemuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties = goodsSpecProperties

	b.logger.Infof("商品规格属性构建完成，共%d个规格", len(goodsSpecProperties))
	return nil
}
