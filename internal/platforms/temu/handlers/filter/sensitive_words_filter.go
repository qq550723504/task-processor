package filter

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/jsonx"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// SensitiveWordsFilter TEMU敏感词过滤器
type SensitiveWordsFilter struct {
	logger       *logrus.Entry
	staticWords  map[string][]string
	dynamicWords map[string][]*regexp.Regexp
	configPath   string
}

// NewSensitiveWordsFilter 创建TEMU敏感词过滤器
func NewSensitiveWordsFilter() *SensitiveWordsFilter {
	filter := &SensitiveWordsFilter{
		logger:       logrus.WithField("handler", "SensitiveWordsFilter"),
		staticWords:  make(map[string][]string),
		dynamicWords: make(map[string][]*regexp.Regexp),
		configPath:   "data/sensitive_words_temu.json",
	}

	if err := filter.loadConfig(); err != nil {
		filter.logger.WithError(err).Warn("加载敏感词配置失败，使用默认配置")
		filter.loadDefaultConfig()
	}

	return filter
}

// Name 返回处理器名称
func (f *SensitiveWordsFilter) Name() string {
	return "TEMU敏感词过滤处理器"
}

// Handle 处理任务
// HandleTemu 处理TEMU任务（实现TemuHandler接口）
func (f *SensitiveWordsFilter) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	f.logger.Info("开始过滤TEMU敏感词")

	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	temuProduct := temuCtx.TemuProduct

	// 过滤标题
	originalTitle := temuProduct.GoodsBasic.GoodsName
	filteredTitle, titleViolations := f.FilterText(originalTitle)
	if len(titleViolations) > 0 {
		f.logger.Warnf("标题包含敏感词: %v", titleViolations)
		temuProduct.GoodsBasic.GoodsName = filteredTitle
		f.logger.Infof("标题已过滤: %s -> %s", originalTitle, filteredTitle)
	}

	// 过滤描述
	originalDesc := temuProduct.GoodsExtensionInfo.GoodsDesc
	filteredDesc, descViolations := f.FilterText(originalDesc)
	if len(descViolations) > 0 {
		f.logger.Warnf("描述包含敏感词: %v", descViolations)
		temuProduct.GoodsExtensionInfo.GoodsDesc = filteredDesc
		f.logger.Infof("描述已过滤，长度: %d -> %d", len(originalDesc), len(filteredDesc))
	}

	// 过滤要点
	originalPoints := temuProduct.GoodsExtensionInfo.BulletPoints
	filteredPoints := []string{}
	pointsModified := false

	for i, point := range originalPoints {
		filteredPoint, pointViolations := f.FilterText(point)
		if len(pointViolations) > 0 {
			f.logger.Warnf("要点[%d]包含敏感词: %v", i+1, pointViolations)
			pointsModified = true
		}
		if strings.TrimSpace(filteredPoint) != "" {
			filteredPoints = append(filteredPoints, filteredPoint)
		}
	}

	if pointsModified {
		temuProduct.GoodsExtensionInfo.BulletPoints = filteredPoints
		f.logger.Infof("要点已过滤: %d -> %d个", len(originalPoints), len(filteredPoints))
	}

	f.logger.Info("TEMU敏感词过滤完成")
	return nil
}

// Handle 兼容原有的Handler接口（用于pipeline.AddHandler）
func (f *SensitiveWordsFilter) Handle(ctx pipeline.TaskContext) error {
	// 尝试类型断言为TemuTaskContext
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return f.HandleTemu(temuCtx)
	}
	// 如果不是TemuTaskContext，返回错误
	return fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
}

// FilterText 过滤文本中的敏感词
func (f *SensitiveWordsFilter) FilterText(text string) (string, []string) {
	violations := []string{}
	filtered := text

	// 1. 过滤静态敏感词（英文）
	if words, ok := f.staticWords["en"]; ok {
		for _, word := range words {
			pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
			if pattern.MatchString(filtered) {
				violations = append(violations, word)
				filtered = pattern.ReplaceAllString(filtered, "")
			}
		}
	}

	// 2. 过滤静态敏感词（中文）
	if words, ok := f.staticWords["zh"]; ok {
		for _, word := range words {
			if strings.Contains(filtered, word) {
				violations = append(violations, word)
				filtered = strings.ReplaceAll(filtered, word, "")
			}
		}
	}

	// 3. 过滤动态敏感词（正则表达式）
	if patterns, ok := f.dynamicWords["en"]; ok {
		for _, pattern := range patterns {
			if pattern.MatchString(filtered) {
				violations = append(violations, pattern.String())
				filtered = pattern.ReplaceAllString(filtered, "")
			}
		}
	}

	// 4. 清理过滤后的多余空格
	filtered = f.cleanFilteredText(filtered)

	return filtered, violations
}

// cleanFilteredText 清理过滤后的文本
func (f *SensitiveWordsFilter) cleanFilteredText(text string) string {
	// 移除多余的空格
	spacePattern := regexp.MustCompile(`\s+`)
	text = spacePattern.ReplaceAllString(text, " ")

	// 移除首尾空格
	text = strings.TrimSpace(text)

	// 移除多余的标点符号
	text = regexp.MustCompile(`[,;.]\s*[,;.]`).ReplaceAllString(text, ",")

	return text
}

// loadConfig 加载敏感词配置
func (f *SensitiveWordsFilter) loadConfig() error {
	data, err := os.ReadFile(f.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg config.SensitiveWordsConfig
	if err := jsonx.UnmarshalBytes(data, &cfg, "解析配置文件失败"); err != nil {
		return err
	}

	// 加载静态敏感词
	f.staticWords = cfg.StaticWords

	// 编译动态敏感词正则表达式
	for lang, patterns := range cfg.DynamicWords {
		f.dynamicWords[lang] = []*regexp.Regexp{}
		for _, pattern := range patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				f.logger.WithError(err).Warnf("编译正则表达式失败: %s", pattern)
				continue
			}
			f.dynamicWords[lang] = append(f.dynamicWords[lang], re)
		}
	}

	f.logger.Infof("加载敏感词配置成功: 静态词=%d, 动态模式=%d",
		len(f.staticWords["en"])+len(f.staticWords["zh"]),
		len(f.dynamicWords["en"]))

	return nil
}

// loadDefaultConfig 加载默认配置
func (f *SensitiveWordsFilter) loadDefaultConfig() {
	// 默认环保相关敏感词
	f.staticWords["en"] = []string{
		"environmentally friendly",
		"eco-friendly",
		"eco friendly",
		"environment friendly",
		"sustainable",
		"sustainability",
		"biodegradable",
		"recyclable",
		"recycled",
		"organic",
		"green product",
		"earth friendly",
		"planet friendly",
		"carbon neutral",
		"zero waste",
	}

	f.staticWords["zh"] = []string{
		"环保",
		"环境友好",
		"生态友好",
		"可持续",
		"可降解",
		"可回收",
		"有机",
	}

	// 编译默认动态模式
	defaultPatterns := []string{
		`(?i)\b(eco[\\s-]?friendly|ecofriendly)\b`,
		`(?i)\b(environment[\\s-]?friendly|environment friendly)\b`,
		`(?i)\b(environmentally[\\s-]?friendly|environmentally friendly)\b`,
		`(?i)\b(environmental|environmentally)\b`,
		`(?i)\b(sustainable|sustainability)\b`,
		`(?i)\b(biodegradable)\b`,
		`(?i)\b(recyclable|recycled)\b`,
		`(?i)\b(organic)\b`,
	}

	f.dynamicWords["en"] = []*regexp.Regexp{}
	for _, pattern := range defaultPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			f.dynamicWords["en"] = append(f.dynamicWords["en"], re)
		}
	}

	f.logger.Info("使用默认敏感词配置")
}

// CheckText 检查文本是否包含敏感词（不修改文本）
func (f *SensitiveWordsFilter) CheckText(text string) []string {
	violations := []string{}

	// 检查静态敏感词
	if words, ok := f.staticWords["en"]; ok {
		for _, word := range words {
			pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
			if pattern.MatchString(text) {
				violations = append(violations, word)
			}
		}
	}

	// 检查动态敏感词
	if patterns, ok := f.dynamicWords["en"]; ok {
		for _, pattern := range patterns {
			if pattern.MatchString(text) {
				violations = append(violations, pattern.String())
			}
		}
	}

	return violations
}
