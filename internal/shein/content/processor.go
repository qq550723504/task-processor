// Package content 提供SHEIN平台的敏感词文本处理功能
package content

import (
	"regexp"
	"strings"
	"unicode"

	"task-processor/internal/shein/api/product"
	sheinctx "task-processor/internal/shein/context"
)

// removeSensitiveWords 移除文本中的敏感词
func (s *SensitiveWordService) removeSensitiveWords(text string) string {
	if text == "" {
		return text
	}

	// 不对原始文本做 preprocess，直接在原文上做大小写不敏感替换，
	// 避免 normalizeSpecialCharacters 把连字符（如 Non-toxic）转成空格后导致匹配失败
	allWords := s.getAllSensitiveWords()

	for _, word := range allWords {
		if word != "" {
			text = s.removeWordFromText(text, word)
		}
	}

	text = s.cleanupText(text)
	return text
}

// removeSensitiveWordsAndBrandsWithContext 移除文本中的敏感词、Amazon品牌词和上下文中的品牌词
func (s *SensitiveWordService) removeSensitiveWordsAndBrandsWithContext(ctx *sheinctx.TaskContext, text string) string {
	if text == "" {
		return text
	}

	// 1. 移除敏感词
	text = s.removeSensitiveWords(text)

	// 2. 移除Amazon品牌词
	text = s.removeAmazonBrandWords(text)

	// 3. 移除上下文中的品牌词
	text = s.removeContextBrandWords(ctx, text)

	// 4. 为SHEIN平台清理文本
	text = s.cleanTextForSheinPlatform(text)

	return text
}

// SanitizeTextWithContext applies the full SHEIN-sensitive cleanup pipeline to a single text field.
func (s *SensitiveWordService) SanitizeTextWithContext(ctx *sheinctx.TaskContext, text string) string {
	return s.removeSensitiveWordsAndBrandsWithContext(ctx, text)
}

// SanitizeDisplayTextWithContext removes sensitive and brand words while preserving
// the source text's case and punctuation as much as possible for preview/draft copy.
func (s *SensitiveWordService) SanitizeDisplayTextWithContext(ctx *sheinctx.TaskContext, text string) string {
	if text == "" {
		return text
	}

	text = s.removeSensitiveWords(text)
	text = s.removeAmazonBrandWords(text)
	text = s.removeContextBrandWords(ctx, text)
	return s.cleanupText(text)
}

// processMultiLanguageNames 处理多语言名称
func (s *SensitiveWordService) processMultiLanguageNames(nameList []product.LanguageContent) int {
	if nameList == nil {
		return 0
	}

	processedCount := 0
	for i, name := range nameList {
		if cleaned := s.removeSensitiveWords(name.Name); cleaned != name.Name {
			nameList[i].Name = cleaned
			processedCount++
		}
	}
	return processedCount
}

// processMultiLanguageNamesWithBrandsAndContext 处理多语言名称（包含Amazon品牌词和上下文品牌词移除）
func (s *SensitiveWordService) processMultiLanguageNamesWithBrandsAndContext(ctx *sheinctx.TaskContext, nameList []product.LanguageContent) int {
	if nameList == nil {
		return 0
	}

	processedCount := 0
	for i, name := range nameList {
		if cleaned := s.removeSensitiveWordsAndBrandsWithContext(ctx, name.Name); cleaned != name.Name {
			nameList[i].Name = cleaned
			processedCount++
		}
	}
	return processedCount
}

// processMultiLanguageDescs 处理多语言描述
func (s *SensitiveWordService) processMultiLanguageDescs(descList []product.LanguageContent) int {
	if descList == nil {
		return 0
	}

	processedCount := 0
	for i, desc := range descList {
		if cleaned := s.removeSensitiveWords(desc.Name); cleaned != desc.Name {
			descList[i].Name = cleaned
			processedCount++
		}
	}
	return processedCount
}

// processSKCData 处理SKC数据
func (s *SensitiveWordService) processSKCData(ctx *sheinctx.TaskContext, skcList []product.SKC) int {
	if skcList == nil {
		return 0
	}

	processedCount := 0
	for i, skc := range skcList {
		// 处理SKC多语言名称（包含Amazon品牌词移除）
		if cleaned := s.removeSensitiveWordsAndBrandsWithContext(ctx, skc.MultiLanguageName.Name); cleaned != skc.MultiLanguageName.Name {
			skcList[i].MultiLanguageName.Name = cleaned
			processedCount++
		}

		// 处理SKC多语言名称列表（包含Amazon品牌词移除）
		if skc.MultiLanguageNameList != nil {
			processedCount += s.processMultiLanguageNamesWithBrandsAndContext(ctx, skc.MultiLanguageNameList)
		}
	}
	return processedCount
}

// processProductAttributes cleans only free-text attribute extra values and leaves structured values intact.
func (s *SensitiveWordService) processProductAttributes(ctx *sheinctx.TaskContext, attrs []product.ProductAttribute) int {
	if attrs == nil {
		return 0
	}

	processedCount := 0
	for i := range attrs {
		value := attrs[i].AttributeExtraValue
		if !shouldSanitizeFreeTextAttributeValue(value) {
			continue
		}
		if cleaned := s.removeSensitiveWordsAndBrandsWithContext(ctx, value); cleaned != value {
			attrs[i].AttributeExtraValue = cleaned
			processedCount++
		}
	}
	return processedCount
}

func shouldSanitizeFreeTextAttributeValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if len(value) >= 3 && strings.ContainsAny(value, " \t\r\n") {
		return true
	}
	hasLetter := false
	for _, r := range value {
		if unicode.IsLetter(r) {
			hasLetter = true
			continue
		}
		if unicode.IsDigit(r) || unicode.IsSpace(r) {
			continue
		}
		switch r {
		case '.', ',', '-', '_', '/', '\\', '+', '%', 'x', 'X':
			continue
		default:
			return hasLetter
		}
	}
	return false
}

// cleanTextForSheinPlatform 为SHEIN平台清理文本
func (s *SensitiveWordService) cleanTextForSheinPlatform(text string) string {
	if text == "" {
		return text
	}

	// 移除所有表情符号
	text = s.removeAllEmojisAggressively(text)

	// 移除特殊字符和符号
	text = s.normalizeSpecialCharacters(text)

	// 移除多余的空格和换行
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// 移除开头和结尾的标点符号
	text = strings.Trim(text, ".,!?;:-_()[]{}\"'`~")

	// 确保文本不为空
	if strings.TrimSpace(text) == "" {
		return "Product"
	}

	// 限制长度（SHEIN平台限制）
	if len(text) > 200 {
		text = text[:200]
		// 确保不在单词中间截断
		if lastSpace := strings.LastIndex(text, " "); lastSpace > 150 {
			text = text[:lastSpace]
		}
		text = strings.TrimSpace(text)
	}

	return text
}

// removeAmazonBrandWords 移除Amazon品牌词
func (s *SensitiveWordService) removeAmazonBrandWords(text string) string {
	if text == "" {
		return text
	}

	originalText := text
	cleanedText := text
	amazonBrandWords := s.getAmazonBrandWords()
	removedWords := []string{}

	for _, brandWord := range amazonBrandWords {
		beforeRemoval := cleanedText
		cleanedText = s.removeWordFromText(cleanedText, brandWord)

		// 记录被移除的品牌词
		if beforeRemoval != cleanedText {
			removedWords = append(removedWords, brandWord)
		}
	}

	// 记录品牌词移除统计
	if len(removedWords) > 0 {
		s.logger.Debugf("🏷️ 移除Amazon品牌词: %v", removedWords)
		s.logger.Debugf("🏷️ 品牌词清理: %s -> %s", originalText, cleanedText)
	}

	return cleanedText
}

// removeContextBrandWords 移除上下文中的品牌词（从AmazonProduct.Brand字段）
func (s *SensitiveWordService) removeContextBrandWords(ctx *sheinctx.TaskContext, text string) string {
	if text == "" || ctx == nil || ctx.AmazonProduct == nil {
		return text
	}

	brandWord := strings.TrimSpace(ctx.AmazonProduct.Brand)
	if brandWord == "" {
		return text
	}

	originalText := text
	cleanedText := s.removeWordFromText(text, brandWord)

	// 记录品牌词移除统计
	if originalText != cleanedText {
		s.logger.Debugf("🏷️ 移除上下文品牌词: %s", brandWord)
		s.logger.Debugf("🏷️ 上下文品牌词清理: %s -> %s", originalText, cleanedText)
	}

	return cleanedText
}

// getAmazonBrandWords 获取 Amazon 自有品牌词列表（仅保留真正的品牌名，避免误删正常产品描述）
func (s *SensitiveWordService) getAmazonBrandWords() []string {
	return []string{
		// Amazon 自有品牌
		"Amazon Basics",
		"Amazon Essentials",
		"Amazon",
		"AmazonBasics",
		"AmazonEssentials",
		"Solimo",
		"Goodthreads",
		"Daily Ritual",
		"Core 10",
		"Lark & Ro",
		"28 Palms",
		"Buttoned Down",
		// Amazon 平台专属标识（不会出现在正常产品描述中）
		"Fulfillment by Amazon",
		"Ships from Amazon",
		"Sold by Amazon",
		"Amazon Warehouse",
		"Amazon's Choice",
		"#1 Best Seller",
	}
}

func (s *SensitiveWordService) TestEmojiFiltering(text string) string {
	s.logger.Infof("🧪 测试表情符号过滤:")
	s.logger.Infof("  原文: %s", text)
	filtered := s.filterEmojis(text)
	s.logger.Infof("  过滤后: %s", filtered)
	aggressive := s.removeAllEmojisAggressively(text)
	s.logger.Infof("  激进过滤: %s", aggressive)
	return aggressive
}
