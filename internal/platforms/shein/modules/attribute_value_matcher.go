// Package modules 提供SHEIN平台的属性值匹配功能
package modules

import (
	"strings"
	"task-processor/internal/common/shein/api/attribute"

	"github.com/sirupsen/logrus"
)

// AttributeValueMatcher 属性值匹配器，负责在平台已有属性值中查找匹配
type AttributeValueMatcher struct{}

// NewAttributeValueMatcher 创建新的属性值匹配器
// 返回值:
//   - *AttributeValueMatcher: 属性值匹配器实例
func NewAttributeValueMatcher() *AttributeValueMatcher {
	return &AttributeValueMatcher{}
}

// FindMatchingPlatformValue 在平台值中查找匹配的属性值
// 参数:
//   - value: 要匹配的属性值
//   - platformValues: 平台已有的属性值映射表
//
// 返回值:
//   - int: 匹配到的平台属性值ID，0表示未找到匹配
func (m *AttributeValueMatcher) FindMatchingPlatformValue(value string, platformValues map[string]int) int {
	if value == "" {
		return 0
	}

	// 1. 精确匹配（优先级最高）
	if id, exists := platformValues[value]; exists {
		logrus.Debugf("✓ 属性值 '%s' 找到精确匹配，平台ID: %d", value, id)
		return id
	}

	// 2. 忽略大小写匹配
	lowerValue := strings.ToLower(strings.TrimSpace(value))
	if id, exists := platformValues[lowerValue]; exists {
		logrus.Debugf("✓ 属性值 '%s' 找到大小写匹配，平台ID: %d", value, id)
		return id
	}

	// 3. 清理特殊字符后匹配
	sanitizedValue := SanitizeForSheinAttribute(value)
	if sanitizedValue != value {
		logrus.Debugf("尝试使用清理后的值进行匹配: '%s' -> '%s'", value, sanitizedValue)

		// 使用清理后的值进行精确匹配
		if id, exists := platformValues[sanitizedValue]; exists {
			logrus.Debugf("✓ 属性值 '%s' 清理后找到精确匹配，平台ID: %d", value, id)
			return id
		}

		// 使用清理后的值进行大小写匹配
		lowerSanitized := strings.ToLower(strings.TrimSpace(sanitizedValue))
		if id, exists := platformValues[lowerSanitized]; exists {
			logrus.Debugf("✓ 属性值 '%s' 清理后找到大小写匹配，平台ID: %d", value, id)
			return id
		}
	}

	// 4. 模糊匹配（仅去除特殊字符，不做颜色标准化）
	normalizedValue := m.normalizeValueForMatching(value)
	for platformValue, id := range platformValues {
		if m.normalizeValueForMatching(platformValue) == normalizedValue {
			logrus.Debugf("✓ 属性值 '%s' 找到模糊匹配，平台ID: %d", value, id)
			return id
		}
	}

	// 移除颜色特殊处理，保持Amazon原始属性值的完整性
	logrus.Debugf("✗ 属性值 '%s' 在平台中未找到任何匹配", value)
	return 0
}

// GetPlatformAttributeValues 获取平台属性的所有可用值
// 参数:
//   - attrID: 属性ID
//   - attributeTemplates: 属性模板信息
//
// 返回值:
//   - map[string]int: 属性值到ID的映射表
func (m *AttributeValueMatcher) GetPlatformAttributeValues(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) map[string]int {
	platformValues := make(map[string]int)

	if attributeTemplates == nil || len(attributeTemplates.Data) == 0 {
		logrus.Warnf("属性模板为空，无法获取平台属性值")
		return platformValues
	}

	// 遍历属性模板找到对应的属性
	for _, template := range attributeTemplates.Data {
		for _, attrInfo := range template.AttributeInfos {
			if attrInfo.AttributeID == attrID {
				// 提取属性值列表
				for _, valueInfo := range attrInfo.AttributeValueInfoList {
					if valueInfo.AttributeValueID > 0 && valueInfo.AttributeValue != "" {
						// 支持多种匹配方式
						platformValues[strings.ToLower(strings.TrimSpace(valueInfo.AttributeValue))] = valueInfo.AttributeValueID
						// 也存储原始值
						platformValues[valueInfo.AttributeValue] = valueInfo.AttributeValueID
						// 如果有英文值，也存储
						if valueInfo.AttributeValueEn != "" {
							platformValues[strings.ToLower(strings.TrimSpace(valueInfo.AttributeValueEn))] = valueInfo.AttributeValueID
							platformValues[valueInfo.AttributeValueEn] = valueInfo.AttributeValueID
						}
					}
				}
				logrus.Debugf("属性ID %d 找到 %d 个平台属性值", attrID, len(platformValues)/2) // 除以2因为每个值存储了两次
				return platformValues
			}
		}
	}

	logrus.Warnf("在属性模板中未找到属性ID %d", attrID)
	return platformValues
}

// normalizeValueForMatching 标准化属性值用于匹配
func (m *AttributeValueMatcher) normalizeValueForMatching(value string) string {
	// 只做最基本的标准化：转小写和去除首尾空格
	normalized := strings.ToLower(strings.TrimSpace(value))
	// 只去除多余的空格，保留其他字符以维持属性值的唯一性
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}
