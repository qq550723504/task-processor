package product

import (
	"fmt"
	"strings"

	"task-processor/internal/model"
	"task-processor/internal/infra/clients/management/api"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/skugen"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/handlerbase"
	"task-processor/internal/temu/property"
	"task-processor/internal/temu/rules"
	"task-processor/internal/temu/template"

	"github.com/sirupsen/logrus"
)

// 辅助函数别名
var GetTemplateInfoFromContext = template.GetTemplateInfoFromContext

// =============================================================================
// 核心构建器
// =============================================================================

// SpuBuilder SPU构建器
type SpuBuilder struct {
	logger         *logrus.Entry
	textProcessor  *rules.TextProcessor
	priceHandler   *PriceHandler
	skuBuilder     handlerbase.SkuBuilder
	specHandler    handlerbase.SpecHandler
	propertyMapper *property.PropertyMapper
	openaiClient   *openaiClient.Client
}

// NewSpuBuilder 创建新的SPU构建器
func NewSpuBuilder(logger *logrus.Entry, openaiConfig *openaiClient.ClientConfig, profitRuleClient api.ProfitRuleAPI, skuBuilder handlerbase.SkuBuilder, specHandler handlerbase.SpecHandler) *SpuBuilder {
	var aiClient *openaiClient.Client
	if openaiConfig != nil {
		aiClient = openaiClient.NewClient(openaiConfig)
	}

	return &SpuBuilder{
		logger:         logger,
		textProcessor:  rules.NewTextProcessor(),
		priceHandler:   NewPriceHandler(profitRuleClient),
		skuBuilder:     skuBuilder,
		specHandler:    specHandler,
		propertyMapper: property.NewPropertyMapper(logger),
		openaiClient:   aiClient,
	}
}

// =============================================================================
// 主要构建方法
// =============================================================================

// BuildBasicInfo 构建基本信息
func (b *SpuBuilder) BuildBasicInfo(temuCtx *temucontext.TemuTaskContext, temuProduct *models.Product) error {
	b.logger.Info("构建产品基本信息")

	basic := &temuProduct.GoodsBasic

	// 获取Amazon产品信息
	amazonProduct := temuCtx.GetAmazonProduct()

	// 设置商品名称
	b.setProductName(amazonProduct, basic)

	// 设置外部商品编号
	b.setOutGoodsSN(temuCtx, amazonProduct, basic)

	// 设置地区和语言信息
	basic.Lang = "en"
	basic.AllowSite = []int{100}

	// 记录品牌信息
	if amazonProduct != nil && amazonProduct.Brand != "" {
		b.logger.Infof("Amazon品牌信息: %s", amazonProduct.Brand)
	}

	b.logger.Info("产品基本信息构建完成")
	return nil
}

// BuildExtensionInfo 构建扩展信息
func (b *SpuBuilder) BuildExtensionInfo(temuCtx *temucontext.TemuTaskContext, temuProduct *models.Product) error {
	b.logger.Info("构建产品扩展信息")

	ext := &temuProduct.GoodsExtensionInfo

	// 构建商品属性（使用AI智能映射）
	if err := b.propertyMapper.BuildGoodsProperties(temuCtx, ext); err != nil {
		b.logger.WithError(err).Warn("构建商品属性失败，使用默认属性")
	}

	// 从强类型上下文获取Amazon产品信息
	amazonProduct := temuCtx.GetAmazonProduct()

	// 设置产品描述
	b.setProductDescription(amazonProduct, ext)

	// 设置要点描述
	b.setBulletPoints(amazonProduct, ext)

	// 设置原产地信息
	if ext.GoodsOriginInfo.OriginRegionName1 == "" {
		ext.GoodsOriginInfo.OriginRegionName1 = "United States"
	}

	b.logger.Info("产品扩展信息构建完成")
	return nil
}

// BuildSkcAndSku 构建SKC和SKU
func (b *SpuBuilder) BuildSkcAndSku(temuCtx *temucontext.TemuTaskContext, temuProduct *models.Product) error {
	b.logger.Info("构建SKC和SKU信息")

	// 创建SKC列表
	if len(temuProduct.SkcList) == 0 {
		// 创建SKC
		if err := b.createSkcList(temuCtx, temuProduct); err != nil {
			return fmt.Errorf("创建SKC列表失败: %w", err)
		}
	} else {
		b.logger.Infof("使用现有的 %d 个SKC", len(temuProduct.SkcList))
	}

	// 处理每个SKC
	for i := range temuProduct.SkcList {
		if err := b.skuBuilder.ProcessSkcItem(temuCtx, i); err != nil {
			return fmt.Errorf("处理SKC[%d]失败: %w", i, err)
		}
	}

	// 构建商品规格属性（基于SKU中使用的规格）
	if err := b.buildGoodsSpecProperties(temuCtx, temuProduct); err != nil {
		b.logger.WithError(err).Warn("构建商品规格属性失败")
	}

	b.logger.Infof("SKC和SKU构建完成: %d个SKC",
		len(temuProduct.SkcList))
	return nil
}

// BuildServicePromise 构建服务承诺
func (b *SpuBuilder) BuildServicePromise(ctx pipeline.TaskContext, temuProduct *models.Product) error {
	b.logger.Info("构建服务承诺信息")

	// 验证CostTemplateID是否已设置
	if temuProduct.GoodsServicePromise.CostTemplateID == "" {
		b.logger.Warn("运费模板ID未设置，这可能导致后续处理失败")
	} else {
		b.logger.Infof("运费模板ID已设置: %s", temuProduct.GoodsServicePromise.CostTemplateID)
	}

	// 只设置其他必要的服务承诺字段
	temuProduct.GoodsServicePromise.ShipmentLimitSecond = 2 // 2天发货
	temuProduct.GoodsServicePromise.FulfillmentType = 1     // 自发货

	b.logger.Info("服务承诺信息构建完成")
	return nil
}

// BuildSaleInfo 构建销售信息
func (b *SpuBuilder) BuildSaleInfo(temuCtx *temucontext.TemuTaskContext, temuProduct *models.Product) error {
	b.logger.Info("验证销售信息")

	// 验证销售信息是否已在初始化时设置
	if temuProduct.GoodsSaleInfo.GoodsPattern == 0 {
		b.logger.Warn("商品模式未设置，使用默认值")
		temuProduct.GoodsSaleInfo.GoodsPattern = 0
	} else {
		b.logger.Infof("商品模式已设置: %d", temuProduct.GoodsSaleInfo.GoodsPattern)
	}

	b.logger.Info("销售信息验证完成")
	return nil
}

// =============================================================================
// 辅助构建方法
// =============================================================================

// getAmazonProductFromContext 从上下文获取Amazon产品信息
func (b *SpuBuilder) getAmazonProductFromContext(ctx pipeline.TaskContext) *model.Product {
	if amazonCtx, ok := ctx.(pipeline.AmazonContext); ok {
		return amazonCtx.GetAmazonProduct()
	}
	b.logger.Warn("无法从上下文获取Amazon产品信息")
	return nil
}

// setProductName 设置商品名称
func (b *SpuBuilder) setProductName(amazonProduct *model.Product, basic *models.GoodsBasicInfo) {
	if basic.GoodsName == "" && amazonProduct != nil {
		basic.GoodsName = b.textProcessor.ProcessProductTitle(amazonProduct.Title)
		b.logger.Infof("从Amazon设置商品名称: %s", basic.GoodsName)
	}
}

// setOutGoodsSN 设置外部商品编号
func (b *SpuBuilder) setOutGoodsSN(temuCtx *temucontext.TemuTaskContext, amazonProduct *model.Product, basic *models.GoodsBasicInfo) {
	if basic.OutGoodsSN != "" || amazonProduct == nil {
		return
	}

	strategy := skugen.StrategyASINOnly
	prefix := ""
	suffix := ""

	// 从强类型上下文中获取店铺信息
	if temuCtx.StoreInfo != nil {
		storeInfo := temuCtx.StoreInfo
		switch storeInfo.SkuGenerateStrategy {
		case "asin_only":
			strategy = skugen.StrategyASINOnly
		case "random":
			strategy = skugen.StrategyRandom
		case "timestamp":
			strategy = skugen.StrategyTimestamp
		case "hash":
			strategy = skugen.StrategyHash
		default:
			strategy = skugen.StrategyASINOnly
		}

		prefix = storeInfo.Prefix
		suffix = storeInfo.Suffix

		b.logger.Infof("使用店铺SKU配置: 策略=%s, 前缀=%s, 后缀=%s",
			storeInfo.SkuGenerateStrategy, prefix, suffix)
	} else {
		b.logger.Warn("店铺信息为空，使用默认SKU生成配置")
	}

	basic.OutGoodsSN = skugen.Generate(amazonProduct.Asin, strategy, prefix, suffix)
	b.logger.Infof("设置外部商品编号: %s (基于ASIN: %s)", basic.OutGoodsSN, amazonProduct.Asin)
}

// setProductDescription 设置产品描述
func (b *SpuBuilder) setProductDescription(amazonProduct *model.Product, ext *models.ExtensionInfo) {
	if ext.GoodsDesc != "" {
		return
	}

	if amazonProduct != nil && amazonProduct.Title != "" {
		ext.GoodsDesc = b.textProcessor.ProcessDescription(amazonProduct.Title)
		b.logger.Infof("从Amazon设置产品描述，长度: %d", len(ext.GoodsDesc))
	} else {
		ext.GoodsDesc = "High quality product with excellent features."
	}
}

// setBulletPoints 设置要点描述
func (b *SpuBuilder) setBulletPoints(amazonProduct *model.Product, ext *models.ExtensionInfo) {
	if len(ext.BulletPoints) > 0 {
		return
	}

	if amazonProduct != nil && len(amazonProduct.Features) > 0 {
		ext.BulletPoints = b.textProcessor.ProcessBulletPoints(amazonProduct.Features)
		b.logger.Infof("从Amazon设置要点描述，数量: %d", len(ext.BulletPoints))
	} else {
		ext.BulletPoints = b.textProcessor.GetDefaultBulletPoints()
		b.logger.Infof("设置默认要点描述，数量: %d", len(ext.BulletPoints))
	}
}

// createSkcList 创建SKC列表
func (b *SpuBuilder) createSkcList(temuCtx *temucontext.TemuTaskContext, temuProduct *models.Product) error {
	variants := temuCtx.Variants
	if len(variants) > 0 {
		// 有变体，创建变体SKC
		b.logger.Infof("发现 %d 个变体数据，尝试创建变体SKC", len(variants))
		if err := b.skuBuilder.BuildVariantSkcs(temuCtx, variants); err != nil {
			b.logger.Errorf("❌ 创建变体SKC失败: %v", err)
			return fmt.Errorf("创建变体SKC失败: %w", err)
		}
		return nil
	}

	// 没有变体，创建默认SKC（从模板中选择规格）
	b.logger.Info("没有找到Amazon变体数据，创建默认SKC")
	skc, err := b.skuBuilder.CreateDefaultSkc(temuCtx)
	if err != nil {
		b.logger.Errorf("❌ 创建默认SKC失败: %v", err)
		return fmt.Errorf("创建默认SKC失败: %w", err)
	}

	// 将创建的SKC添加到产品中
	temuProduct.SkcList = []models.Skc{skc}

	b.logger.Info("✅ 成功创建默认SKC")
	return nil
}

// buildGoodsSpecProperties 构建商品规格属性（基于SKU中使用的规格）
func (b *SpuBuilder) buildGoodsSpecProperties(temuCtx *temucontext.TemuTaskContext, temuProduct *models.Product) error {
	b.logger.Info("开始构建商品规格属性")

	// 收集所有SKU中使用的规格
	specMap := make(map[string]*models.GoodSpecProperty) // key: parent_spec_id_spec_id

	// 尝试获取模板信息以获取vid值
	var templateSpecMap map[string]map[string]int // parent_spec_id -> spec_id -> vid
	if temuCtx != nil {
		templateSpecMap = b.buildTemplateSpecMap(temuCtx)
	}

	for _, skc := range temuProduct.SkcList {
		for _, sku := range skc.SkuList {
			for _, spec := range sku.Spec {
				// 检查是否还有临时ID（不应该出现）
				if strings.HasPrefix(spec.SpecID, "TEMP_") {
					b.logger.Errorf("❌ 发现未解析的临时规格ID: %s (parent: %s, name: %s)",
						spec.SpecID, spec.ParentSpecID, spec.SpecName)
					return fmt.Errorf("发现未解析的临时规格ID: %s，请检查规格解析逻辑", spec.SpecID)
				}

				// 🔧 新增：规格值标准化处理
				standardizedSpecName := b.standardizeSpecValue(spec.SpecName, spec.ParentSpecName)
				if standardizedSpecName != spec.SpecName {
					b.logger.Infof("🔧 规格值标准化: %s → %s (规格: %s)", spec.SpecName, standardizedSpecName, spec.ParentSpecName)
					spec.SpecName = standardizedSpecName
				}

				key := fmt.Sprintf("%s_%s", spec.ParentSpecID, spec.SpecID)
				if _, exists := specMap[key]; !exists {
					// 根据规格类型设置feature值
					feature := 0
					if b.specHandler.IsSizeSpec(strings.ToLower(spec.ParentSpecName)) {
						feature = 2 // 尺码规格
					}

					// 尝试从模板中获取vid值和模板信息
					vid := 0
					templateModuleID := 0
					templatePid := 0
					if templateSpecMap != nil {
						if specMap, exists := templateSpecMap[spec.ParentSpecID]; exists {
							if v, exists := specMap[spec.SpecID]; exists {
								vid = v
								b.logger.Debugf("从模板获取vid: %s_%s -> %d", spec.ParentSpecID, spec.SpecID, vid)
							}
						}
					}

					// 从模板信息中获取TemplateModuleID和TemplatePid
					if templateInfo, exists := GetTemplateInfoFromContext(temuCtx); exists {
						for _, templateSpec := range templateInfo.GoodsSpecProperties {
							if templateSpec.ParentSpecID == spec.ParentSpecID {
								templateModuleID = templateSpec.TemplateModuleID
								templatePid = templateSpec.TemplatePID
								b.logger.Debugf("从模板获取规格模板信息: %s -> TemplateModuleID=%d, TemplatePid=%d",
									spec.ParentSpecName, templateModuleID, templatePid)
								break
							}
						}
					}

					specMap[key] = &models.GoodSpecProperty{
						Value:            spec.SpecName,
						SpecID:           spec.SpecID,
						ParentSpecID:     spec.ParentSpecID,
						ParentSpecName:   spec.ParentSpecName,
						Feature:          feature,
						Checked:          true, // 修复：实际使用的规格应该设置为true
						ControlType:      0,
						Disabled:         false,
						Name:             spec.ParentSpecName,
						IsCustomized:     1,                // 1表示用户自定义规格
						Vid:              vid,              // 设置从模板获取的vid值
						TemplateModuleID: templateModuleID, // 设置模板模块ID
						TemplatePid:      templatePid,      // 设置模板PID
					}
				}
			}
		}
	}

	// 转换为切片
	var goodsSpecProperties []models.GoodSpecProperty
	for _, specProp := range specMap {
		goodsSpecProperties = append(goodsSpecProperties, *specProp)
	}

	// 设置到产品扩展信息中
	temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties = goodsSpecProperties

	b.logger.Infof("商品规格属性构建完成，共%d个规格", len(goodsSpecProperties))
	return nil
}

// standardizeSpecValue 标准化规格值
func (b *SpuBuilder) standardizeSpecValue(specValue, parentSpecName string) string {
	// 针对"Number Of Products"规格进行特殊处理
	if strings.ToLower(parentSpecName) == "number of products" {
		// 移除"pcs"后缀，统一为纯数字格式
		if strings.HasSuffix(strings.ToLower(specValue), "pcs") {
			return strings.TrimSuffix(specValue, "pcs")
		}
	}

	return specValue
}

// buildTemplateSpecMap 从模板信息中构建规格映射 (parent_spec_id -> spec_id -> vid)
func (b *SpuBuilder) buildTemplateSpecMap(temuCtx *temucontext.TemuTaskContext) map[string]map[string]int {
	templateSpecMap := make(map[string]map[string]int)

	// 尝试从上下文获取模板信息
	if templateInfo, exists := template.GetTemplateInfoFromContext(temuCtx); exists {
		for _, specProp := range templateInfo.GoodsSpecProperties {
			if specProp.ParentSpecID != "" && len(specProp.Values) > 0 {
				if templateSpecMap[specProp.ParentSpecID] == nil {
					templateSpecMap[specProp.ParentSpecID] = make(map[string]int)
				}

				// 遍历所有可能的规格值，建立spec_id到vid的映射
				for _, value := range specProp.Values {
					if value.SpecID != "" {
						templateSpecMap[specProp.ParentSpecID][value.SpecID] = value.VID
						b.logger.Debugf("添加规格映射: %s_%s -> vid:%d (value:%s)",
							specProp.ParentSpecID, value.SpecID, value.VID, value.Value)
					}
				}
			}
		}
		b.logger.Infof("构建模板规格映射完成，共%d个父规格", len(templateSpecMap))
	} else {
		b.logger.Warn("未找到模板信息，无法获取vid值")
	}

	return templateSpecMap
}

