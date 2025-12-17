package variations

// Config 变体提取配置
type Config struct {
	// 属性优先级，用于排序和显示
	AttributePriority []string
	// 统一的属性名映射配置（合并了KeyNormalization和AttributeNameMapping）
	AttributeMapping map[string]string
	// 属性类型映射，用于智能推断
	AttributeTypeMapping map[string][]string
	// 是否启用智能推断
	EnableSmartInference bool
	// 是否启用详细日志
	EnableDebugLogging bool
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() *Config {
	return &Config{
		AttributePriority: []string{"size", "color", "item_package_quantity", "style", "pattern", "material", "brand"},
		AttributeMapping: map[string]string{
			// 原始属性名到标准化名称的映射
			"color_name":            "color",
			"size_name":             "size",
			"quantity":              "item_package_quantity",
			"item_package_quantity": "item_package_quantity",
			// 通用属性名到语义化名称的映射
			"attribute_1":   "color",
			"attribute_2":   "size",
			"attribute_3":   "style",
			"attribute_4":   "material",
			"attribute_5":   "pattern",
			"variant_code":  "variant",
			"variant_style": "style",
		},
		AttributeTypeMapping: map[string][]string{
			"color":    {"color"},
			"size":     {"size", "product dimensions"},
			"material": {"material"},
			"brand":    {"brand"},
		},
		EnableSmartInference: true,
		EnableDebugLogging:   false,
	}
}
