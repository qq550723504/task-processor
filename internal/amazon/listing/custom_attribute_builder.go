package listing

import (
	"strings"
	"task-processor/internal/amazon/model"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// CustomAttributeBuilder 自定义属性构建器
type CustomAttributeBuilder struct {
	logger *logrus.Entry
}

// NewCustomAttributeBuilder 创建自定义属性构建器
func NewCustomAttributeBuilder() *CustomAttributeBuilder {
	return &CustomAttributeBuilder{
		logger: logger.GetGlobalLogger("CustomAttributeBuilder"),
	}
}

// AddCustomAttributes 添加用户自定义属性
func (cab *CustomAttributeBuilder) AddCustomAttributes(attrs map[string]any, data *model.ProductData, marketplaceID string) {
	for attrName, attrValue := range data.Attributes {
		if _, exists := attrs[attrName]; !exists {
			// 清理属性值中的特殊字符
			cleanValue := cab.sanitizeAttributeValue(attrValue)

			// 构建标准格式的属性值
			attrs[attrName] = []map[string]any{
				{"value": cleanValue, "marketplace_id": marketplaceID},
			}
		}
	}
}

// sanitizeAttributeValue 清理属性值中可能导致API错误的特殊字符
func (cab *CustomAttributeBuilder) sanitizeAttributeValue(value any) any {
	if str, ok := value.(string); ok {
		// 替换可能导致问题的特殊字符
		cleaned := strings.ReplaceAll(str, "<", "less than ")
		cleaned = strings.ReplaceAll(cleaned, ">", "greater than ")
		cleaned = strings.ReplaceAll(cleaned, "&", "and")

		// 如果值被修改了，记录日志
		if cleaned != str {
			cab.logger.WithFields(logrus.Fields{
				"original": str,
				"cleaned":  cleaned,
			}).Info("清理属性值中的特殊字符")
		}

		return cleaned
	}
	return value
}
