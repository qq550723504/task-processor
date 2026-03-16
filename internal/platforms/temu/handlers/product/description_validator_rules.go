// Package product 提供TEMU平台产品描述验证器的规则功能
package product

import (
	"regexp"
	"strings"
	"unicode"
)

// 这个文件包含ProductDescriptionValidator的规则验证功能
// 注意：主要方法定义在 product_description_validator.go 中
// 这里只包含规则相关的辅助功能，避免方法重复定义

// validateDescriptionRules 验证描述规则
func (h *ProductDescriptionValidator) validateDescriptionRules(description string, result *DescriptionValidationResult) {
	// 长度检查
	if len(description) < 10 {
		result.Violations = append(result.Violations, "描述过短，建议至少10个字符")
		result.QualityScore -= 20
	}

	if len(description) > 2000 {
		result.Violations = append(result.Violations, "描述过长，建议不超过2000个字符")
		result.QualityScore -= 10
	}

	// 检查是否包含禁用词汇
	h.checkForbiddenWords(description, result)

	// 检查格式问题
	h.checkFormatIssues(description, result)

	// 检查字符支持
	h.checkCharacterSupport(description, result)
}

// checkForbiddenWords 检查禁用词汇
func (h *ProductDescriptionValidator) checkForbiddenWords(description string, result *DescriptionValidationResult) {
	forbiddenWords := []string{
		"best", "perfect", "amazing", "incredible", "unbelievable",
		"guarantee", "promise", "100%", "free shipping", "discount",
	}

	lowerDesc := strings.ToLower(description)
	for _, word := range forbiddenWords {
		if strings.Contains(lowerDesc, strings.ToLower(word)) {
			result.Violations = append(result.Violations, "包含可能的禁用词汇: "+word)
			result.QualityScore -= 5
		}
	}
}

// checkFormatIssues 检查格式问题
func (h *ProductDescriptionValidator) checkFormatIssues(description string, result *DescriptionValidationResult) {
	// 检查是否全大写
	if description == strings.ToUpper(description) && len(description) > 20 {
		result.Violations = append(result.Violations, "描述全部为大写字母")
		result.QualityScore -= 15
	}

	// 检查重复标点符号
	if matched, _ := regexp.MatchString(`[!]{2,}|[?]{2,}|[.]{3,}`, description); matched {
		result.Violations = append(result.Violations, "包含重复的标点符号")
		result.QualityScore -= 5
	}

	// 检查过多的感叹号
	exclamationCount := strings.Count(description, "!")
	if exclamationCount > 3 {
		result.Violations = append(result.Violations, "感叹号使用过多")
		result.QualityScore -= 5
	}
}

// checkCharacterSupport 检查字符支持
func (h *ProductDescriptionValidator) checkCharacterSupport(description string, result *DescriptionValidationResult) {
	unsupportedChars := make([]rune, 0)

	for _, r := range description {
		// 检查是否为支持的字符
		if !isSupportedCharacter(r) {
			unsupportedChars = append(unsupportedChars, r)
		}
	}

	if len(unsupportedChars) > 0 {
		result.Violations = append(result.Violations, "包含不支持的字符")
		result.QualityScore -= 10
	}
}

// isSupportedCharacter 检查字符是否被支持
func isSupportedCharacter(r rune) bool {
	// 支持基本拉丁字符、数字和常用标点符号
	if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
		return true
	}

	// 支持的标点符号
	supportedPuncts := ".,!?;:()[]{}\"'-_/\\@#$%^&*+=<>|~`"
	return strings.ContainsRune(supportedPuncts, r)
}

// applyDescriptionRules 应用描述规则修复
func (h *ProductDescriptionValidator) applyDescriptionRules(description string) string {
	// 移除多余的空格
	description = regexp.MustCompile(`\s+`).ReplaceAllString(description, " ")
	description = strings.TrimSpace(description)

	// 修复标点符号问题
	description = regexp.MustCompile(`[!]{2,}`).ReplaceAllString(description, "!")
	description = regexp.MustCompile(`[?]{2,}`).ReplaceAllString(description, "?")
	description = regexp.MustCompile(`[.]{3,}`).ReplaceAllString(description, "...")

	// 确保句子以适当的标点符号结尾
	if !strings.HasSuffix(description, ".") && !strings.HasSuffix(description, "!") && !strings.HasSuffix(description, "?") {
		description += "."
	}

	return description
}
