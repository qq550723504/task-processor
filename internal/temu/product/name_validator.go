package product

import (
	"fmt"
	"regexp"
	"strings"

	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/temu/context"

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
		logger:    logrus.WithField("handler", "ProductNameValidator"),
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

	// 验证括号前是否有空格（TEMU要求）
	if strings.Contains(optimizedName, "(") {
		// 检查是否有括号前没有空格的情况
		if regexp.MustCompile(`\S\(`).MatchString(optimizedName) {
			h.logger.Warnf("⚠️ 检测到括号前缺少空格，正在修复...")
			optimizedName = regexp.MustCompile(`(\S)\(`).ReplaceAllString(optimizedName, "$1 (")
			h.logger.Infof("✅ 已修复括号前的空格问题")
		}
	}

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
	var violations []string
	cleaned := name

	// 1. 检查和移除不支持的装饰字符
	decorativeChars := []string{"~", "!", "*", "$", "?", "_", "~", "{", "}", "#", "<", ">", "|", "*", ";", "^", "¬", "¦"}
	for _, char := range decorativeChars {
		if strings.Contains(cleaned, char) {
			violations = append(violations, fmt.Sprintf("包含不支持的装饰字符: %s", char))
			cleaned = strings.ReplaceAll(cleaned, char, "")
		}
	}

	// 2. 检查和移除高ASCII字符（如®, ©, ™等）
	cleaned = h.removeHighASCIIChars(cleaned, &violations)

	// 3. 验证只包含英文字母、数字和支持的符号
	cleaned = h.validateAllowedChars(cleaned, &violations)

	// 4. 清理多余的空格
	cleaned = h.cleanSpaces(cleaned)

	// 5. 检查长度限制
	if len(cleaned) > 500 {
		violations = append(violations, fmt.Sprintf("超过500字符限制: %d", len(cleaned)))
	}

	return cleaned, violations
}

// removeHighASCIIChars 移除高ASCII字符和中文字符
func (h *ProductNameValidator) removeHighASCIIChars(text string, violations *[]string) string {
	var result strings.Builder
	hasHighASCII := false
	hasChinese := false

	for _, r := range text {
		// 检查是否为中文字符
		if r >= 0x4e00 && r <= 0x9fff {
			hasChinese = true
			continue // 跳过中文字符
		}

		// 检查是否为高ASCII字符（如®, ©, ™等）
		if r > 127 {
			// 特殊处理一些常见的高ASCII字符
			switch r {
			case '®':
				result.WriteString("(R)")
				hasHighASCII = true
			case '©':
				result.WriteString("(C)")
				hasHighASCII = true
			case '™':
				result.WriteString("(TM)")
				hasHighASCII = true
			default:
				// 其他高ASCII字符直接移除
				hasHighASCII = true
			}
		} else {
			result.WriteRune(r)
		}
	}

	if hasChinese {
		*violations = append(*violations, "包含中文字符（已移除）")
	}
	if hasHighASCII {
		*violations = append(*violations, "包含高ASCII字符（已转换或移除）")
	}

	return result.String()
}

// validateAllowedChars 验证允许的字符（英文字母、数字和支持的符号）
func (h *ProductNameValidator) validateAllowedChars(text string, violations *[]string) string {
	// 允许的字符：英文字母、数字、空格和一些基本符号（不包括中文）
	allowedPattern := regexp.MustCompile(`[^a-zA-Z0-9\s\-\+\=\(\)\[\]\.\,\:\/"'&%@]+`)

	if allowedPattern.MatchString(text) {
		*violations = append(*violations, "包含不支持的字符")
		// 移除不允许的字符（包括中文）
		text = allowedPattern.ReplaceAllString(text, " ")
	}

	return text
}

// cleanSpaces 清理多余的空格
func (h *ProductNameValidator) cleanSpaces(text string) string {
	// 移除首尾空格
	text = strings.TrimSpace(text)

	// 将多个连续空格替换为单个空格
	spacePattern := regexp.MustCompile(`\s+`)
	text = spacePattern.ReplaceAllString(text, " ")

	// 确保左括号前有空格（TEMU要求：左括号前必须有空格）
	// 注意：这个必须在移除标点符号前的空格之前执行
	text = regexp.MustCompile(`(\S)\(`).ReplaceAllString(text, "$1 (")

	// 确保右括号后有空格（如果后面还有字符的话）
	text = regexp.MustCompile(`\)(\S)`).ReplaceAllString(text, ") $1")

	// 移除逗号前的空格（TEMU要求：逗号前不能有空格）
	text = regexp.MustCompile(`\s+,`).ReplaceAllString(text, ",")

	// 移除其他标点符号前的空格（但不包括括号）
	text = regexp.MustCompile(`\s+([.!?;:])`).ReplaceAllString(text, "$1")

	return text
}

// ValidateProductNameAPI 调用TEMU API验证产品名称（如果需要）
func (h *ProductNameValidator) ValidateProductNameAPI(ctx pipeline.TaskContext, productName string) error {
	// 这里可以调用TEMU的违规词汇检查API
	// temu.local.goods.illegal.vocabulary.check

	h.logger.Debugf("TODO: 调用TEMU API验证产品名称: %s", productName)

	// 暂时返回nil，实际实现时需要调用真实的API
	return nil
}
