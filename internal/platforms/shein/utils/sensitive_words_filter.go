package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

// SensitiveWordsFilter SHEIN 敏感词过滤器
type SensitiveWordsFilter struct {
	mu              sync.RWMutex
	config          *config.SensitiveWordsConfig
	configPath      string
	compiledRegexes map[string][]*regexp.Regexp
	hardcodedWords  map[string][]string
}

// NewSensitiveWordsFilter 创建敏感词过滤器
func NewSensitiveWordsFilter(configPath string) (*SensitiveWordsFilter, error) {
	filter := &SensitiveWordsFilter{
		configPath:      configPath,
		compiledRegexes: make(map[string][]*regexp.Regexp),
		hardcodedWords:  initHardcodedWords(),
	}

	if err := filter.LoadConfig(); err != nil {
		return nil, fmt.Errorf("加载敏感词配置失败: %w", err)
	}

	return filter, nil
}

// initHardcodedWords 初始化硬编码敏感词（优先级最高）
func initHardcodedWords() map[string][]string {
	return map[string][]string{
		"en": {
			"(?i)925\\s*sterling", // 925 Sterling Silver
			"(?i)\\bfake\\b",
			"(?i)\\bcounterfeit\\b",
			"(?i)\\breplica\\b",
		},
		"zh": {
			"假货",
			"仿品",
		},
	}
}

// LoadConfig 加载配置文件
func (f *SensitiveWordsFilter) LoadConfig() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg config.SensitiveWordsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	f.config = &cfg

	// 编译动态正则表达式
	f.compiledRegexes = make(map[string][]*regexp.Regexp)
	for lang, patterns := range cfg.DynamicWords {
		var regexes []*regexp.Regexp
		for _, pattern := range patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{
					"language": lang,
					"pattern":  pattern,
				}).Warn("编译正则表达式失败，跳过该规则")
				continue
			}
			regexes = append(regexes, re)
		}
		f.compiledRegexes[lang] = regexes
	}

	logrus.WithFields(logrus.Fields{
		"config_path": f.configPath,
		"version":     cfg.Version,
		"platform":    cfg.Platform,
	}).Info("✅ 敏感词配置加载成功")

	return nil
}

// ReloadConfig 重新加载配置（支持热更新）
func (f *SensitiveWordsFilter) ReloadConfig() error {
	return f.LoadConfig()
}

// CheckText 检查文本是否包含敏感词
func (f *SensitiveWordsFilter) CheckText(text string, language string) (bool, []string) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var foundWords []string

	// 1. 优先检查硬编码敏感词（最高优先级）
	if hardcodedPatterns, ok := f.hardcodedWords[language]; ok {
		for _, pattern := range hardcodedPatterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				logrus.WithError(err).Warn("编译硬编码正则失败")
				continue
			}
			if re.MatchString(text) {
				foundWords = append(foundWords, pattern)
			}
		}
	}

	// 2. 检查配置文件中的静态敏感词
	if staticWords, ok := f.config.StaticWords[language]; ok {
		for _, word := range staticWords {
			if strings.Contains(strings.ToLower(text), strings.ToLower(word)) {
				foundWords = append(foundWords, word)
			}
		}
	}

	// 3. 检查配置文件中的动态正则表达式
	if regexes, ok := f.compiledRegexes[language]; ok {
		for _, re := range regexes {
			if re.MatchString(text) {
				foundWords = append(foundWords, re.String())
			}
		}
	}

	return len(foundWords) > 0, foundWords
}

// CheckProduct 检查产品信息是否包含敏感词
func (f *SensitiveWordsFilter) CheckProduct(title, description string, languages []string) (bool, map[string][]string) {
	result := make(map[string][]string)
	hasSensitiveWords := false

	for _, lang := range languages {
		var allFoundWords []string

		// 检查标题
		if titleHasSensitive, titleWords := f.CheckText(title, lang); titleHasSensitive {
			allFoundWords = append(allFoundWords, titleWords...)
			hasSensitiveWords = true
		}

		// 检查描述
		if descHasSensitive, descWords := f.CheckText(description, lang); descHasSensitive {
			allFoundWords = append(allFoundWords, descWords...)
			hasSensitiveWords = true
		}

		if len(allFoundWords) > 0 {
			result[lang] = allFoundWords
		}
	}

	return hasSensitiveWords, result
}

// GetConfig 获取当前配置（只读）
func (f *SensitiveWordsFilter) GetConfig() *config.SensitiveWordsConfig {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config
}

// GetLastUpdated 获取配置最后更新时间
func (f *SensitiveWordsFilter) GetLastUpdated() (time.Time, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.config.LastUpdated == "" {
		return time.Time{}, fmt.Errorf("配置中没有更新时间")
	}

	return time.Parse(time.RFC3339, f.config.LastUpdated)
}
