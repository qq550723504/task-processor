package sale

import (
	"fmt"
	"strings"
	"task-processor/internal/domain/model"
	shein_model "task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// compareAttributeDataDifferences 对比AI生成前后的属性数据差异
func (h *SaleAttributeHandler) compareAttributeDataDifferences(amazonProduct model.Product, generatedData shein_model.ResultSaleAttribute) {
	logrus.Info("⚖️ [AI属性对比] 开始对比原始数据与AI生成数据的差异")

	if amazonProduct.VariationsValues == nil {
		logrus.Warn("⚠️ [AI属性对比] 原始数据中没有变体数据，无法进行对比")
		return
	}

	// 构建原始属性值映射 (属性名 -> 属性值列表)
	originalAttributeMap := make(map[string][]string)
	for _, variation := range amazonProduct.VariationsValues {
		// 标准化属性名（转为小写，去除空格）
		normalizedName := strings.ToLower(strings.ReplaceAll(variation.VariantName, " ", ""))
		originalAttributeMap[normalizedName] = variation.Values
	}

	// 构建AI生成的属性值映射
	aiAttributeMap := make(map[string][]string)
	for _, saleAttr := range generatedData.SaleAttributes {
		attrKey := fmt.Sprintf("attr_%d", saleAttr.AttrID)
		var values []string
		for _, attrValue := range saleAttr.AttrValue {
			values = append(values, attrValue.Value)
		}
		aiAttributeMap[attrKey] = values
	}
	_ = aiAttributeMap // 避免未使用变量警告

	// 从变体中提取AI生成的属性值
	aiVariantAttributeMap := make(map[string]map[string]bool) // 属性名 -> 属性值 -> 是否存在
	for _, variant := range generatedData.Variants {
		for attrName, attrValue := range variant.Attributes {
			normalizedAttrName := strings.ToLower(strings.ReplaceAll(attrName, " ", ""))
			if aiVariantAttributeMap[normalizedAttrName] == nil {
				aiVariantAttributeMap[normalizedAttrName] = make(map[string]bool)
			}
			aiVariantAttributeMap[normalizedAttrName][attrValue] = true
		}
	}

	// 对比分析
	var inconsistencies []string
	var matchedAttributes []string
	var newAttributes []string

	// 检查原始属性是否被AI正确使用
	for originalAttrName, originalValues := range originalAttributeMap {
		found := false
		var matchedAIAttrName string

		// 在AI生成的变体属性中查找匹配
		for aiAttrName, aiValues := range aiVariantAttributeMap {
			if isAttributeNameSimilar(originalAttrName, aiAttrName) {
				found = true
				matchedAIAttrName = aiAttrName

				// 检查属性值是否一致
				originalValueSet := convertToSet(originalValues)
				aiValueSet := convertToSet(getKeysFromMap(aiValues))

				if isValueSetEqual(originalValueSet, aiValueSet) {
					matchedAttributes = append(matchedAttributes, fmt.Sprintf("✅ 属性'%s'->'%s'完全匹配: %v",
						originalAttrName, matchedAIAttrName, originalValues))
				} else {
					inconsistencies = append(inconsistencies, fmt.Sprintf("❌ 属性'%s'->'%s'值不匹配: 原始%v vs AI生成%v",
						originalAttrName, matchedAIAttrName, originalValues, getKeysFromMap(aiValues)))
				}
				break
			}
		}

		if !found {
			inconsistencies = append(inconsistencies, fmt.Sprintf("⚠️ 原始属性'%s'在AI生成数据中未找到: %v", originalAttrName, originalValues))
		}
	}

	// 检查AI是否生成了原始数据中不存在的新属性
	for aiAttrName, aiValues := range aiVariantAttributeMap {
		found := false
		for originalAttrName := range originalAttributeMap {
			if isAttributeNameSimilar(originalAttrName, aiAttrName) {
				found = true
				break
			}
		}
		if !found {
			newAttributes = append(newAttributes, fmt.Sprintf("🆕 AI新增属性'%s': %v", aiAttrName, getKeysFromMap(aiValues)))
		}
	}

	// 详细记录匹配的属性
	for _, match := range matchedAttributes {
		logrus.Info(fmt.Sprintf("⚖️ [AI属性对比] %s", match))
	}

	// 详细记录不一致的属性
	for _, inconsistency := range inconsistencies {
		logrus.Warn(fmt.Sprintf("⚖️ [AI属性对比] %s", inconsistency))
	}

	// 详细记录新增的属性
	for _, newAttr := range newAttributes {
		logrus.Info(fmt.Sprintf("⚖️ [AI属性对比] %s", newAttr))
	}

	// 总体评估
	if len(inconsistencies) == 0 {
		logrus.Info("⚖️ [AI属性对比] ✅ AI生成的属性与原始数据完全一致")
	} else {
		logrus.Warnf("⚖️ [AI属性对比] ⚠️ 发现%d个不一致项", len(inconsistencies))
	}
}
