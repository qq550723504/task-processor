// Package content 提供SHEIN平台的敏感词验证功能
package content

import (
	"regexp"
	"strings"

	sheinctx "task-processor/internal/shein/context"
)

// extractSensitiveWordsFromValidation 从验证结果中提取敏感词
func (s *SensitiveWordService) extractSensitiveWordsFromValidation(results []sheinctx.PreValidResult) []string {
	var sensitiveWords []string

	for _, result := range results {
		sensitiveWords = append(sensitiveWords, s.extractWordsFromMessages(result.Messages)...)

		for _, messages := range result.OtherLanguageMessageMap {
			sensitiveWords = append(sensitiveWords, s.extractWordsFromMessages(messages)...)
		}

		for _, skcError := range result.SkcErrorMessageMap {
			sensitiveWords = append(sensitiveWords, s.extractWordsFromMessages(skcError.Messages)...)

			for _, messages := range skcError.OtherLanguageMessageMap {
				sensitiveWords = append(sensitiveWords, s.extractWordsFromMessages(messages)...)
			}
		}
	}

	return s.deduplicateWords(sensitiveWords)
}

// extractWordsFromMessages 从消息中提取敏感词
func (s *SensitiveWordService) extractWordsFromMessages(messages []string) []string {
	var words []string

	// 正则按精确度从高到低排列：
	// 1. 带方括号的精确格式（如 敏感词：[Food Grade、Bpa Free]）
	// 2. 顿号/逗号分隔的无括号格式（如 敏感词：Food Grade、Bpa Free、Snap-on）
	// 3. 英文格式
	// 4. 其他违禁词格式
	// 注意：不使用 [^\]\s]+ 这类遇空格截断的正则，避免含空格的词被截断
	patterns := []string{
		`包含敏感词[：:]\s*\[([^\]]+)\]`,
		`敏感词[：:]\s*\[([^\]]+)\]`,
		`敏感词[：:]\s*([^，,\[\]\n]+(?:[，,、][^，,、\[\]\n]+)*)`,
		`contains?\s+sensitive\s+words?\s*[：:]\s*\[?([^\]]+)\]?`,
		`sensitive\s+words?\s*[：:]\s*\[?([^\]]+)\]?`,
		`违禁词[：:]\s*\[?([^\]]+)\]?`,
		`禁用词[：:]\s*\[?([^\]]+)\]?`,
		`不当词汇[：:]\s*\[?([^\]]+)\]?`,
	}

	for _, message := range messages {
		matched := false
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindAllStringSubmatch(message, -1)
			if len(matches) == 0 {
				continue
			}
			for _, match := range matches {
				if len(match) > 1 {
					wordsStr := strings.TrimSpace(match[1])
					extractedWords := s.splitWords(wordsStr)
					words = append(words, extractedWords...)
					matched = true
				}
			}
			// 匹配到后不再尝试后续正则，避免重复提取
			if matched {
				break
			}
		}
	}

	return s.deduplicateWords(words)
}

// splitWords 分割单词字符串
func (s *SensitiveWordService) splitWords(wordsStr string) []string {
	var words []string

	switch {
	case strings.Contains(wordsStr, "、"):
		words = strings.Split(wordsStr, "、")
	case strings.Contains(wordsStr, ","):
		words = strings.Split(wordsStr, ",")
	case strings.Contains(wordsStr, "，"):
		words = strings.Split(wordsStr, "，")
	default:
		words = strings.Fields(wordsStr)
	}

	var cleanWords []string
	for _, word := range words {
		if word = strings.TrimSpace(strings.Trim(word, "[]")); word != "" {
			cleanWords = append(cleanWords, word)
		}
	}

	return cleanWords
}

// addWordsByLanguage 按指定语言添加敏感词
func (s *SensitiveWordService) addWordsByLanguage(configMap map[string][]string, language string, words []string, wordType string) {
	if len(words) == 0 {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if configMap == nil {
		s.logger.Errorf("配置映射为空，无法添加%s敏感词", wordType)
		return
	}

	words = s.deduplicateWords(words)
	existing := configMap[language]
	combined := append(existing, words...)
	configMap[language] = s.deduplicateWords(combined)

	s.logger.Infof("✅ 已添加 %d 个%s敏感词到语言 [%s]，当前总数: %d",
		len(words), wordType, language, len(configMap[language]))

	s.saveConfigAsync()
}

// addWordsToConfig 将分类后的敏感词添加到配置中
func (s *SensitiveWordService) addWordsToConfig(configMap map[string][]string, wordsByLang map[string][]string, wordType string) int {
	totalAdded := 0

	for lang, words := range wordsByLang {
		if len(words) > 0 {
			beforeCount := len(configMap[lang])
			existing := configMap[lang]
			combined := append(existing, words...)
			configMap[lang] = s.deduplicateWords(combined)

			addedCount := len(configMap[lang]) - beforeCount
			totalAdded += addedCount

			if addedCount > 0 {
				s.logger.Infof("✅ [%s] 添加了 %d 个%s敏感词", lang, addedCount, wordType)
			}
		}
	}

	return totalAdded
}
