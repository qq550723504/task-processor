// Package attribute 提供SHEIN平台属性重要性计算功能
package attribute

import (
	"strings"

	"task-processor/internal/platforms/shein/api/attribute"
	"task-processor/internal/platforms/shein"
)

// ImportanceService 属性重要性计算服务
type ImportanceService struct {
	calculator *shein.AttributeImportanceCalculator
}

// NewImportanceService 创建新的属性重要性计算服务
func NewImportanceService() *ImportanceService {
	return &ImportanceService{
		calculator: shein.NewAttributeImportanceCalculator(),
	}
}

// EnhanceAttributeDataWithTemplateInfo 增强属性数据，添加重要性评分和依赖关系
func (s *ImportanceService) EnhanceAttributeDataWithTemplateInfo(attributeData []shein.GenerateAttribute, attributeTemplates *attribute.AttributeTemplateInfo) []shein.EnhancedGenerateAttribute {
	var enhancedData []shein.EnhancedGenerateAttribute

	// 创建属性模板映射，便于快速查找
	templateMap := make(map[int]*attribute.AttributeInfo)
	if len(attributeTemplates.Data) > 0 {
		for _, attribute := range attributeTemplates.Data[0].AttributeInfos {
			templateMap[attribute.AttributeID] = &attribute
		}
	}

	for _, attr := range attributeData {
		enhanced := shein.EnhancedGenerateAttribute{
			AttrID:        attr.AttrID,
			AttrValue:     attr.AttrValue,
			Required:      attr.Required,
			Type:          attr.Type,
			Importance:    0,
			AttributeName: "",
		}

		// 从属性模板中获取增强信息
		if template, exists := templateMap[attr.AttrID]; exists {
			// 使用统一的重要性计算函数
			importanceResult := s.CalculateAttributeImportance(template)

			// 设置重要性评分和相关标识
			enhanced.Importance = importanceResult.Importance
			enhanced.HasRemarkList = importanceResult.HasRemarkList
			enhanced.IsLabelAttribute = importanceResult.IsLabelAttribute
			enhanced.IsSampleAttribute = importanceResult.IsSampleAttribute
			enhanced.IsActiveStatus = importanceResult.IsActiveStatus

			// 设置属性名称
			enhanced.AttributeName = template.AttributeName

			// 检查依赖关系
			deps := s.getAttributeDependencies(attr.AttrID)
			enhanced.HasDependency = len(deps) > 0
		}

		enhancedData = append(enhancedData, enhanced)
	}

	return enhancedData
}

// CalculateAttributeImportance 计算属性重要性评分
func (s *ImportanceService) CalculateAttributeImportance(attribute *attribute.AttributeInfo) shein.AttributeImportanceResult {
	result := shein.AttributeImportanceResult{
		Importance: 0,
	}

	// 基础重要性评分规则
	if len(attribute.AttributeRemarkList) > 0 {
		result.Importance += 100 // 有备注列表 +100分
		result.HasRemarkList = true
	}
	if attribute.AttributeLabel == 1 {
		result.Importance += 80 // 标签为1 +80分
		result.IsLabelAttribute = true
	}
	if attribute.IsSample == 1 {
		result.Importance += 40 // 示例属性 +40分
		result.IsSampleAttribute = true
	}
	if attribute.AttributeStatus == 3 {
		result.Importance += 30 // 状态为3 +30分
		result.IsActiveStatus = true
	}
	if attribute.AttributeIsShow == 1 {
		result.Importance += 20 // 显示标记 +20分
	}

	// 为关键主属性添加特殊权重 - 确保电商重要属性优先级
	if s.IsKeyPrimaryAttribute(attribute.AttributeName, attribute.AttributeNameEn) {
		result.Importance += 60 // 关键主属性 +60分
		result.IsKeyPrimary = true
	}

	return result
}

// IsKeyPrimaryAttribute 判断是否为关键主属性
func (s *ImportanceService) IsKeyPrimaryAttribute(attributeName string, attributeNameEn string) bool {
	// 定义关键主属性列表 - 这些属性在电商平台中通常是最重要的主属性
	keyPrimaryAttributes := map[string]bool{
		// 中文属性名
		"颜色":   true,
		"尺寸":   false,
		"香味":   true,
		"净含量":  true,
		"风格":   true,
		"材质":   true,
		"其他材质": true,
		"功能":   true,
		"类别":   true,
		// 英文属性名
		"color":          true,
		"size":           false,
		"scent":          true,
		"Net Content":    true,
		"Style Type":     false,
		"Other Material": true,
		"Material":       true,
		"Function":       true,
		"Type":           true,
	}

	// 检查中文属性名
	if isKey, exists := keyPrimaryAttributes[attributeName]; exists {
		return isKey
	}

	// 检查英文属性名
	if isKey, exists := keyPrimaryAttributes[strings.ToLower(attributeNameEn)]; exists {
		return isKey
	}

	// 通过属性名包含关键词判断
	attributeNameLower := strings.ToLower(attributeName)
	attributeNameEnLower := strings.ToLower(attributeNameEn)

	// 特别重要的属性关键词
	highPriorityKeywords := []string{"颜色", "color", "材质", "material", "风格", "style", "香味", "scent", "净含量", "net", "功能", "function"}
	for _, keyword := range highPriorityKeywords {
		if strings.Contains(attributeNameLower, keyword) || strings.Contains(attributeNameEnLower, keyword) {
			return true
		}
	}

	return false
}

// getAttributeDependencies 获取属性的依赖关系
func (s *ImportanceService) getAttributeDependencies(attrID int) []int {
	dependencies := map[int][]int{
		1002187: {1002188, 1002189}, // 主料类型2依赖
	}

	if deps, exists := dependencies[attrID]; exists {
		return deps
	}
	return []int{}
}


