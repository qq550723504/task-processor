// Package modules 敏感词处理器
package content

import (
	"fmt"
	"task-processor/internal/platforms/shein"

	"github.com/sirupsen/logrus"
)

// SensitiveWordsFilterInterface 敏感词过滤器接口
type SensitiveWordsFilterInterface interface {
	CheckProduct(title, description string, languages []string) (bool, map[string][]string)
}

// SensitiveWordsMode 敏感词处理模式
type SensitiveWordsMode int

const (
	// ModeBlock 拦截模式 - 发现敏感词直接拦截
	ModeBlock SensitiveWordsMode = iota
	// ModeClean 清理模式 - 发现敏感词自动替换
	ModeClean
	// ModeWarn 警告模式 - 发现敏感词仅记录日志
	ModeWarn
)

// SensitiveWordsProcessor 敏感词处理器
type SensitiveWordsProcessor struct {
	mode                 SensitiveWordsMode
	filter               SensitiveWordsFilterInterface
	sensitiveWordService *SensitiveWordService
}

// NewSensitiveWordsProcessor 创建敏感词处理器
func NewSensitiveWordsProcessor(mode SensitiveWordsMode, filter SensitiveWordsFilterInterface) *SensitiveWordsProcessor {
	return &SensitiveWordsProcessor{
		mode:                 mode,
		filter:               filter,
		sensitiveWordService: NewSensitiveWordService(),
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
		logrus.WithField("asin", ctx.AmazonProduct.Asin).Debug("✅ 产品未包含敏感词")
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

// handleBlock 拦截模式 - 直接返回错误
func (h *SensitiveWordsProcessor) handleBlock(ctx *shein.TaskContext, foundWords map[string][]string) error {
	logrus.WithFields(logrus.Fields{
		"asin":            ctx.AmazonProduct.Asin,
		"title":           ctx.AmazonProduct.Title,
		"sensitive_words": foundWords,
	}).Warn("⚠️ 产品包含敏感词,拦截发布")

	return shein.NewFilteredError(fmt.Sprintf("产品包含敏感词: %v", foundWords))
}

// handleClean 清理模式 - 自动替换敏感词
func (h *SensitiveWordsProcessor) handleClean(ctx *shein.TaskContext, foundWords map[string][]string) error {
	logrus.WithFields(logrus.Fields{
		"asin":            ctx.AmazonProduct.Asin,
		"sensitive_words": foundWords,
	}).Info("🔧 检测到敏感词,开始自动清理...")

	// 检查产品数据是否已准备
	if ctx.ProductData == nil {
		return fmt.Errorf("产品数据未获取,无法执行清理")
	}

	// 使用敏感词服务清理产品数据
	if err := h.sensitiveWordService.ProcessProductData(ctx); err != nil {
		return fmt.Errorf("敏感词清理失败: %v", err)
	}

	logrus.Info("✅ 敏感词清理完成")
	return nil
}

// handleWarn 警告模式 - 仅记录日志,不拦截
func (h *SensitiveWordsProcessor) handleWarn(ctx *shein.TaskContext, foundWords map[string][]string) error {
	logrus.WithFields(logrus.Fields{
		"asin":            ctx.AmazonProduct.Asin,
		"title":           ctx.AmazonProduct.Title,
		"sensitive_words": foundWords,
	}).Warn("⚠️ 产品包含敏感词(仅警告)")

	return nil
}

// NewSensitiveWordsBlockHandler 创建拦截模式处理器(兼容旧接口)
func NewSensitiveWordsBlockHandler(filter SensitiveWordsFilterInterface) *SensitiveWordsProcessor {
	return NewSensitiveWordsProcessor(ModeBlock, filter)
}

// NewSensitiveWordsCleanHandler 创建清理模式处理器(兼容旧接口)
func NewSensitiveWordsCleanHandler(filter SensitiveWordsFilterInterface) *SensitiveWordsProcessor {
	return NewSensitiveWordsProcessor(ModeClean, filter)
}

// NewSensitiveWordsWarnHandler 创建警告模式处理器
func NewSensitiveWordsWarnHandler(filter SensitiveWordsFilterInterface) *SensitiveWordsProcessor {
	return NewSensitiveWordsProcessor(ModeWarn, filter)
}


