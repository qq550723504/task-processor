// Package content 敏感词处理器
package content

import (
	"fmt"
	corelogger "task-processor/internal/core/logger"
	"task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

// SensitiveWordsFilter 敏感词过滤器接口
type SensitiveWordsFilter interface {
	CheckProduct(title, description string, languages []string) (bool, map[string][]string)
}

// SensitiveWordsMode 敏感词处理模式
type SensitiveWordsMode int

const (
	ModeBlock SensitiveWordsMode = iota
	ModeClean
	ModeWarn
)

// SensitiveWordsProcessor 敏感词处理器
type SensitiveWordsProcessor struct {
	mode                 SensitiveWordsMode
	filter               SensitiveWordsFilter
	sensitiveWordService *SensitiveWordService
	logger               *logrus.Entry
}

// NewSensitiveWordsProcessor 创建敏感词处理器
func NewSensitiveWordsProcessor(mode SensitiveWordsMode, filter SensitiveWordsFilter) *SensitiveWordsProcessor {
	return &SensitiveWordsProcessor{
		mode:                 mode,
		filter:               filter,
		sensitiveWordService: NewSensitiveWordService(),
		logger:               corelogger.GetGlobalLogger("shein.sensitive_words_processor"),
	}
}

// Name 返回处理器名称
func (h *SensitiveWordsProcessor) Name() string {
	switch h.mode {
	case ModeBlock:
		return "敏感词检查(拦截)"
	case ModeClean:
		return "敏感词清理(替换)"
	case ModeWarn:
		return "敏感词检查(警告)"
	default:
		return "敏感词处理"
	}
}

// Handle 执行敏感词处理
func (h *SensitiveWordsProcessor) Handle(ctx *shein.TaskContext) error {
	if ctx.AmazonProduct == nil {
		return fmt.Errorf("产品信息为空")
	}

	title := ctx.AmazonProduct.Title
	description := ctx.AmazonProduct.Description
	languages := []string{"en", "zh", "es", "fr", "de"}

	// 检查是否包含敏感词
	hasSensitive, foundWords := h.filter.CheckProduct(title, description, languages)

	if !hasSensitive {
		h.logger.WithField("asin", ctx.AmazonProduct.Asin).Debug("✅ 产品未包含敏感词")
		return nil
	}

	// 根据模式处理
	switch h.mode {
	case ModeBlock:
		return h.handleBlock(ctx, foundWords)
	case ModeClean:
		return h.handleClean(ctx, foundWords)
	case ModeWarn:
		return h.handleWarn(ctx, foundWords)
	default:
		return fmt.Errorf("未知的处理模式: %d", h.mode)
	}
}

func (h *SensitiveWordsProcessor) handleBlock(ctx *shein.TaskContext, foundWords map[string][]string) error {
	h.logger.WithFields(logrus.Fields{
		"asin":            ctx.AmazonProduct.Asin,
		"title":           ctx.AmazonProduct.Title,
		"sensitive_words": foundWords,
	}).Warn("⚠️ 产品包含敏感词,拦截发布")
	return shein.NewFilteredError(fmt.Sprintf("产品包含敏感词: %v", foundWords))
}

func (h *SensitiveWordsProcessor) handleClean(ctx *shein.TaskContext, foundWords map[string][]string) error {
	h.logger.WithFields(logrus.Fields{
		"asin":            ctx.AmazonProduct.Asin,
		"sensitive_words": foundWords,
	}).Info("🔧 检测到敏感词,开始自动清理...")

	if ctx.ProductData == nil {
		return fmt.Errorf("产品数据未获取,无法执行清理")
	}
	if err := h.sensitiveWordService.ProcessProductData(ctx); err != nil {
		return fmt.Errorf("敏感词清理失败: %w", err)
	}
	h.logger.Info("✅ 敏感词清理完成")
	return nil
}

func (h *SensitiveWordsProcessor) handleWarn(ctx *shein.TaskContext, foundWords map[string][]string) error {
	h.logger.WithFields(logrus.Fields{
		"asin":            ctx.AmazonProduct.Asin,
		"title":           ctx.AmazonProduct.Title,
		"sensitive_words": foundWords,
	}).Warn("⚠️ 产品包含敏感词(仅警告)")
	return nil
}

// NewSensitiveWordsBlockHandler 创建拦截模式处理器(兼容旧接口)
func NewSensitiveWordsBlockHandler(filter SensitiveWordsFilter) *SensitiveWordsProcessor {
	return NewSensitiveWordsProcessor(ModeBlock, filter)
}

// NewSensitiveWordsCleanHandler 创建清理模式处理器(兼容旧接口)
func NewSensitiveWordsCleanHandler(filter SensitiveWordsFilter) *SensitiveWordsProcessor {
	return NewSensitiveWordsProcessor(ModeClean, filter)
}

// NewSensitiveWordsWarnHandler 创建警告模式处理器
func NewSensitiveWordsWarnHandler(filter SensitiveWordsFilter) *SensitiveWordsProcessor {
	return NewSensitiveWordsProcessor(ModeWarn, filter)
}
