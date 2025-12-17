package handlers

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

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
