// Package content 提供SHEIN平台的敏感词验证功能
package content

import (
	"regexp"
	"strings"
	"task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

// extractSensitiveWordsFromValidation 从验证结果中提取敏感词
func (s *SensitiveWordService) extractSensitiveWordsFromValidation(results []shein.PreValidResult) []string {
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

	patterns := []string{
		`敏感词[：:]\s*\[?([^\]\s]+)\]?`,
		`包含敏感词[：:]\s*\[([^\]]+)\]`,
		`敏感词[：:]\s*\[([^\]]+)\]`,
		`敏感词[：:]\s*([^，,\[\]]+(?:[，,][^，,\[\]]+)*)`,
		`contains?\s+sensitive\s+words?\s*[：:]\s*\[?([^\]]+)\]?`,
		`sensitive\s+words?\s*[：:]\s*\[?([^\]]+)\]?`,
		`违禁词[：:]\s*\[?([^\]]+)\]?`,
		`禁用词[：:]\s*\[?([^\]]+)\]?`,
		`不当词汇[：:]\s*\[?([^\]]+)\]?`,
	}

	for _, message := range messages {
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindAllStringSubmatch(message, -1)

			for _, match := range matches {
				if len(match) > 1 {
					wordsStr := strings.TrimSpace(match[1])
					extractedWords := s.splitWords(wordsStr)
					words = append(words, extractedWords...)
				}
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
		logrus.Errorf("配置映射为空，无法添加%s敏感词", wordType)
		return
	}

	// 去重
	words = s.deduplicateWords(words)

	// 获取现有词汇
	existing := configMap[language]

	// 合并并去重
	combined := append(existing, words...)
	configMap[language] = s.deduplicateWords(combined)

	logrus.Infof("✅ 已添加 %d 个%s敏感词到语言 [%s]，当前总数: %d",
		len(words), wordType, language, len(configMap[language]))

	// 异步保存配置
	s.saveConfigAsync()
}

// addWordsToConfig 将分类后的敏感词添加到配置中
func (s *SensitiveWordService) addWordsToConfig(configMap map[string][]string, wordsByLang map[string][]string, wordType string) int {
	totalAdded := 0

	for lang, words := range wordsByLang {
		if len(words) > 0 {
			beforeCount := len(configMap[lang])

			// 合并并去重
			existing := configMap[lang]
			combined := append(existing, words...)
			configMap[lang] = s.deduplicateWords(combined)

			addedCount := len(configMap[lang]) - beforeCount
			totalAdded += addedCount

			if addedCount > 0 {
				logrus.Infof("✅ [%s] 添加了 %d 个%s敏感词", lang, addedCount, wordType)
			}
		}
	}

	return totalAdded
}
