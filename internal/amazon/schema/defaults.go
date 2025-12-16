// Package schema 提供Amazon产品属性默认值配置
package schema

// DefaultValueProvider 默认值提供器
type DefaultValueProvider struct {
	marketplaceID string
	languageTag   string
	brand         string
}

// NewDefaultValueProvider 创建默认值提供器
func NewDefaultValueProvider(marketplaceID, languageTag, brand string) *DefaultValueProvider {
	return &DefaultValueProvider{
		marketplaceID: marketplaceID,
		languageTag:   languageTag,
		brand:         brand,
	}
}

// GetDefaultValue 获取属性默认值
func (p *DefaultValueProvider) GetDefaultValue(attrName string) any {
	// 简单格式默认值
	simpleDefaults := map[string]any{
		// 通用属性
		"supplier_declared_dg_hz_regulation": "not_applicable",
		"batteries_required":                 false,
		"country_of_origin":                  "CN",
		"number_of_items":                    1,
		"merchant_suggested_asin":            "B000000000",

		// 服装属性
		"target_gender":         "unisex",
		"style":                 "casual",
		"care_instructions":     "machine_wash",
		"age_range_description": "adult",
		"size":                  "Medium",
		"color":                 "Black",
		"material":              "Cotton",
		"fabric_type":           "cotton",
		"department":            "Clothing",
		"import_designation":    "imported",
		"item_type_keyword":     "product",

		// 电子产品属性
		"connectivity_technology":  "Wireless",
		"headphones_form_factor":   "over_ear",
		"headphones_ear_placement": "over_ear",

		// 厨房用品属性
		"handle_material": "Stainless Steel",
		"blade_color":     "Silver",
		"blade_edge":      "Plain",
		"blade_material":  "Stainless Steel",

		// 汽车配件属性
		"automotive_fit_type":            "Universal Fit",
		"part_number":                    "PART-001",
		"is_assembly_required":           "false",
		"product_compliance_certificate": "not_applicable",
	}

	if val, ok := simpleDefaults[attrName]; ok {
		return p.buildSimpleValue(val)
	}

	// 带语言标签的属性
	langTagDefaults := map[string]string{
		"manufacturer":         p.brand,
		"warranty_description": "1 Year Manufacturer Warranty",
		"included_components":  "Product, User Manual",
	}

	if val, ok := langTagDefaults[attrName]; ok {
		return p.buildLangTagValue(val)
	}

	// 特殊格式属性
	switch attrName {
	case "closure":
		return p.buildClosureValue()
	case "blade_length", "item_length":
		return p.buildDimensionValue(20, "centimeters")
	}

	return nil
}

// buildSimpleValue 构建简单格式值
func (p *DefaultValueProvider) buildSimpleValue(value any) []map[string]any {
	return []map[string]any{
		{"value": value, "marketplace_id": p.marketplaceID},
	}
}

// buildLangTagValue 构建带语言标签的值
func (p *DefaultValueProvider) buildLangTagValue(value string) []map[string]any {
	return []map[string]any{
		{"value": value, "language_tag": p.languageTag, "marketplace_id": p.marketplaceID},
	}
}

// buildClosureValue 构建闭合类型值（嵌套格式）
func (p *DefaultValueProvider) buildClosureValue() []map[string]any {
	return []map[string]any{
		{
			"marketplace_id": p.marketplaceID,
			"type": []map[string]any{
				{"language_tag": p.languageTag, "value": "Pull-On"},
			},
		},
	}
}

// buildDimensionValue 构建尺寸值
func (p *DefaultValueProvider) buildDimensionValue(value float64, unit string) []map[string]any {
	return []map[string]any{
		{
			"value":          value,
			"unit":           unit,
			"marketplace_id": p.marketplaceID,
		},
	}
}
