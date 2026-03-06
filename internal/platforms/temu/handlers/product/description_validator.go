package product

import (
	"fmt"
	"regexp"
	"strings"

	"task-processor/internal/pipeline"
	"task-processor/internal/platforms/temu/api/models"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// ProductDescriptionValidator 产品描述验证器
type ProductDescriptionValidator struct {
	logger *logrus.Entry
}

// DescriptionValidationResult 描述验证结果
type DescriptionValidationResult struct {
	OriginalDescription  string   `json:"original_description"`
	ValidatedDescription string   `json:"validated_description"`
	Violations           []string `json:"violations"`
	Suggestions          []string `json:"suggestions"`
	Length               int      `json:"length"`
	IsValid              bool     `json:"is_valid"`
	QualityScore         int      `json:"quality_score"`
}

// NewProductDescriptionValidator 创建新的产品描述验证器
func NewProductDescriptionValidator() *ProductDescriptionValidator {
	return &ProductDescriptionValidator{
		logger: logrus.WithField("handler", "ProductDescriptionValidator"),
	}
}

// Name 返回处理器名称
func (h *ProductDescriptionValidator) Name() string {
	return "产品描述验证处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *ProductDescriptionValidator) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *ProductDescriptionValidator) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始验证和优化产品描述")

	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	originalDesc := temuCtx.TemuProduct.GoodsExtensionInfo.GoodsDesc
	if originalDesc == "" {
		h.logger.Info("未找到产品描述，生成默认描述")
		defaultDesc := h.generateDefaultDescription(temuCtx, temuCtx.TemuProduct)
		temuCtx.TemuProduct.GoodsExtensionInfo.GoodsDesc = defaultDesc
		h.logger.Infof("生成了默认描述，长度: %d字符", len(defaultDesc))
		return nil
	}

	// 验证和优化描述
	result := h.validateAndOptimizeDescription(originalDesc, temuCtx, temuCtx.TemuProduct)

	// 记录处理结果
	if len(result.Violations) > 0 {
		h.logger.Warnf("产品描述存在违规内容: %v", result.Violations)
	}
	if len(result.Suggestions) > 0 {
		h.logger.Infof("产品描述优化建议: %v", result.Suggestions)
	}

	if originalDesc != result.ValidatedDescription {
		h.logger.Infof("原始描述长度: %d字符", len(originalDesc))
		h.logger.Infof("优化后描述长度: %d字符", len(result.ValidatedDescription))
		h.logger.Infof("质量评分: %d/100", result.QualityScore)
	}

	// 更新产品描述
	temuCtx.TemuProduct.GoodsExtensionInfo.GoodsDesc = result.ValidatedDescription

	h.logger.Infof("产品描述验证完成: 长度=%d字符, 质量评分=%d/100",
		result.Length, result.QualityScore)
	return nil
}

// validateAndOptimizeDescription 验证和优化产品描述
func (h *ProductDescriptionValidator) validateAndOptimizeDescription(description string, temuCtx *temucontext.TemuTaskContext, temuProduct *models.Product) *DescriptionValidationResult {
	result := &DescriptionValidationResult{
		OriginalDescription:  description,
		ValidatedDescription: description,
		Violations:           []string{},
		Suggestions:          []string{},
		IsValid:              true,
	}

	// 1. 清理和格式化
	cleaned := h.cleanAndFormatDescription(description, result)
	result.ValidatedDescription = cleaned

	// 2. 验证字符支持
	validated := h.validateCharacterSupport(cleaned, result)
	result.ValidatedDescription = validated

	// 3. 验证长度限制
	if len(validated) > 10000 {
		result.Violations = append(result.Violations, fmt.Sprintf("描述长度超过限制: %d > 10000字符", len(validated)))
		result.ValidatedDescription = h.truncateDescription(validated, 10000)
	}

	// 4. 增强描述内容
	enhanced := h.enhanceDescription(result.ValidatedDescription, temuCtx, temuProduct, result)
	result.ValidatedDescription = enhanced

	// 5. 计算质量评分
	result.QualityScore = h.calculateQualityScore(result.ValidatedDescription, temuCtx, temuProduct)

	// 6. 设置最终状态
	result.Length = len(result.ValidatedDescription)
	result.IsValid = len(result.Violations) == 0

	return result
}

// ValidateDescriptionAPI 调用TEMU API验证描述（如果需要）
func (h *ProductDescriptionValidator) ValidateDescriptionAPI(temuCtx *temucontext.TemuTaskContext, description string) error {
	// 这里可以调用TEMU的违规词汇检查API
	// temu.local.goods.illegal.vocabulary.check

	h.logger.Debugf("TODO: 调用TEMU API验证产品描述，长度: %d字符", len(description))

	// 暂时返回nil，实际实现时需要调用真实的API
	return nil
}

// generateDefaultDescription 生成默认产品描述
func (h *ProductDescriptionValidator) generateDefaultDescription(temuCtx *temucontext.TemuTaskContext, temuProduct *models.Product) string {
	productName := temuProduct.GoodsBasic.GoodsName

	// 基于产品名称生成默认描述
	defaultDesc := fmt.Sprintf("High-quality %s with excellent features and reliable performance. Perfect for various applications and designed to meet your needs.", productName)

	return defaultDesc
}

// cleanAndFormatDescription 清理和格式化描述
func (h *ProductDescriptionValidator) cleanAndFormatDescription(description string, result *DescriptionValidationResult) string {
	// 基本清理逻辑
	cleaned := strings.TrimSpace(description)

	// 移除多余的空格
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")

	return cleaned
}

// validateCharacterSupport 验证字符支持
func (h *ProductDescriptionValidator) validateCharacterSupport(description string, result *DescriptionValidationResult) string {
	// 移除不支持的字符（保留英文字母、数字和基本符号）
	var cleaned strings.Builder

	for _, r := range description {
		// 跳过中文字符
		if r >= 0x4e00 && r <= 0x9fff {
			continue
		}

		// 保留ASCII字符
		if r <= 127 {
			cleaned.WriteRune(r)
		}
	}

	return cleaned.String()
}

// truncateDescription 截断描述到指定长度
func (h *ProductDescriptionValidator) truncateDescription(description string, maxLength int) string {
	if len(description) <= maxLength {
		return description
	}

	// 截断到最大长度，但尝试在单词边界截断
	truncated := description[:maxLength-3]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLength/2 {
		truncated = truncated[:lastSpace]
	}

	return truncated + "..."
}

// enhanceDescription 增强描述内容
func (h *ProductDescriptionValidator) enhanceDescription(description string, temuCtx *temucontext.TemuTaskContext, temuProduct *models.Product, result *DescriptionValidationResult) string {
	// 基本增强逻辑
	return description
}

// calculateQualityScore 计算质量评分
func (h *ProductDescriptionValidator) calculateQualityScore(description string, temuCtx *temucontext.TemuTaskContext, temuProduct *models.Product) int {
	score := 50 // 基础分数

	// 根据长度评分
	length := len(description)
	if length >= 200 && length <= 2000 {
		score += 20
	}

	// 根据内容质量评分
	if strings.Contains(strings.ToLower(description), "quality") {
		score += 10
	}

	if score > 100 {
		score = 100
	}

	return score
}
