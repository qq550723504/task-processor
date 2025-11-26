package handlers

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

// ProductDescriptionValidator 产品描述验证器
type ProductDescriptionValidator struct {
	logger *logrus.Entry
}

// DescriptionValidationResult 描述验证结果
type DescriptionValidationResult struct {
	OriginalDescription  string   `json:"original_description"`
	ValidatedDescription string   `json:"validated_description"`
	Violations           []string `json:"violations"`
	Suggestions          []string `json:"suggestions"`
	Length               int      `json:"length"`
	IsValid              bool     `json:"is_valid"`
	QualityScore         int      `json:"quality_score"`
}

// NewProductDescriptionValidator 创建新的产品描述验证器
func NewProductDescriptionValidator() *ProductDescriptionValidator {
	return &ProductDescriptionValidator{
		logger: logrus.WithField("handler", "ProductDescriptionValidator"),
	}
}

// Name 返回处理器名称
func (h *ProductDescriptionValidator) Name() string {
	return "产品描述验证处理器"
}

// Handle 处理任务
func (h *ProductDescriptionValidator) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始验证和优化产品描述")

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	originalDesc := ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc
	if originalDesc == "" {
		h.logger.Info("未找到产品描述，生成默认描述")
		defaultDesc := h.generateDefaultDescription(ctx)
		ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc = defaultDesc
		h.logger.Infof("生成了默认描述，长度: %d字符", len(defaultDesc))
		return nil
	}

	// 验证和优化描述
	result := h.validateAndOptimizeDescription(originalDesc, ctx)

	// 记录处理结果
	if len(result.Violations) > 0 {
		h.logger.Warnf("产品描述存在违规内容: %v", result.Violations)
	}
	if len(result.Suggestions) > 0 {
		h.logger.Infof("产品描述优化建议: %v", result.Suggestions)
	}

	if originalDesc != result.ValidatedDescription {
		h.logger.Infof("原始描述长度: %d字符", len(originalDesc))
		h.logger.Infof("优化后描述长度: %d字符", len(result.ValidatedDescription))
		h.logger.Infof("质量评分: %d/100", result.QualityScore)
	}

	// 更新产品描述
	ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc = result.ValidatedDescription

	h.logger.Infof("产品描述验证完成: 长度=%d字符, 质量评分=%d/100",
		result.Length, result.QualityScore)
	return nil
}

// validateAndOptimizeDescription 验证和优化产品描述
func (h *ProductDescriptionValidator) validateAndOptimizeDescription(description string, ctx *pipeline.TaskContext) *DescriptionValidationResult {
	result := &DescriptionValidationResult{
		OriginalDescription:  description,
		ValidatedDescription: description,
		Violations:           []string{},
		Suggestions:          []string{},
		IsValid:              true,
	}

	// 1. 清理和格式化
	cleaned := h.cleanAndFormatDescription(description, result)
	result.ValidatedDescription = cleaned

	// 2. 验证字符支持
	validated := h.validateCharacterSupport(cleaned, result)
	result.ValidatedDescription = validated

	// 3. 验证长度限制
	if len(validated) > 10000 {
		result.Violations = append(result.Violations, fmt.Sprintf("描述长度超过限制: %d > 10000字符", len(validated)))
		result.ValidatedDescription = h.truncateDescription(validated, 10000)
	}

	// 4. 增强描述内容
	enhanced := h.enhanceDescription(result.ValidatedDescription, ctx, result)
	result.ValidatedDescription = enhanced

	// 5. 计算质量评分
	result.QualityScore = h.calculateQualityScore(result.ValidatedDescription, ctx)

	// 6. 设置最终状态
	result.Length = len(result.ValidatedDescription)
	result.IsValid = len(result.Violations) == 0

	return result
}

// cleanAndFormatDescription 清理和格式化描述
func (h *ProductDescriptionValidator) cleanAndFormatDescription(description string, result *DescriptionValidationResult) string {
	// 移除首尾空格
	cleaned := strings.TrimSpace(description)

	// 移除富文本标签（HTML标签等）
	htmlTagPattern := regexp.MustCompile(`<[^>]*>`)
	if htmlTagPattern.MatchString(cleaned) {
		result.Violations = append(result.Violations, "包含不支持的富文本标签")
		cleaned = htmlTagPattern.ReplaceAllString(cleaned, "")
	}

	// 清理多余的空行和空格
	cleaned = h.normalizeWhitespace(cleaned)

	// 移除重复的句子
	cleaned = h.removeDuplicateSentences(cleaned, result)

	return cleaned
}

// validateCharacterSupport 验证字符支持
func (h *ProductDescriptionValidator) validateCharacterSupport(description string, result *DescriptionValidationResult) string {
	var validatedBuilder strings.Builder
	hasInvalidChars := false

	for i, r := range description {
		// 支持字母、数字、符号，但不支持富文本
		if h.isValidChar(r) {
			validatedBuilder.WriteRune(r)
		} else {
			hasInvalidChars = true
			// 对一些特殊字符进行转换
			if converted := h.convertSpecialChar(r); converted != "" {
				validatedBuilder.WriteString(converted)
			} else {
				// 记录被移除的字符（用于调试）
				if r == '.' {
					h.logger.Warnf("⚠️ 小数点在位置%d被移除: 前文=%s", i, description[max(0, i-5):min(len(description), i+5)])
				}
			}
		}
	}

	if hasInvalidChars {
		result.Violations = append(result.Violations, "包含不支持的字符（已转换或移除）")
	}

	return validatedBuilder.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// isValidChar 检查字符是否有效（不包括中文）
func (h *ProductDescriptionValidator) isValidChar(r rune) bool {
	// 跳过中文字符
	if r >= 0x4e00 && r <= 0x9fff {
		return false
	}

	// 支持英文字母、数字、空格和基本符号（仅ASCII范围）
	// 特别注意：小数点(.)必须保留
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
		unicode.IsSpace(r) || r == '.' || strings.ContainsRune(",!?()-[]/:;\"'&%@+=*#$^", r) ||
		r == '\n' || r == '\r' || r == '\t'
}

// convertSpecialChar 转换特殊字符
func (h *ProductDescriptionValidator) convertSpecialChar(r rune) string {
	switch r {
	case '®':
		return "(R)"
	case '©':
		return "(C)"
	case '™':
		return "(TM)"
	case '°':
		return " degrees"
	case '×':
		return "x"
	case '÷':
		return "/"
	case '\u2013': // en dash
		return "-"
	case '\u2014': // em dash
		return "-"
	case '\u201C': // left double quotation mark
		return "\""
	case '\u201D': // right double quotation mark
		return "\""
	case '\u2018': // left single quotation mark
		return "'"
	case '\u2019': // right single quotation mark
		return "'"
	default:
		return ""
	}
}

// normalizeWhitespace 规范化空白字符
func (h *ProductDescriptionValidator) normalizeWhitespace(text string) string {
	// 将多个连续空格替换为单个空格
	spacePattern := regexp.MustCompile(`[ \t]+`)
	text = spacePattern.ReplaceAllString(text, " ")

	// 移除逗号前的空格（TEMU要求：逗号前不能有空格）
	text = regexp.MustCompile(`\s+,`).ReplaceAllString(text, ",")

	// 移除其他标点符号前的空格
	text = regexp.MustCompile(`\s+([.!?;:])`).ReplaceAllString(text, "$1")

	// 确保左括号前有空格（TEMU要求：左括号前必须有空格）
	text = regexp.MustCompile(`(\S)\(`).ReplaceAllString(text, "$1 (")

	// 确保右括号后有空格（如果后面还有字符的话）
	text = regexp.MustCompile(`\)(\S)`).ReplaceAllString(text, ") $1")

	// 将多个连续换行替换为最多两个换行
	newlinePattern := regexp.MustCompile(`\n{3,}`)
	text = newlinePattern.ReplaceAllString(text, "\n\n")

	// 移除行首行尾空格
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	text = strings.Join(lines, "\n")

	return strings.TrimSpace(text)
}

// removeDuplicateSentences 移除重复句子
func (h *ProductDescriptionValidator) removeDuplicateSentences(text string, result *DescriptionValidationResult) string {
	sentences := h.splitIntoSentences(text)
	seen := make(map[string]bool)
	var uniqueSentences []string
	duplicateCount := 0

	for _, sentence := range sentences {
		normalized := strings.ToLower(strings.TrimSpace(sentence))
		if normalized == "" {
			continue
		}

		if !seen[normalized] {
			seen[normalized] = true
			uniqueSentences = append(uniqueSentences, sentence)
		} else {
			duplicateCount++
		}
	}

	if duplicateCount > 0 {
		result.Suggestions = append(result.Suggestions, fmt.Sprintf("移除了%d个重复句子", duplicateCount))
	}

	return strings.Join(uniqueSentences, " ")
}

// splitIntoSentences 将文本分割为句子
func (h *ProductDescriptionValidator) splitIntoSentences(text string) []string {
	// 改进的句子分割：只在句号/问号/感叹号后面有空格或结尾时才分割
	// 这样可以保留小数点（如 35.6）
	sentencePattern := regexp.MustCompile(`[.!?]+\s+|[.!?]+$`)
	sentences := sentencePattern.Split(text, -1)

	var result []string
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence != "" {
			result = append(result, sentence)
		}
	}

	return result
}

// enhanceDescription 增强描述内容
func (h *ProductDescriptionValidator) enhanceDescription(description string, ctx *pipeline.TaskContext, result *DescriptionValidationResult) string {
	// 如果描述过短，尝试扩展
	if len(description) < 200 {
		result.Suggestions = append(result.Suggestions, "描述过短，建议添加更多产品特性和用途说明")
		enhanced := h.expandDescription(description, ctx)
		if enhanced != description {
			result.Suggestions = append(result.Suggestions, "自动扩展了产品描述")
			return enhanced
		}
	}

	// 优化描述结构
	structured := h.structureDescription(description, result)

	return structured
}

// expandDescription 扩展描述
func (h *ProductDescriptionValidator) expandDescription(description string, ctx *pipeline.TaskContext) string {
	bulletPoints := ctx.TemuProduct.GoodsExtensionInfo.BulletPoints

	var expanded strings.Builder
	expanded.WriteString(description)

	// 如果有要点，基于要点扩展描述
	if len(bulletPoints) > 0 {
		expanded.WriteString("\n\nKey Features:\n")
		for _, point := range bulletPoints {
			expanded.WriteString("• ")
			expanded.WriteString(point)
			expanded.WriteString("\n")
		}
	}

	// 添加通用的产品说明（适用于所有品类）
	if len(description) < 100 && len(bulletPoints) == 0 {
		expanded.WriteString("\n\nThis product is designed with quality and functionality in mind. ")
		expanded.WriteString("Suitable for various applications and built to meet your needs.")
	}

	return expanded.String()
}

// structureDescription 结构化描述
func (h *ProductDescriptionValidator) structureDescription(description string, result *DescriptionValidationResult) string {
	// 检查是否已经有良好的结构
	if h.hasGoodStructure(description) {
		return description
	}

	// 尝试添加结构
	structured := h.addStructureToDescription(description)
	if structured != description {
		result.Suggestions = append(result.Suggestions, "优化了描述结构")
	}

	return structured
}

// hasGoodStructure 检查是否有良好的结构
func (h *ProductDescriptionValidator) hasGoodStructure(description string) bool {
	// 检查是否有段落分隔
	paragraphs := strings.Split(description, "\n\n")
	if len(paragraphs) >= 2 {
		return true
	}

	// 检查是否有列表结构
	if strings.Contains(description, "•") || strings.Contains(description, "-") {
		return true
	}

	return false
}

// addStructureToDescription 为描述添加结构
func (h *ProductDescriptionValidator) addStructureToDescription(description string) string {
	sentences := h.splitIntoSentences(description)
	if len(sentences) <= 2 {
		return description
	}

	var structured strings.Builder

	// 第一句作为开头
	if len(sentences) > 0 {
		structured.WriteString(sentences[0])
		structured.WriteString(".\n\n")
	}

	// 中间句子作为特性描述
	if len(sentences) > 2 {
		structured.WriteString("Features & Benefits:\n")
		for i := 1; i < len(sentences)-1 && i < 4; i++ {
			structured.WriteString("• ")
			structured.WriteString(sentences[i])
			structured.WriteString(".\n")
		}
		structured.WriteString("\n")
	}

	// 最后一句作为总结
	if len(sentences) > 1 {
		structured.WriteString(sentences[len(sentences)-1])
		structured.WriteString(".")
	}

	return structured.String()
}

// truncateDescription 截断描述
func (h *ProductDescriptionValidator) truncateDescription(description string, maxLength int) string {
	if len(description) <= maxLength {
		return description
	}

	// 在句子边界截断
	sentences := h.splitIntoSentences(description)
	var result strings.Builder

	for _, sentence := range sentences {
		if result.Len()+len(sentence)+1 <= maxLength-3 { // 为"..."预留空间
			if result.Len() > 0 {
				result.WriteString(" ")
			}
			result.WriteString(sentence)
			result.WriteString(".")
		} else {
			break
		}
	}

	if result.Len() < len(description) {
		result.WriteString("...")
	}

	return result.String()
}

// calculateQualityScore 计算质量评分
func (h *ProductDescriptionValidator) calculateQualityScore(description string, ctx *pipeline.TaskContext) int {
	score := 0

	// 长度评分 (0-25分)
	length := len(description)
	if length >= 500 && length <= 2000 {
		score += 25
	} else if length >= 200 && length <= 500 {
		score += 20
	} else if length >= 100 && length <= 200 {
		score += 15
	} else if length > 0 {
		score += 10
	}

	// 结构评分 (0-25分)
	if h.hasGoodStructure(description) {
		score += 25
	} else if strings.Contains(description, "\n") {
		score += 15
	} else {
		score += 5
	}

	// 内容丰富度评分 (0-25分)
	sentences := h.splitIntoSentences(description)
	if len(sentences) >= 5 {
		score += 25
	} else if len(sentences) >= 3 {
		score += 20
	} else if len(sentences) >= 2 {
		score += 15
	} else {
		score += 10
	}

	// 关键词覆盖评分 (0-25分)
	keywordScore := h.calculateKeywordScore(description, ctx)
	score += keywordScore

	// 确保评分在0-100范围内
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return score
}

// calculateKeywordScore 计算关键词评分
func (h *ProductDescriptionValidator) calculateKeywordScore(description string, ctx *pipeline.TaskContext) int {
	descLower := strings.ToLower(description)
	productName := strings.ToLower(ctx.TemuProduct.GoodsBasic.GoodsName)

	score := 0

	// 检查产品名称中的关键词
	nameWords := strings.Fields(productName)
	for _, word := range nameWords {
		if len(word) > 3 && strings.Contains(descLower, word) {
			score += 3
		}
	}

	// 检查通用质量词汇
	qualityWords := []string{
		"quality", "durable", "comfortable", "ergonomic", "premium",
		"professional", "reliable", "efficient", "easy", "convenient",
	}
	for _, word := range qualityWords {
		if strings.Contains(descLower, word) {
			score += 2
		}
	}

	// 限制最大评分
	if score > 25 {
		score = 25
	}

	return score
}

// generateDefaultDescription 生成默认描述
func (h *ProductDescriptionValidator) generateDefaultDescription(ctx *pipeline.TaskContext) string {
	productName := ctx.TemuProduct.GoodsBasic.GoodsName
	bulletPoints := ctx.TemuProduct.GoodsExtensionInfo.BulletPoints

	var desc strings.Builder

	// 基于产品名称生成通用开头
	desc.WriteString(fmt.Sprintf("Introducing the %s, ", productName))
	desc.WriteString("a high-quality product designed to meet your needs. ")
	desc.WriteString("Crafted with attention to detail and built to last.")

	// 添加要点作为特性说明
	if len(bulletPoints) > 0 {
		desc.WriteString("\n\nKey Features:\n")
		for _, point := range bulletPoints {
			desc.WriteString("• ")
			desc.WriteString(point)
			desc.WriteString("\n")
		}
	}

	// 添加通用结尾
	desc.WriteString("\nPerfect for both personal and professional use, ")
	desc.WriteString("this product offers excellent value and reliable performance.")

	return desc.String()
}

// ValidateDescriptionAPI 调用TEMU API验证描述（如果需要）
func (h *ProductDescriptionValidator) ValidateDescriptionAPI(ctx *pipeline.TaskContext, description string) error {
	// 这里可以调用TEMU的违规词汇检查API
	// temu.local.goods.illegal.vocabulary.check

	h.logger.Debugf("TODO: 调用TEMU API验证产品描述，长度: %d字符", len(description))

	// 暂时返回nil，实际实现时需要调用真实的API
	return nil
}
