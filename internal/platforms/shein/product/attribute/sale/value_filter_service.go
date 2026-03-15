// Package modules 提供SHEIN平台销售属性值筛选功能
package sale

import (
	"strings"
	"task-processor/internal/domain/model"
	shein_model "task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// SaleAttributeValueFilter 销售属性值筛选器，负责根据实际变体使用情况筛选属性值
type SaleAttributeValueFilter struct{}

// NewSaleAttributeValueFilter 创建属性值筛选器实例
func NewSaleAttributeValueFilter() *SaleAttributeValueFilter {
	return &SaleAttributeValueFilter{}
}

// FilterAttributeValuesByUsage 根据实际变体使用情况筛选属性值
// 参数:
//   - candidateValues: 候选属性值列表
//   - actualValues: 变体中实际使用的属性值
//   - attributeName: 属性名称（用于日志）
//
// 返回值:
//   - []GenerateAttributeValue: 筛选后的属性值列表
func (f *SaleAttributeValueFilter) FilterAttributeValuesByUsage(
	candidateValues []shein_model.GenerateAttributeValue,
	actualValues []string,
	attributeName string,
) []shein_model.GenerateAttributeValue {
	if len(actualValues) == 0 {
		logrus.Debugf("属性 %s 没有实际使用值，保留前5个候选值", attributeName)
		if len(candidateValues) > 5 {
			return candidateValues[:5]
		}
		return candidateValues
	}

	var filteredValues []shein_model.GenerateAttributeValue
	usedValueMap := make(map[string]bool)

	// 创建实际使用值的映射
	for _, actualValue := range actualValues {
		usedValueMap[strings.TrimSpace(actualValue)] = true
	}

	// 筛选匹配的候选值
	for _, candidate := range candidateValues {
		candidateValue := strings.TrimSpace(candidate.Value)
		if usedValueMap[candidateValue] {
			filteredValues = append(filteredValues, candidate)
		}
	}

	// 如果没有找到匹配的值，保留前3个候选值作为备选
	if len(filteredValues) == 0 {
		logrus.Warnf("属性 %s 没有找到匹配的候选值，保留前3个作为备选", attributeName)
		if len(candidateValues) > 3 {
			return candidateValues[:3]
		}
		return candidateValues
	}

	logrus.Infof("属性 %s 筛选结果：从 %d 个候选值筛选出 %d 个实际使用值",
		attributeName, len(candidateValues), len(filteredValues))

	return filteredValues
}

// ExtractActualValuesFromVariations 从变体数据中提取实际使用的属性值
// 参数:
//   - variationsValues: Amazon产品的变体属性值
//   - attributeName: 要提取的属性名称
//
// 返回值:
//   - []string: 实际使用的属性值列表
func (f *SaleAttributeValueFilter) ExtractActualValuesFromVariations(
	variationsValues []model.VariationValue,
	attributeName string,
) []string {
	var actualValues []string
	valueMap := make(map[string]bool)

	for _, variation := range variationsValues {
		// 匹配属性名称（不区分大小写）
		if strings.EqualFold(variation.VariantName, attributeName) ||
			strings.EqualFold(variation.VariantName, strings.ToLower(attributeName)) {
			for _, value := range variation.Values {
				trimmedValue := strings.TrimSpace(value)
				if trimmedValue != "" && !valueMap[trimmedValue] {
					actualValues = append(actualValues, trimmedValue)
					valueMap[trimmedValue] = true
				}
			}
			break
		}
	}

	return actualValues
}

// ExtractActualValuesFromProducts 从产品数据中提取实际使用的属性值
// 参数:
//   - productsData: 产品变体数据
//   - attributeName: 要提取的属性名称
//
// 返回值:
//   - []string: 实际使用的属性值列表
func (f *SaleAttributeValueFilter) ExtractActualValuesFromProducts(
	productsData []shein_model.ProductVariantData,
	attributeName string,
) []string {
	var actualValues []string
	valueMap := make(map[string]bool)

	for _, product := range productsData {
		for key, value := range product.Attributes {
			// 匹配属性名称（不区分大小写）
			if strings.EqualFold(key, attributeName) ||
				strings.EqualFold(key, strings.ToLower(attributeName)) {
				trimmedValue := strings.TrimSpace(value)
				if trimmedValue != "" && !valueMap[trimmedValue] {
					actualValues = append(actualValues, trimmedValue)
					valueMap[trimmedValue] = true
				}
			}
		}
	}

	return actualValues
}
