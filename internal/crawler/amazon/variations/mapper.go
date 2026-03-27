package variations

import (
	"task-processor/internal/core/logger"
	"fmt"
	"regexp"
	"strings"

)

// Mapper 属性映射器
type Mapper struct {
	config *Config
}

// NewMapper 创建属性映射器
func NewMapper(config *Config) *Mapper {
	return &Mapper{config: config}
}

// MapAttributeNames 将通用属性名映射为语义化名称
func (m *Mapper) MapAttributeNames(attributes map[string]any) map[string]any {
	mapped := make(map[string]any)

	for key, value := range attributes {
		var finalKey string

		// 首先检查是否有预定义的映射配置
		if mappedName, exists := m.config.AttributeMapping[key]; exists {
			finalKey = mappedName
			if m.config.EnableDebugLogging {
				logger.GetGlobalLogger("crawler/amazon").Infof("Mapped attribute: %s -> %s", key, finalKey)
			}
		} else if m.config.EnableSmartInference {
			// 如果启用了智能推断，尝试基于值推断属性类型
			inferredType := m.InferAttributeType(value)

			// 对于attribute_N或variant_*格式的键，使用推断的类型
			if strings.HasPrefix(key, "attribute_") || strings.HasPrefix(key, "variant_") {
				finalKey = inferredType
				if m.config.EnableDebugLogging {
					logger.GetGlobalLogger("crawler/amazon").Infof("Inferred attribute type: %s -> %s (value: %v)", key, finalKey, value)
				}
			} else {
				// 对于其他键名，保持原样
				finalKey = key
			}
		} else {
			// 如果没有映射且未启用智能推断，保持原键名
			finalKey = key
		}

		mapped[finalKey] = value
	}

	return mapped
}

// InferAttributeType 基于属性值内容推断属性类型
func (m *Mapper) InferAttributeType(value any) string {
	if value == nil {
		return "unknown"
	}

	valueStr := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", value)))

	// 颜色检测
	if m.isColor(valueStr) {
		return "color"
	}

	// 尺寸检测
	if m.isSize(valueStr) {
		return "size"
	}

	// 材质检测
	if m.isMaterial(valueStr) {
		return "material"
	}

	// 样式检测
	if m.isStyle(valueStr) {
		return "style"
	}

	// 数量检测
	if m.isQuantity(valueStr) {
		return "item_package_quantity"
	}

	// 品牌检测
	if m.isBrand(valueStr) {
		return "brand"
	}

	// 如果都不匹配，返回通用变体类型
	return "variant"
}

// isColor 检测是否为颜色
func (m *Mapper) isColor(valueStr string) bool {
	colorKeywords := []string{
		"black", "white", "red", "blue", "green", "yellow", "orange", "purple", "pink", "brown",
		"gray", "grey", "silver", "gold", "beige", "navy", "maroon", "olive", "lime", "aqua",
		"teal", "fuchsia", "violet", "indigo", "turquoise", "coral", "salmon", "khaki", "cream",
		"ivory", "charcoal", "burgundy", "magenta", "cyan", "amber", "bronze", "copper", "platinum",
		"黑色", "白色", "红色", "蓝色", "绿色", "黄色", "橙色", "紫色", "粉色", "棕色",
		"灰色", "银色", "金色", "米色", "深蓝", "栗色", "橄榄", "青色", "蓝绿", "紫红",
	}

	for _, color := range colorKeywords {
		if strings.Contains(valueStr, color) {
			return true
		}
	}

	// 颜色模式匹配
	colorPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\b(light|dark|deep|bright|pale)\s+\w+`),
		regexp.MustCompile(`\w+\s+(color|colored)`),
		regexp.MustCompile(`#[0-9a-f]{3,6}`),
	}

	for _, pattern := range colorPatterns {
		if pattern.MatchString(valueStr) {
			return true
		}
	}

	return false
}

// isSize 检测是否为尺寸
func (m *Mapper) isSize(valueStr string) bool {
	sizePatterns := []string{
		"xs", "s", "m", "l", "xl", "xxl", "xxxl", "xxxxl",
		"small", "medium", "large", "extra large", "extra small",
		"tiny", "mini", "big", "huge", "petite", "plus", "regular",
		"小号", "中号", "大号", "特大号", "加大", "超大",
	}

	for _, size := range sizePatterns {
		if strings.Contains(valueStr, size) {
			return true
		}
	}

	// 数字尺寸模式
	sizeNumPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\d+\s*(cm|mm|inch|in|ft|meter|m|"|')\b`),
		regexp.MustCompile(`\b\d+(\.\d+)?\s*x\s*\d+(\.\d+)?`),
		regexp.MustCompile(`\b(size|sz)\s*:?\s*\d+`),
		regexp.MustCompile(`\b\d+\s*(us|uk|eu|cn)\b`),
		regexp.MustCompile(`\b\d+[a-z]*\s*(wide|narrow|regular)\b`),
	}

	for _, pattern := range sizeNumPatterns {
		if pattern.MatchString(valueStr) {
			return true
		}
	}

	return false
}

// isMaterial 检测是否为材质
func (m *Mapper) isMaterial(valueStr string) bool {
	materialKeywords := []string{
		"cotton", "polyester", "wool", "silk", "leather", "denim", "canvas", "linen",
		"nylon", "spandex", "bamboo", "cashmere", "velvet", "satin", "chiffon",
		"plastic", "metal", "wood", "glass", "ceramic", "rubber", "foam", "mesh",
		"microfiber", "fleece", "jersey", "twill", "corduroy", "suede", "vinyl",
		"neoprene", "latex", "silicone", "eva", "pu", "pvc", "tpu",
		"棉", "聚酯", "羊毛", "丝绸", "皮革", "牛仔", "帆布", "亚麻",
		"尼龙", "氨纶", "竹纤维", "羊绒", "天鹅绒", "缎子", "塑料", "金属",
		"氯丁橡胶", "乳胶", "硅胶", "EVA", "聚氨酯", "PVC", "TPU",
	}

	for _, material := range materialKeywords {
		if strings.Contains(valueStr, material) {
			return true
		}
	}

	return false
}

// isStyle 检测是否为样式
func (m *Mapper) isStyle(valueStr string) bool {
	styleKeywords := []string{
		"classic", "modern", "vintage", "casual", "formal", "sporty", "elegant",
		"minimalist", "bohemian", "retro", "contemporary", "traditional", "trendy",
		"chic", "sophisticated", "rustic", "industrial", "artistic", "luxury",
		"经典", "现代", "复古", "休闲", "正式", "运动", "优雅",
		"简约", "波西米亚", "怀旧", "当代", "传统", "时尚",
	}

	for _, style := range styleKeywords {
		if strings.Contains(valueStr, style) {
			return true
		}
	}

	return false
}

// isQuantity 检测是否为数量
func (m *Mapper) isQuantity(valueStr string) bool {
	quantityPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\b\d+\s*(pack|pcs?|pieces?|count|ct)\b`),
		regexp.MustCompile(`\b(pack\s+of|set\s+of)\s+\d+`),
		regexp.MustCompile(`\b\d+\s*-?\s*(unit|item)s?\b`),
	}

	for _, pattern := range quantityPatterns {
		if pattern.MatchString(valueStr) {
			return true
		}
	}

	return false
}

// isBrand 检测是否为品牌
func (m *Mapper) isBrand(valueStr string) bool {
	return strings.Contains(valueStr, "brand") || strings.Contains(valueStr, "by ")
}

// NormalizeKey 标准化变体键名
func (m *Mapper) NormalizeKey(key string) string {
	if normalized, exists := m.config.AttributeMapping[key]; exists {
		return normalized
	}
	return key
}
