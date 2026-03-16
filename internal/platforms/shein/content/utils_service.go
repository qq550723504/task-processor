// Package content 提供SHEIN平台的敏感词工具函数
package content

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"
)

// classifyWordsByLanguage 按语言分类敏感词
func (s *SensitiveWordService) classifyWordsByLanguage(words []string) map[string][]string {
	result := make(map[string][]string)

	for _, word := range words {
		if word = strings.TrimSpace(word); word != "" {
			lang := s.detectLanguage(word)
			result[lang] = append(result[lang], word)
		}
	}

	return result
}

// detectLanguage 检测单词的语言
func (s *SensitiveWordService) detectLanguage(word string) string {
	word = strings.TrimSpace(strings.ToLower(word))

	if word == "" {
		return "en" // 默认英文
	}

	// 检测日文
	if s.containsJapanese(word) {
		return "ja"
	}

	// 检测中文
	if s.containsChinese(word) {
		return "zh"
	}

	// 检测韩文
	if s.containsKorean(word) {
		return "ko"
	}

	// 检测俄文等西里尔字符
	if s.containsCyrillic(word) {
		return "ru"
	}

	// 检测阿拉伯文
	if s.containsArabic(word) {
		return "ar"
	}

	// 默认为英文
	return "en"
}

// containsJapanese 检测是否包含日文字符
func (s *SensitiveWordService) containsJapanese(text string) bool {
	japanesePattern := regexp.MustCompile("[\u3040-\u309F\u30A0-\u30FF\u4E00-\u9FAF\u3000-\u303F]")
	return japanesePattern.MatchString(text)
}

// containsChinese 检测是否包含中文字符
func (s *SensitiveWordService) containsChinese(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Scripts["Han"], r) {
			return true
		}
	}
	return false
}

// containsKorean 检测是否包含韩文字符
func (s *SensitiveWordService) containsKorean(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Scripts["Hangul"], r) {
			return true
		}
	}
	return false
}

// containsCyrillic 检测是否包含西里尔字符（俄文等）
func (s *SensitiveWordService) containsCyrillic(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Scripts["Cyrillic"], r) {
			return true
		}
	}
	return false
}

// containsArabic 检测是否包含阿拉伯文字符
func (s *SensitiveWordService) containsArabic(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Scripts["Arabic"], r) {
			return true
		}
	}
	return false
}

// preprocessText 预处理文本
func (s *SensitiveWordService) preprocessText(text string) string {
	text = s.filterEmojis(text)
	text = s.normalizeSpecialCharacters(text)
	return text
}

// removeWordFromText 从文本中移除指定单词
func (s *SensitiveWordService) removeWordFromText(text, word string) string {
	if s.containsJapanese(word) {
		return strings.ReplaceAll(text, word, "")
	}

	pattern := `\b` + regexp.QuoteMeta(word) + `\b`
	re := regexp.MustCompile(`(?i)` + pattern)
	return re.ReplaceAllString(text, "")
}

// cleanupText 清理文本中的多余空格
func (s *SensitiveWordService) cleanupText(text string) string {
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// filterEmojis 过滤表情符号
func (s *SensitiveWordService) filterEmojis(text string) string {
	// 更全面的表情符号正则表达式，覆盖所有Unicode表情符号范围
	emojiRegex := regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|` + // 表情符号 (Emoticons)
		`[\x{1F300}-\x{1F5FF}]|` + // 杂项符号和象形文字 (Misc Symbols and Pictographs)
		`[\x{1F680}-\x{1F6FF}]|` + // 交通和地图符号 (Transport and Map Symbols)
		`[\x{1F1E0}-\x{1F1FF}]|` + // 区域指示符号 (Regional Indicator Symbols)
		`[\x{2600}-\x{26FF}]|` + // 杂项符号 (Miscellaneous Symbols)
		`[\x{2700}-\x{27BF}]|` + // 装饰符号 (Dingbats)
		`[\x{1F900}-\x{1F9FF}]|` + // 补充符号和象形文字 (Supplemental Symbols and Pictographs)
		`[\x{1FA70}-\x{1FAFF}]|` + // 符号和象形文字扩展-A (Symbols and Pictographs Extended-A)
		`[\x{FE00}-\x{FE0F}]|` + // 变体选择器 (Variation Selectors)
		`[\x{200D}]`) // 零宽连接符 (Zero Width Joiner)

	return emojiRegex.ReplaceAllString(text, "")
}

// removeAllEmojisAggressively 激进地移除所有表情符号
func (s *SensitiveWordService) removeAllEmojisAggressively(text string) string {
	// 先使用字符串替换移除常见的表情符号
	commonEmojis := []string{
		"😀", "😁", "😂", "🤣", "😃", "😄", "😅", "😆", "😉", "😊",
		"😋", "😎", "😍", "😘", "🥰", "😗", "😙", "😚", "☺️", "🙂",
		"🤗", "🤩", "🤔", "🤨", "😐", "😑", "😶", "🙄", "😏", "😣",
		"😥", "😮", "🤐", "😯", "😪", "😫", "🥱", "😴", "😌", "😛",
		"😜", "😝", "🤤", "😒", "😓", "😔", "😕", "🙃", "🤑", "😲",
		"☹️", "🙁", "😖", "😞", "😟", "😤", "😢", "😭", "😦", "😧",
		"😨", "😩", "🤯", "😬", "😰", "😱", "🥵", "🥶", "😳", "🤪",
		"😵", "🥴", "😠", "😡", "🤬", "😷", "🤒", "🤕", "🤢", "🤮",
		"🤧", "😇", "🥳", "🥺", "🤠", "🤡", "🤥", "🤫", "🤭", "🧐",
		"🤓", "😈", "👿", "👹", "👺", "💀", "☠️", "👻", "👽", "👾",
		"🤖", "🎃", "😺", "😸", "😹", "😻", "😼", "😽", "🙀", "😿",
		"😾", "❤️", "🧡", "💛", "💚", "💙", "💜", "🖤", "🤍", "🤎",
		"💔", "❣️", "💕", "💞", "💓", "💗", "💖", "💘", "💝", "💟",
	}

	for _, emoji := range commonEmojis {
		text = strings.ReplaceAll(text, emoji, "")
	}

	// 然后使用正则表达式移除剩余的表情符号
	text = s.filterEmojis(text)

	return text
}

// normalizeSpecialCharacters 标准化特殊字符
func (s *SensitiveWordService) normalizeSpecialCharacters(input string) string {
	result := []rune{}

	for _, r := range input {
		switch {
		case r >= 'A' && r <= 'Z':
			result = append(result, r+32) // 转换为小写
		case r >= 'a' && r <= 'z':
			result = append(result, r)
		case r >= '0' && r <= '9':
			result = append(result, r)
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			result = append(result, ' ')
		case unicode.Is(unicode.Scripts["Han"], r):
			result = append(result, r) // 保留中文字符
		case unicode.Is(unicode.Scripts["Hiragana"], r) || unicode.Is(unicode.Scripts["Katakana"], r):
			result = append(result, r) // 保留日文字符
		case unicode.Is(unicode.Scripts["Hangul"], r):
			result = append(result, r) // 保留韩文字符
		case unicode.Is(unicode.Scripts["Cyrillic"], r):
			result = append(result, r) // 保留俄文字符
		case unicode.Is(unicode.Scripts["Arabic"], r):
			result = append(result, r) // 保留阿拉伯文字符
		default:
			// 其他字符转换为空格
			if len(result) > 0 && result[len(result)-1] != ' ' {
				result = append(result, ' ')
			}
		}
	}

	return strings.TrimSpace(string(result))
}

// deduplicateWords 去重单词列表
func (s *SensitiveWordService) deduplicateWords(words []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, word := range words {
		word = strings.TrimSpace(strings.ToLower(word))
		if word != "" && !seen[word] {
			seen[word] = true
			result = append(result, word)
		}
	}

	return result
}

// countWordsInConfig 统计配置中的敏感词数量
func (s *SensitiveWordService) countWordsInConfig(configMap map[string][]string) int {
	total := 0
	for _, words := range configMap {
		total += len(words)
	}
	return total
}

// logConfigLoadStats 记录配置加载统计信息
func (s *SensitiveWordService) logConfigLoadStats() {
	staticTotal := s.countWordsInConfig(s.config.StaticWords)
	dynamicTotal := s.countWordsInConfig(s.config.DynamicWords)

	logrus.WithFields(map[string]interface{}{
		"static_total":  staticTotal,
		"dynamic_total": dynamicTotal,
		"total":         staticTotal + dynamicTotal,
		"version":       s.config.Version,
		"last_updated":  s.config.LastUpdated.Format("2006-01-02 15:04:05"),
	}).Info("✅ 敏感词配置加载完成")

	// 按语言统计
	for lang, words := range s.config.StaticWords {
		if len(words) > 0 {
			logrus.Infof("  📝 静态敏感词 [%s]: %d个", lang, len(words))
		}
	}

	for lang, words := range s.config.DynamicWords {
		if len(words) > 0 {
			logrus.Infof("  🔄 动态敏感词 [%s]: %d个", lang, len(words))
		}
	}
}

// logSensitiveWordStats 记录敏感词统计信息
func (s *SensitiveWordService) logSensitiveWordStats() {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.config == nil {
		logrus.Warn("敏感词配置未初始化")
		return
	}

	staticTotal := s.countWordsInConfig(s.config.StaticWords)
	dynamicTotal := s.countWordsInConfig(s.config.DynamicWords)
	amazonBrandWordsCount := len(s.getAmazonBrandWords())

	logrus.Infof("📊 敏感词统计:")

	// 显示各语言的敏感词数量
	for lang, words := range s.config.StaticWords {
		if count := len(words); count > 0 {
			logrus.Infof("   静态敏感词 [%s]: %d 个", lang, count)
		}
	}

	for lang, words := range s.config.DynamicWords {
		if count := len(words); count > 0 {
			logrus.Infof("   动态敏感词 [%s]: %d 个", lang, count)
		}
	}

	logrus.Infof("   Amazon品牌词: %d 个", amazonBrandWordsCount)
	logrus.Infof("   总计: 静态(%d) + 动态(%d) + 品牌词(%d) = %d 个",
		staticTotal, dynamicTotal, amazonBrandWordsCount, staticTotal+dynamicTotal+amazonBrandWordsCount)
	logrus.Infof("   配置文件: %s", s.configPath)
	logrus.Infof("   最后更新: %s", s.config.LastUpdated.Format("2006-01-02 15:04:05"))
}
