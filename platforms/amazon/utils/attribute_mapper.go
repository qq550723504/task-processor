package utils

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AttributeMapper 属性映射器
type AttributeMapper struct {
	config *AttributeConfig
}

// AttributeConfig 属性映射配置
type AttributeConfig struct {
	ProductTypes         map[string]ProductTypeConfig    `yaml:"product_types"`
	AttributeMappings    map[string]AttributeMappingRule `yaml:"attribute_mappings"`
	ValueTransformations map[string]map[string]string    `yaml:"value_transformations"`
	ValidationRules      map[string]ValidationRule       `yaml:"validation_rules"`
	DefaultValues        map[string]interface{}          `yaml:"default_values"`
}

// ProductTypeConfig 产品类型配置
type ProductTypeConfig struct {
	DisplayName        string   `yaml:"display_name"`
	RequiredAttributes []string `yaml:"required_attributes"`
	OptionalAttributes []string `yaml:"optional_attributes"`
}

// AttributeMappingRule 属性映射规则
type AttributeMappingRule struct {
	SourceFields []string    `yaml:"source_fields"`
	MaxLength    int         `yaml:"max_length"`
	Required     bool        `yaml:"required"`
	Default      interface{} `yaml:"default"`
	Unit         string      `yaml:"unit"`
}

// ValidationRule 验证规则
type ValidationRule struct {
	MinLength     int      `yaml:"min_length"`
	MaxLength     int      `yaml:"max_length"`
	Pattern       string   `yaml:"pattern"`
	AllowedValues []string `yaml:"allowed_values"`
	MinValue      float64  `yaml:"min_value"`
	MaxValue      float64  `yaml:"max_value"`
	Unit          string   `yaml:"unit"`
}

// NewAttributeMapper 创建属性映射器
func NewAttributeMapper(configPath string) (*AttributeMapper, error) {
	config, err := loadAttributeConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("加载属性映射配置失败: %w", err)
	}

	return &AttributeMapper{
		config: config,
	}, nil
}

// loadAttributeConfig 加载配置文件
func loadAttributeConfig(configPath string) (*AttributeConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config AttributeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &config, nil
}

// MapAttributes 映射属性
func (m *AttributeMapper) MapAttributes(
	sourceData map[string]interface{},
	productType string,
) (map[string]interface{}, error) {
	// 验证产品类型
	typeConfig, exists := m.config.ProductTypes[productType]
	if !exists {
		return nil, fmt.Errorf("不支持的产品类型: %s", productType)
	}

	result := make(map[string]interface{})

	// 映射必填属性
	for _, attrName := range typeConfig.RequiredAttributes {
		value, err := m.mapAttribute(sourceData, attrName)
		if err != nil {
			return nil, fmt.Errorf("映射必填属性 %s 失败: %w", attrName, err)
		}
		result[attrName] = value
	}

	// 映射可选属性
	for _, attrName := range typeConfig.OptionalAttributes {
		value, err := m.mapAttribute(sourceData, attrName)
		if err == nil && value != nil {
			result[attrName] = value
		}
	}

	return result, nil
}

// mapAttribute 映射单个属性
func (m *AttributeMapper) mapAttribute(
	sourceData map[string]interface{},
	attrName string,
) (interface{}, error) {
	// 获取映射规则
	rule, exists := m.config.AttributeMappings[attrName]
	if !exists {
		return nil, fmt.Errorf("未找到属性 %s 的映射规则", attrName)
	}

	// 从源数据中查找值
	var value interface{}
	for _, sourceField := range rule.SourceFields {
		if val, ok := sourceData[sourceField]; ok && val != nil {
			value = val
			break
		}
	}

	// 如果未找到值，使用默认值
	if value == nil {
		if rule.Default != nil {
			value = rule.Default
		} else if defaultVal, ok := m.config.DefaultValues[attrName]; ok {
			value = defaultVal
		} else if rule.Required {
			return nil, fmt.Errorf("必填属性 %s 缺失且无默认值", attrName)
		} else {
			return nil, nil
		}
	}

	// 转换值
	transformedValue := m.transformValue(attrName, value)

	// 验证长度限制
	if rule.MaxLength > 0 {
		if strVal, ok := transformedValue.(string); ok {
			if len(strVal) > rule.MaxLength {
				transformedValue = strVal[:rule.MaxLength]
			}
		}
	}

	return transformedValue, nil
}

// transformValue 转换属性值
func (m *AttributeMapper) transformValue(attrName string, value interface{}) interface{} {
	// 获取转换规则
	transformMap, exists := m.config.ValueTransformations[attrName]
	if !exists {
		return value
	}

	// 转换字符串值
	if strVal, ok := value.(string); ok {
		strVal = strings.TrimSpace(strVal)
		if transformed, found := transformMap[strVal]; found {
			return transformed
		}
		return strVal
	}

	return value
}

// GetProductTypeConfig 获取产品类型配置
func (m *AttributeMapper) GetProductTypeConfig(productType string) (*ProductTypeConfig, error) {
	config, exists := m.config.ProductTypes[productType]
	if !exists {
		return nil, fmt.Errorf("不支持的产品类型: %s", productType)
	}
	return &config, nil
}

// GetRequiredAttributes 获取必填属性列表
func (m *AttributeMapper) GetRequiredAttributes(productType string) ([]string, error) {
	config, err := m.GetProductTypeConfig(productType)
	if err != nil {
		return nil, err
	}
	return config.RequiredAttributes, nil
}

// GetOptionalAttributes 获取可选属性列表
func (m *AttributeMapper) GetOptionalAttributes(productType string) ([]string, error) {
	config, err := m.GetProductTypeConfig(productType)
	if err != nil {
		return nil, err
	}
	return config.OptionalAttributes, nil
}
