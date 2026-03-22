package product

import (
	"fmt"
	"regexp"
	"strings"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// ProductNameOptimizer 产品名称优化器
type ProductNameOptimizer struct {
	logger *logrus.Entry
}

// NewProductNameOptimizer 创建新的产品名称优化器
func NewProductNameOptimizer() *ProductNameOptimizer {
	return &ProductNameOptimizer{
		logger: logger.GetGlobalLogger("ProductNameOptimizer"),
	}
}

// OptimizeProductName 优化产品名称以提高TEMU平台表现
func (o *ProductNameOptimizer) OptimizeProductName(originalName string) (string, []string) {
	var optimizations []string
	optimized := originalName

	// 1. 移除多余的品牌重复
	optimized = o.removeBrandDuplication(optimized, &optimizations)

	// 2. 优化关键词顺序（将重要关键词前置）
	optimized = o.optimizeKeywordOrder(optimized, &optimizations)

	// 3. 移除冗余词汇
	optimized = o.removeRedundantWords(optimized, &optimizations)

	// 4. 标准化尺寸和颜色描述
	optimized = o.standardizeDescriptions(optimized, &optimizations)

	// 5. 确保关键信息完整
	optimized = o.ensureKeyInformation(optimized, &optimizations)

	// 6. 优化长度（保持在合理范围内）
	optimized = o.optimizeLength(optimized, &optimizations)

	return optimized, optimizations
}

// removeBrandDuplication 移除品牌重复
func (o *ProductNameOptimizer) removeBrandDuplication(name string, optimizations *[]string) string {
	// 处理重复单词 - 使用自定义逻辑而不是反向引用
	result := o.removeRepeatedWords(name, optimizations)

	// 处理特定的重复模式
	patterns := []struct {
		pattern     string
		replacement string
		description string
	}{
		{`\bfor\s+for\b`, "for", "移除重复的for"},
		{`\bwith\s+with\b`, "with", "移除重复的with"},
		{`\band\s+and\b`, "and", "移除重复的and"},
		{`\bthe\s+the\b`, "the", "移除重复的the"},
	}

	for _, p := range patterns {
		re := regexp.MustCompile(`(?i)` + p.pattern)
		if re.MatchString(result) {
			result = re.ReplaceAllString(result, p.replacement)
			*optimizations = append(*optimizations, p.description)
		}
	}

	return result
}

// removeRepeatedWords 移除重复的单词
func (o *ProductNameOptimizer) removeRepeatedWords(name string, optimizations *[]string) string {
	words := strings.Fields(name)
	if len(words) <= 1 {
		return name
	}

	var result []string
	var removed bool

	for i := 0; i < len(words); i++ {
		// 检查当前单词是否与下一个单词相同（忽略大小写）
		if i < len(words)-1 && strings.EqualFold(words[i], words[i+1]) {
			// 跳过重复的单词
			result = append(result, words[i])
			i++ // 跳过下一个重复的单词
			removed = true
		} else {
			result = append(result, words[i])
		}
	}

	if removed {
		*optimizations = append(*optimizations, "移除重复单词")
	}

	return strings.Join(result, " ")
}

// optimizeKeywordOrder 优化关键词顺序
func (o *ProductNameOptimizer) optimizeKeywordOrder(name string, optimizations *[]string) string {
	// 重要关键词列表（按优先级排序）
	importantKeywords := []string{
		"Gaming", "Office", "Ergonomic", "Computer", "PC",
		"Chair", "Desk", "Swivel", "Executive", "Racing",
		"High-Back", "Lumbar Support", "Adjustable",
	}

	words := strings.Fields(name)
	var priorityWords []string
	var otherWords []string

	// 分离重要关键词和其他词汇
	for _, word := range words {
		isImportant := false
		for _, keyword := range importantKeywords {
			if strings.EqualFold(word, keyword) || strings.Contains(strings.ToLower(keyword), strings.ToLower(word)) {
				priorityWords = append(priorityWords, word)
				isImportant = true
				break
			}
		}
		if !isImportant {
			otherWords = append(otherWords, word)
		}
	}

	// 重新组合，重要关键词在前
	if len(priorityWords) > 0 {
		result := strings.Join(append(priorityWords, otherWords...), " ")
		if result != name {
			*optimizations = append(*optimizations, "优化关键词顺序")
			return result
		}
	}

	return name
}

// removeRedundantWords 移除冗余词汇
func (o *ProductNameOptimizer) removeRedundantWords(name string, optimizations *[]string) string {
	// 常见的冗余词汇
	redundantWords := []string{
		"High Quality", "Premium Quality", "Best", "Top", "Perfect",
		"Amazing", "Awesome", "Great", "Excellent", "Superior",
		"Professional", "Commercial", "Industrial", "Heavy Duty",
	}

	result := name
	for _, word := range redundantWords {
		pattern := `\b` + regexp.QuoteMeta(word) + `\b`
		re := regexp.MustCompile(`(?i)` + pattern)
		if re.MatchString(result) {
			result = re.ReplaceAllString(result, "")
			*optimizations = append(*optimizations, fmt.Sprintf("移除冗余词汇: %s", word))
		}
	}

	// 清理多余空格
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	result = strings.TrimSpace(result)

	return result
}

// standardizeDescriptions 标准化描述
func (o *ProductNameOptimizer) standardizeDescriptions(name string, optimizations *[]string) string {
	// 标准化颜色描述
	colorMappings := map[string]string{
		"Black Color": "Black",
		"White Color": "White",
		"Red Color":   "Red",
		"Blue Color":  "Blue",
		"Green Color": "Green",
	}

	result := name
	for old, new := range colorMappings {
		if strings.Contains(strings.ToLower(result), strings.ToLower(old)) {
			re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(old) + `\b`)
			result = re.ReplaceAllString(result, new)
			*optimizations = append(*optimizations, fmt.Sprintf("标准化颜色描述: %s -> %s", old, new))
		}
	}

	// 标准化尺寸描述
	sizeMappings := map[string]string{
		"Large Size":  "Large",
		"Medium Size": "Medium",
		"Small Size":  "Small",
		"Extra Large": "XL",
		"Extra Small": "XS",
	}

	for old, new := range sizeMappings {
		if strings.Contains(strings.ToLower(result), strings.ToLower(old)) {
			re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(old) + `\b`)
			result = re.ReplaceAllString(result, new)
			*optimizations = append(*optimizations, fmt.Sprintf("标准化尺寸描述: %s -> %s", old, new))
		}
	}

	return result
}

// ensureKeyInformation 确保关键信息完整
func (o *ProductNameOptimizer) ensureKeyInformation(name string, optimizations *[]string) string {
	result := name

	// 检查是否包含产品类型
	productTypes := []string{"Chair", "Desk", "Table", "Stool", "Bench"}
	hasProductType := false
	for _, pType := range productTypes {
		if strings.Contains(strings.ToLower(result), strings.ToLower(pType)) {
			hasProductType = true
			break
		}
	}

	// 如果没有产品类型，尝试从上下文推断
	if !hasProductType {
		if strings.Contains(strings.ToLower(result), "gaming") ||
			strings.Contains(strings.ToLower(result), "office") ||
			strings.Contains(strings.ToLower(result), "ergonomic") {
			result = result + " Chair"
			*optimizations = append(*optimizations, "添加产品类型: Chair")
		}
	}

	return result
}

// optimizeLength 优化长度
func (o *ProductNameOptimizer) optimizeLength(name string, optimizations *[]string) string {
	// TEMU推荐的产品名称长度范围
	minLength := 20
	maxLength := 200

	if len(name) < minLength {
		*optimizations = append(*optimizations, fmt.Sprintf("产品名称过短 (%d字符)，建议增加描述", len(name)))
	} else if len(name) > maxLength {
		// 智能截断，保留重要信息
		words := strings.Fields(name)
		result := ""
		for _, word := range words {
			if len(result+" "+word) <= maxLength {
				if result == "" {
					result = word
				} else {
					result += " " + word
				}
			} else {
				break
			}
		}
		*optimizations = append(*optimizations, fmt.Sprintf("截断过长名称: %d -> %d字符", len(name), len(result)))
		return result
	}

	return name
}

// GetOptimizationSuggestions 获取优化建议
func (o *ProductNameOptimizer) GetOptimizationSuggestions(name string) []string {
	var suggestions []string

	// 检查长度
	if len(name) < 20 {
		suggestions = append(suggestions, "建议增加产品描述，提高搜索可见性")
	}
	if len(name) > 200 {
		suggestions = append(suggestions, "建议缩短产品名称，提高可读性")
	}

	// 检查关键词
	if !strings.Contains(strings.ToLower(name), "chair") &&
		!strings.Contains(strings.ToLower(name), "desk") {
		suggestions = append(suggestions, "建议明确产品类型（如Chair、Desk等）")
	}

	// 检查特性描述
	features := []string{"ergonomic", "adjustable", "comfortable", "durable"}
	hasFeature := false
	for _, feature := range features {
		if strings.Contains(strings.ToLower(name), feature) {
			hasFeature = true
			break
		}
	}
	if !hasFeature {
		suggestions = append(suggestions, "建议添加产品特性描述（如Ergonomic、Adjustable等）")
	}

	return suggestions
}
