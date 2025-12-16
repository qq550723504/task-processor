// Package service 提供Amazon属性默认值功能
package service

import (
	"task-processor/platforms/amazon/internal/model"

	"github.com/sirupsen/logrus"
)

// DefaultValueProvider 默认值提供器
type DefaultValueProvider struct {
	marketplaceID string
	languageTag   string
	brand         string
	logger        *logrus.Entry
}

// NewDefaultValueProvider 创建默认值提供器
func NewDefaultValueProvider(marketplaceID, languageTag, brand string) *DefaultValueProvider {
	return &DefaultValueProvider{
		marketplaceID: marketplaceID,
		languageTag:   languageTag,
		brand:         brand,
		logger:        logrus.WithField("component", "DefaultValueProvider"),
	}
}

// GetDefaultValue 获取属性默认值
func (p *DefaultValueProvider) GetDefaultValue(attributeName string) any {
	switch attributeName {
	case "closure":
		return p.getClosureDefault()
	case "country_of_origin":
		return p.getCountryOfOriginDefault()
	case "import_designation":
		return p.getImportDesignationDefault()
	case "material":
		return p.getMaterialDefault()
	case "care_instructions":
		return p.getCareInstructionsDefault()
	case "target_audience":
		return p.getTargetAudienceDefault()
	default:
		return nil
	}
}

// getClosureDefault 获取closure属性默认值
func (p *DefaultValueProvider) getClosureDefault() any {
	return []map[string]any{
		{"value": "pull_on", "marketplace_id": p.marketplaceID},
	}
}

// getCountryOfOriginDefault 获取原产国默认值
func (p *DefaultValueProvider) getCountryOfOriginDefault() any {
	return []map[string]any{
		{"value": "CN", "marketplace_id": p.marketplaceID},
	}
}

// getImportDesignationDefault 获取进口标识默认值
func (p *DefaultValueProvider) getImportDesignationDefault() any {
	return []map[string]any{
		{"value": "imported", "marketplace_id": p.marketplaceID},
	}
}

// getMaterialDefault 获取材质默认值
func (p *DefaultValueProvider) getMaterialDefault() any {
	return []map[string]any{
		{"value": "cotton", "marketplace_id": p.marketplaceID},
	}
}

// getCareInstructionsDefault 获取护理说明默认值
func (p *DefaultValueProvider) getCareInstructionsDefault() any {
	return []map[string]any{
		{"value": "machine_wash", "language_tag": p.languageTag, "marketplace_id": p.marketplaceID},
	}
}

// getTargetAudienceDefault 获取目标受众默认值
func (p *DefaultValueProvider) getTargetAudienceDefault() any {
	return []map[string]any{
		{"value": "unisex_adult", "marketplace_id": p.marketplaceID},
	}
}

// GetCommonDefaults 获取常用默认值映射
func (p *DefaultValueProvider) GetCommonDefaults() map[string]any {
	return map[string]any{
		"country_of_origin":  p.getCountryOfOriginDefault(),
		"import_designation": p.getImportDesignationDefault(),
		"material":           p.getMaterialDefault(),
		"care_instructions":  p.getCareInstructionsDefault(),
		"target_audience":    p.getTargetAudienceDefault(),
		"closure":            p.getClosureDefault(),
	}
}

// BuildAttributeWithDefault 构建带默认值的属性
func (p *DefaultValueProvider) BuildAttributeWithDefault(attr model.AttributeInfo, value any) any {
	if value != nil {
		// 如果有值，使用提供的值
		return []map[string]any{
			{"value": value, "marketplace_id": p.marketplaceID},
		}
	}

	// 如果没有值，尝试获取默认值
	return p.GetDefaultValue(attr.Name)
}
