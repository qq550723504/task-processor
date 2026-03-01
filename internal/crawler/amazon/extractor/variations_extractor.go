package extractor

import (
	"task-processor/internal/crawler/amazon/variations"
	"task-processor/internal/domain/model"

	"github.com/playwright-community/playwright-go"
)

// VariationsConfig 变体提取配置（向后兼容）
type VariationsConfig = variations.Config

// GetDefaultVariationsConfig 获取默认配置（向后兼容）
func GetDefaultVariationsConfig() *VariationsConfig {
	return variations.GetDefaultConfig()
}

// VariationsExtractor 变体信息提取器（向后兼容）
type VariationsExtractor struct {
	extractor *variations.Extractor
	config    *VariationsConfig
}

// VariationsData 保存变体数据和ASIN映射信息（向后兼容）
type VariationsData = variations.VariationsData

// NewVariationsExtractor 创建变体信息提取器实例
func NewVariationsExtractor() *VariationsExtractor {
	extractor := variations.NewExtractor()
	return &VariationsExtractor{
		extractor: extractor,
		config:    variations.GetDefaultConfig(),
	}
}

// NewVariationsExtractorWithConfig 使用自定义配置创建变体信息提取器实例
func NewVariationsExtractorWithConfig(config *VariationsConfig) *VariationsExtractor {
	extractor := variations.NewExtractorWithConfig(config)
	return &VariationsExtractor{
		extractor: extractor,
		config:    config,
	}
}

// Extract 提取产品变体信息
func (ve *VariationsExtractor) Extract(page playwright.Page, product *model.Product) error {
	// 提取变体数据
	variationsData, err := ve.extractor.ExtractFromPage(page)
	if err != nil {
		return err
	}

	// 转换为 VariationValue 格式
	var variationsValues []model.VariationValue
	for variantName, values := range variationsData.VariationsValues {
		if len(values) > 0 {
			variationsValues = append(variationsValues, model.VariationValue{
				VariantName: variantName,
				Values:      values,
			})
		}
	}
	product.VariationsValues = variationsValues

	// 构建变体列表
	varList := ve.extractor.BuildVariations(
		convertToVariationsValues(variationsValues),
		variationsData.ASINMapping,
		variationsData.PriceMapping,
		product.FinalPrice,
		"USD",
	)

	// 转换回 amazon.Variation 类型
	product.Variations = convertToAmazonVariations(varList)

	return nil
}

// GenerateCombinations 公开的组合生成方法，用于调试
func (ve *VariationsExtractor) GenerateCombinations(dimensions map[string][]string) []map[string]interface{} {
	combinator := variations.NewCombinator(ve.config)
	return combinator.Generate(dimensions)
}

// AttributesMatchGeneric 公开的属性匹配方法，用于调试
func (ve *VariationsExtractor) AttributesMatchGeneric(combo map[string]interface{}, asinAttrs map[string]string) bool {
	matcher := variations.NewMatcher(ve.config)
	return matcher.AttributesMatch(combo, asinAttrs)
}

// ValuesMatchGeneric 公开的值匹配方法，用于调试
func (ve *VariationsExtractor) ValuesMatchGeneric(value1, value2 string) bool {
	matcher := variations.NewMatcher(ve.config)
	return matcher.ValuesMatch(value1, value2)
}

// MapAttributeNames 将通用属性名映射为语义化名称
func (ve *VariationsExtractor) MapAttributeNames(attributes map[string]interface{}) map[string]interface{} {
	mapper := variations.NewMapper(ve.config)
	return mapper.MapAttributeNames(attributes)
}

// InferAttributeType 公开的属性类型推断方法，用于测试
func (ve *VariationsExtractor) InferAttributeType(value interface{}) string {
	mapper := variations.NewMapper(ve.config)
	return mapper.InferAttributeType(value)
}

// BuildVariationsFromValues 公开的变体构建方法，用于测试
func (ve *VariationsExtractor) BuildVariationsFromValues(product *model.Product, asinMapping map[string]map[string]string, priceMapping map[string]interface{}) []model.Variation {
	varList := ve.extractor.BuildVariations(
		convertToVariationsValues(product.VariationsValues),
		asinMapping,
		priceMapping,
		product.FinalPrice,
		"USD",
	)
	return convertToAmazonVariations(varList)
}

// 辅助函数：转换类型
func convertToVariationsValues(values []model.VariationValue) []variations.VariationValue {
	result := make([]variations.VariationValue, len(values))
	for i, v := range values {
		result[i] = variations.VariationValue{
			VariantName: v.VariantName,
			Values:      v.Values,
		}
	}
	return result
}

func convertToAmazonVariations(varList []variations.Variation) []model.Variation {
	result := make([]model.Variation, len(varList))
	for i, v := range varList {
		result[i] = model.Variation{
			Name: v.Name,
			Asin: v.Asin,
			//Price:      v.Price,
			//Currency:   v.Currency,
			Attributes: v.Attributes,
		}
	}
	return result
}
