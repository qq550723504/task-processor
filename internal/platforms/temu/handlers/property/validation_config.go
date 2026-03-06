// Package handlers 提供TEMU平台的属性验证配置功能
package property

// PropertyValidationConfig 属性验证配置
type PropertyValidationConfig struct {
	// 是否启用严格验证模式
	StrictMode bool

	// 是否自动修复无效值
	AutoFix bool

	// 是否记录详细的验证日志
	VerboseLogging bool

	// 材质属性的特殊处理规则
	MaterialRules MaterialValidationRules

	// 颜色属性的特殊处理规则
	ColorRules ColorValidationRules
}

// MaterialValidationRules 材质验证规则
type MaterialValidationRules struct {
	// 是否启用材质智能匹配
	EnableIntelligentMatch bool

	// 材质关键词映射
	KeywordMapping map[string][]string
}

// ColorValidationRules 颜色验证规则
type ColorValidationRules struct {
	// 是否启用颜色智能匹配
	EnableIntelligentMatch bool

	// 颜色关键词映射
	KeywordMapping map[string][]string
}

// DefaultPropertyValidationConfig 返回默认的属性验证配置
func DefaultPropertyValidationConfig() PropertyValidationConfig {
	return PropertyValidationConfig{
		StrictMode:     true,
		AutoFix:        true,
		VerboseLogging: true,
		MaterialRules: MaterialValidationRules{
			EnableIntelligentMatch: true,
			KeywordMapping: map[string][]string{
				"steel":   {"钢", "不锈钢", "金属", "steel", "stainless"},
				"plastic": {"塑料", "PP", "ABS", "plastic", "polymer"},
				"wood":    {"木", "木质", "竹", "wood", "bamboo"},
				"glass":   {"玻璃", "glass"},
				"fabric":  {"布", "纺织", "fabric", "textile", "cloth"},
				"leather": {"皮", "皮革", "leather"},
				"ceramic": {"陶瓷", "ceramic"},
				"rubber":  {"橡胶", "rubber"},
				"metal":   {"金属", "metal", "铁", "铝", "铜"},
				"cotton":  {"棉", "cotton", "棉质"},
				"silk":    {"丝", "silk", "丝绸"},
			},
		},
		ColorRules: ColorValidationRules{
			EnableIntelligentMatch: true,
			KeywordMapping: map[string][]string{
				"black":  {"黑", "black"},
				"white":  {"白", "white"},
				"red":    {"红", "red"},
				"blue":   {"蓝", "blue"},
				"green":  {"绿", "green"},
				"yellow": {"黄", "yellow"},
				"gray":   {"灰", "gray", "grey"},
				"brown":  {"棕", "brown"},
				"pink":   {"粉", "pink"},
				"purple": {"紫", "purple"},
				"orange": {"橙", "orange"},
				"silver": {"银", "silver"},
				"gold":   {"金", "gold"},
			},
		},
	}
}
