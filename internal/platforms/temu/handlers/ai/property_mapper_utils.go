package ai

import (
	"strings"

	models "task-processor/internal/platforms/temu/api/product"
	temutemplate "task-processor/internal/platforms/temu/api/template"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// getPropertyTypeName 获取属性类型名称
func getPropertyTypeName(propertyValueType int) string {
	switch propertyValueType {
	case 1:
		return "selection"
	case 2:
		return "numeric"
	case 3:
		return "text"
	default:
		return "unknown"
	}
}

// preparePropertyMappingData 准备属性映射数据
func preparePropertyMappingData(temuCtx *temucontext.TemuTaskContext, templateProps []temutemplate.TemplateRespGoodsProperty) temucontext.PropertyMappingData {
	data := temucontext.PropertyMappingData{
		TemuProperties: make([]temutemplate.TemplateRespGoodsProperty, 0, len(templateProps)),
	}

	if len(templateProps) == 0 {
		logrus.Warn("⚠️ 模板属性列表为空，属性修复可能无法正常工作")
	} else {
		logrus.Infof("📋 准备属性映射数据，模板属性数量: %d", len(templateProps))
	}

	if temuCtx.GetAmazonProduct() != nil {
		data.AmazonProduct = convertAmazonProductData(temuCtx)
	}

	for _, templateProp := range templateProps {
		data.TemuProperties = append(data.TemuProperties, templateProp)
	}

	return data
}

// convertAmazonProductData 将Amazon产品数据转换为AI映射所需的格式
func convertAmazonProductData(temuCtx *temucontext.TemuTaskContext) temucontext.AmazonProductData {
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct == nil {
		return temucontext.AmazonProductData{}
	}

	productDetails := make([]temucontext.ProductDetailData, 0, len(amazonProduct.ProductDetails))
	for _, detail := range amazonProduct.ProductDetails {
		productDetails = append(productDetails, temucontext.ProductDetailData{
			Type:  detail.Type,
			Value: detail.Value,
		})
	}

	return temucontext.AmazonProductData{
		Title:             amazonProduct.Title,
		Brand:             amazonProduct.Brand,
		Description:       amazonProduct.Description,
		Features:          amazonProduct.Features,
		ProductDetails:    productDetails,
		ProductDimensions: amazonProduct.ProductDimensions,
		ItemWeight:        amazonProduct.ItemWeight,
		ModelNumber:       amazonProduct.ModelNumber,
		Department:        amazonProduct.Department,
		Manufacturer:      amazonProduct.Manufacturer,
		Categories:        amazonProduct.Categories,
	}
}

// selectBestTemplate 从多个相同PID的模板中选择最佳匹配
func (m *AIPropertyMapper) selectBestTemplate(prop *models.PropertyItem, templates []temutemplate.TemplateRespGoodsProperty) *temutemplate.TemplateRespGoodsProperty {
	m.logger.Debugf("🎯 为PID=%d选择最佳模板，候选数量: %d", prop.Pid, len(templates))

	for _, template := range templates {
		if m.isValidVID(prop.Vid, template.Values) {
			m.logger.Debugf("✅ VID精确匹配: %s (VID=%d)", template.Name, prop.Vid)
			return &template
		}
	}

	bestMatch := m.findBestValueMatch(prop.Value, templates)
	if bestMatch != nil {
		m.logger.Debugf("✅ 值语义匹配: %s ← %s", bestMatch.Name, prop.Value)
		return bestMatch
	}

	dependentMatch := m.selectByDependency(prop, templates)
	if dependentMatch != nil {
		m.logger.Debugf("✅ 依赖关系匹配: %s", dependentMatch.Name)
		return dependentMatch
	}

	for _, template := range templates {
		if template.Required {
			m.logger.Debugf("✅ 选择必填属性: %s", template.Name)
			return &template
		}
	}

	m.logger.Debugf("⚠️ 使用默认选择: %s", templates[0].Name)
	return &templates[0]
}

// findBestValueMatch 找到最佳的值匹配模板
func (m *AIPropertyMapper) findBestValueMatch(propValue string, templates []temutemplate.TemplateRespGoodsProperty) *temutemplate.TemplateRespGoodsProperty {
	propValue = strings.ToLower(propValue)

	var bestMatch *temutemplate.TemplateRespGoodsProperty
	var bestScore int

	for _, template := range templates {
		score := m.calculateValueMatchScore(propValue, template)
		if score > bestScore {
			bestScore = score
			bestMatch = &template
		}
	}

	if bestScore > 0 {
		return bestMatch
	}
	return nil
}

// calculateValueMatchScore 计算值匹配分数
func (m *AIPropertyMapper) calculateValueMatchScore(propValue string, template temutemplate.TemplateRespGoodsProperty) int {
	score := 0

	for _, value := range template.Values {
		templateValue := strings.ToLower(value.Value)

		if propValue == templateValue {
			return 100
		}

		if strings.Contains(propValue, templateValue) || strings.Contains(templateValue, propValue) {
			score = max(score, 50)
		}

		if m.hasKeywordMatch(propValue, templateValue) {
			score = max(score, 30)
		}
	}

	return score
}

// hasKeywordMatch 检查关键词匹配
func (m *AIPropertyMapper) hasKeywordMatch(propValue, templateValue string) bool {
	propWords := strings.Fields(propValue)
	templateWords := strings.Fields(templateValue)

	for _, propWord := range propWords {
		for _, templateWord := range templateWords {
			if len(propWord) > 2 && len(templateWord) > 2 {
				if strings.Contains(propWord, templateWord) || strings.Contains(templateWord, propWord) {
					return true
				}
			}
		}
	}

	return false
}

// selectByDependency 根据依赖关系选择模板
func (m *AIPropertyMapper) selectByDependency(prop *models.PropertyItem, templates []temutemplate.TemplateRespGoodsProperty) *temutemplate.TemplateRespGoodsProperty {
	for _, template := range templates {
		if len(template.TemplatePropertyValueParentList) > 0 {
			if m.isDependencyReasonable(prop, template) {
				return &template
			}
		}
	}

	return nil
}

// isDependencyReasonable 检查依赖关系是否合理
func (m *AIPropertyMapper) isDependencyReasonable(prop *models.PropertyItem, template temutemplate.TemplateRespGoodsProperty) bool {
	propValue := strings.ToLower(prop.Value)
	templateName := strings.ToLower(template.Name)

	propWords := strings.Fields(propValue)
	templateWords := strings.Fields(templateName)

	for _, propWord := range propWords {
		for _, templateWord := range templateWords {
			if len(propWord) > 2 && len(templateWord) > 2 {
				if strings.Contains(propWord, templateWord) || strings.Contains(templateWord, propWord) {
					return true
				}
			}
		}
	}

	for _, value := range template.Values {
		templateValue := strings.ToLower(value.Value)
		if strings.Contains(propValue, templateValue) || strings.Contains(templateValue, propValue) {
			return true
		}
	}

	return false
}
