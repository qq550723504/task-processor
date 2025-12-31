// Package modules 提供SHEIN平台的敏感词文本处理功能
package modules

import (
	"regexp"
	"strings"
	"task-processor/internal/platforms/shein/api/product"

	"github.com/sirupsen/logrus"
)

// removeSensitiveWords 移除文本中的敏感词
func (s *SensitiveWordService) removeSensitiveWords(text string) string {
	if text == "" {
		return text
	}

	text = s.preprocessText(text)
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
func (s *SensitiveWordService) removeSensitiveWordsAndBrandsWithContext(ctx *TaskContext, text string) string {
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
func (s *SensitiveWordService) processMultiLanguageNamesWithBrandsAndContext(ctx *TaskContext, nameList []product.LanguageContent) int {
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
func (s *SensitiveWordService) processSKCData(ctx *TaskContext, skcList []product.SKC) int {
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
		logrus.Debugf("🏷️ 移除Amazon品牌词: %v", removedWords)
		logrus.Debugf("🏷️ 品牌词清理: %s -> %s", originalText, cleanedText)
	}

	return cleanedText
}

// removeContextBrandWords 移除上下文中的品牌词（从AmazonProduct.Brand字段）
func (s *SensitiveWordService) removeContextBrandWords(ctx *TaskContext, text string) string {
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
		logrus.Debugf("🏷️ 移除上下文品牌词: %s", brandWord)
		logrus.Debugf("🏷️ 上下文品牌词清理: %s -> %s", originalText, cleanedText)
	}

	return cleanedText
}

// getAmazonBrandWords 获取Amazon品牌词列表
func (s *SensitiveWordService) getAmazonBrandWords() []string {
	return []string{
		// Amazon自有品牌
		"Amazon", "amazon", "AMAZON",
		"Amazon Basics", "AmazonBasics", "Amazon basics",
		"Amazon Essentials", "AmazonEssentials", "Amazon essentials",
		"Amazon Choice", "Amazon's Choice", "Amazon choice",
		"Solimo", "SOLIMO", "solimo",
		"Goodthreads", "GOODTHREADS", "goodthreads",
		"Daily Ritual", "DAILY RITUAL", "daily ritual",
		"Core 10", "CORE 10", "core 10",
		"Lark & Ro", "LARK & RO", "lark & ro",
		"28 Palms", "28 PALMS", "28 palms",
		"Buttoned Down", "BUTTONED DOWN", "buttoned down",
		"Brand - ", "Brand: ", "brand - ", "brand: ",

		// 常见的Amazon产品标识词
		"Prime", "PRIME", "prime",
		"Prime Eligible", "Prime eligible", "prime eligible",
		"Free Shipping", "FREE SHIPPING", "free shipping",
		"Best Seller", "BEST SELLER", "best seller",
		"#1 Best Seller", "#1 BEST SELLER", "#1 best seller",
		"Amazon's", "amazon's", "AMAZON'S",

		// 其他Amazon相关词汇
		"Fulfillment by Amazon", "FBA", "fba",
		"Ships from Amazon", "ships from amazon",
		"Sold by Amazon", "sold by amazon",
		"Amazon Warehouse", "amazon warehouse",

		// 品牌标识符
		"Brand New", "BRAND NEW", "brand new",
		"Official", "OFFICIAL", "official",
		"Authentic", "AUTHENTIC", "authentic",
		"Original", "ORIGINAL", "original",
		"Genuine", "GENUINE", "genuine",
	}
}

// TestEmojiFiltering 测试表情符号过滤功能（用于调试）
func (s *SensitiveWordService) TestEmojiFiltering(text string) string {
	logrus.Infof("🧪 测试表情符号过滤:")
	logrus.Infof("  原文: %s", text)

	filtered := s.filterEmojis(text)
	logrus.Infof("  过滤后: %s", filtered)

	aggressive := s.removeAllEmojisAggressively(text)
	logrus.Infof("  激进过滤: %s", aggressive)

	return aggressive
}
