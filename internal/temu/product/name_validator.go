package product

import (
	"fmt"
	"regexp"
	"strings"

	temupublishing "task-processor/internal/marketplace/temu/publishing"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/temu/context"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ProductNameValidator 产品名称验证器
type ProductNameValidator struct {
	logger    *logrus.Entry
	optimizer *ProductNameOptimizer
}

// NewProductNameValidator 创建新的产品名称验证器
func NewProductNameValidator() *ProductNameValidator {
	return &ProductNameValidator{
		logger:    logger.GetGlobalLogger("ProductNameValidator"),
		optimizer: NewProductNameOptimizer(),
	}
}

// Name 返回处理器名称
func (h *ProductNameValidator) Name() string {
	return "产品名称验证处理器"
}

// HandleTemu 处理TEMU任务（实现TemuHandler接口）
func (h *ProductNameValidator) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始验证和清理产品名称")

	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	temuProduct := temuCtx.TemuProduct
	originalName := temuProduct.GoodsBasic.GoodsName
	if originalName == "" {
		return fmt.Errorf("产品名称不能为空")
	}

	// 验证和清理产品名称
	cleanedName, violations := h.validateAndCleanProductName(originalName)

	// 优化产品名称
	optimizedName, optimizations := h.optimizer.OptimizeProductName(cleanedName)

	// 记录处理结果
	if len(violations) > 0 {
		h.logger.Warnf("产品名称存在违规内容: %v", violations)
	}
	if len(optimizations) > 0 {
		h.logger.Infof("产品名称优化: %v", optimizations)
	}

	if originalName != optimizedName {
		h.logger.Infof("原始名称: %s", originalName)
		h.logger.Infof("最终名称: %s", optimizedName)
	}

	optimizedName = h.normalizeOptimizedName(optimizedName)

	// 更新产品名称
	temuProduct.GoodsBasic.GoodsName = optimizedName

	// 验证最终名称长度
	if len(optimizedName) > 500 {
		h.logger.Warnf("产品名称超过500字符限制，进行截断: %d -> 500", len(optimizedName))
		temuProduct.GoodsBasic.GoodsName = optimizedName[:500]
	}

	h.logger.Infof("产品名称验证完成: %s", temuProduct.GoodsBasic.GoodsName)
	return nil
}

// Handle 兼容原有的Handler接口（用于pipeline.AddHandler）
func (h *ProductNameValidator) Handle(ctx pipeline.TaskContext) error {
	// 尝试类型断言为TemuTaskContext
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return h.HandleTemu(temuCtx)
	}
	// 如果不是TemuTaskContext，返回错误
	return fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
}

// validateAndCleanProductName 验证和清理产品名称
func (h *ProductNameValidator) validateAndCleanProductName(name string) (string, []string) {
	result := temupublishing.SanitizeProductName(name)
	return result.Name, result.Violations
}

// cleanSpaces 清理多余的空格
func (h *ProductNameValidator) cleanSpaces(text string) string {
	return temupublishing.NormalizeProductSubmissionName(text)
}

func (h *ProductNameValidator) normalizeOptimizedName(name string) string {
	needsParenthesisFix := strings.Contains(name, "(") && regexp.MustCompile(`\S\(`).MatchString(name)
	if needsParenthesisFix && h.logger != nil {
		h.logger.Warnf("⚠️ 检测到括号前缺少空格，正在修复...")
	}

	normalized := temupublishing.NormalizeProductSubmissionName(name)
	if needsParenthesisFix && h.logger != nil && normalized != name {
		h.logger.Infof("✅ 已修复括号前的空格问题")
	}
	return normalized
}

// ValidateProductNameAPI 调用TEMU API验证产品名称（如果需要）
func (h *ProductNameValidator) ValidateProductNameAPI(ctx pipeline.TaskContext, productName string) error {
	// 这里可以调用TEMU的违规词汇检查API
	// temu.local.goods.illegal.vocabulary.check

	h.logger.Debugf("TODO: 调用TEMU API验证产品名称: %s", productName)

	// 暂时返回nil，实际实现时需要调用真实的API
	return nil
}
