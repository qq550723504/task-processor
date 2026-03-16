package management

import (
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// SensitiveWordCache 敏感词缓存管理器
type SensitiveWordCache struct {
	// 使用map存储不同语言的敏感词列表
	words          map[string][]string
	mutex          sync.RWMutex
	lastUpdate     map[string]time.Time
	updateInterval time.Duration
	clientMgr      *ClientManager
}

// NewSensitiveWordCache 创建新的敏感词缓存管理器
func NewSensitiveWordCache(clientMgr *ClientManager, updateInterval time.Duration) *SensitiveWordCache {
	cache := &SensitiveWordCache{
		words:          make(map[string][]string),
		updateInterval: updateInterval,
		clientMgr:      clientMgr,
		lastUpdate:     make(map[string]time.Time),
	}

	// 注意：不再立即加载敏感词，等待用户登录后手动调用 Initialize()

	return cache
}

// Initialize 初始化敏感词缓存（在用户登录后调用）
func (swc *SensitiveWordCache) Initialize() {
	// 立即加载一次默认语言的敏感词
	go swc.updateWordsForLanguage("")

	// 启动定期更新goroutine
	go swc.startPeriodicUpdate()

	logrus.Info("敏感词缓存已初始化")
}

// startPeriodicUpdate 启动定期更新
func (swc *SensitiveWordCache) startPeriodicUpdate() {
	ticker := time.NewTicker(swc.updateInterval)
	defer ticker.Stop()

	for range ticker.C {
		// 更新所有已缓存语言的敏感词列表
		swc.mutex.RLock()
		languages := make([]string, 0, len(swc.words))
		for lang := range swc.words {
			languages = append(languages, lang)
		}
		swc.mutex.RUnlock()

		// 为每种语言更新敏感词列表
		for _, lang := range languages {
			swc.updateWordsForLanguage(lang)
		}
	}
}

// updateWordsForLanguage 更新指定语言的敏感词列表
func (swc *SensitiveWordCache) updateWordsForLanguage(language string) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Infof("更新语言[%s]敏感词时发生错误: %v", language, r)
		}
	}()

	client := swc.clientMgr.GetSensitiveWordClient()
	var langPtr *string
	if language != "" {
		langPtr = &language
	}

	words, err := client.GetAllEnableSensitiveWordList(langPtr)
	if err != nil {
		// 如果是未登录错误，使用 debug 级别，否则使用 info 级别
		if strings.Contains(err.Error(), "未设置用户访问令牌") {
			logrus.Debugf("获取语言[%s]敏感词列表失败（未登录）: %v", language, err)
		} else {
			logrus.Infof("获取语言[%s]敏感词列表失败: %v", language, err)
		}
		return
	}

	// 检查响应数据
	if words == nil {
		logrus.Infof("语言[%s]敏感词列表数据为空", language)
		return
	}

	// 更新缓存
	swc.mutex.Lock()
	swc.words[language] = *words
	swc.lastUpdate[language] = time.Now()
	swc.mutex.Unlock()

	logrus.Infof("成功更新语言[%s]敏感词列表，共%d个敏感词", language, len(*words))
}

// GetWords 获取指定语言的敏感词列表
func (swc *SensitiveWordCache) GetWords(language string) []string {
	swc.mutex.RLock()
	defer swc.mutex.RUnlock()

	// 返回敏感词列表的副本，避免外部修改
	words, exists := swc.words[language]
	if !exists {
		return []string{}
	}

	wordsCopy := make([]string, len(words))
	copy(wordsCopy, words)

	return wordsCopy
}

// GetAllLanguagesWords 获取所有语言的敏感词列表
func (swc *SensitiveWordCache) GetAllLanguagesWords() map[string][]string {
	swc.mutex.RLock()
	defer swc.mutex.RUnlock()

	// 返回所有语言敏感词列表的副本
	allWords := make(map[string][]string)
	for lang, words := range swc.words {
		wordsCopy := make([]string, len(words))
		copy(wordsCopy, words)
		allWords[lang] = wordsCopy
	}

	return allWords
}

// Contains 检查文本是否包含指定语言的敏感词
func (swc *SensitiveWordCache) Contains(text string, language string) bool {
	words := swc.GetWords(language)

	for _, word := range words {
		if len(word) > 0 && len(text) >= len(word) {
			// 简单的字符串匹配检查
			for i := 0; i <= len(text)-len(word); i++ {
				if text[i:i+len(word)] == word {
					return true
				}
			}
		}
	}

	return false
}

// GetSensitiveWords 获取文本中指定语言的所有敏感词
func (swc *SensitiveWordCache) GetSensitiveWords(text string, language string) []string {
	words := swc.GetWords(language)
	var foundWords []string

	for _, word := range words {
		if len(word) > 0 && len(text) >= len(word) {
			// 简单的字符串匹配检查
			for i := 0; i <= len(text)-len(word); i++ {
				if text[i:i+len(word)] == word {
					foundWords = append(foundWords, word)
					break // 避免重复添加同一个敏感词
				}
			}
		}
	}

	return foundWords
}

// ReplaceSensitiveWords 替换文本中指定语言的敏感词
func (swc *SensitiveWordCache) ReplaceSensitiveWords(text, replaceText string, language string) string {
	words := swc.GetWords(language)
	result := text

	// 按长度从长到短排序，优先替换较长的敏感词
	// 这里简化处理，实际应用中可能需要更复杂的排序逻辑

	for _, word := range words {
		if len(word) > 0 {
			result = replaceString(result, word, replaceText)
		}
	}

	return result
}

// replaceString 替换字符串中的所有匹配项
func replaceString(text, old, new string) string {
	// 简单的字符串替换实现
	if len(old) == 0 {
		return text
	}

	result := ""
	start := 0

	for i := 0; i <= len(text)-len(old); i++ {
		if text[i:i+len(old)] == old {
			// 找到匹配项，添加之前的文本和替换文本
			result += text[start:i] + new
			start = i + len(old)
			i = start - 1 // 调整索引，继续搜索
		}
	}

	// 添加剩余的文本
	result += text[start:]

	return result
}

// ValidateText 验证文本是否包含指定语言的敏感词
func (swc *SensitiveWordCache) ValidateText(text string, language string) bool {
	return swc.Contains(text, language)
}

// GetLastUpdate 获取指定语言的最后更新时间
func (swc *SensitiveWordCache) GetLastUpdate(language string) time.Time {
	swc.mutex.RLock()
	defer swc.mutex.RUnlock()

	if lastUpdate, exists := swc.lastUpdate[language]; exists {
		return lastUpdate
	}

	return time.Time{}
}

// GetCacheInfo 获取缓存信息
func (swc *SensitiveWordCache) GetCacheInfo() map[string]any {
	swc.mutex.RLock()
	defer swc.mutex.RUnlock()

	info := make(map[string]any)

	for lang := range swc.words {
		info[lang] = map[string]any{
			"wordCount":  len(swc.words[lang]),
			"lastUpdate": swc.lastUpdate[lang],
			"nextUpdate": swc.lastUpdate[lang].Add(swc.updateInterval),
		}
	}

	return info
}

// EnsureLanguageCache 确保指定语言的缓存已加载
func (swc *SensitiveWordCache) EnsureLanguageCache(language string) {
	swc.mutex.Lock()
	_, exists := swc.words[language]
	swc.mutex.Unlock()

	if !exists {
		// 如果该语言的缓存不存在，则立即加载
		swc.updateWordsForLanguage(language)
	}
}
