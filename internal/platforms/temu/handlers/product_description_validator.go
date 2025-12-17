package handlers

import (
	"fmt"

	"task-processor/internal/common/pipeline"

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

// Handle 处理任务
func (h *ProductDescriptionValidator) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始验证和优化产品描述")

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	originalDesc := ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc
	if originalDesc == "" {
		h.logger.Info("未找到产品描述，生成默认描述")
		defaultDesc := h.generateDefaultDescription(ctx)
		ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc = defaultDesc
		h.logger.Infof("生成了默认描述，长度: %d字符", len(defaultDesc))
		return nil
	}

	// 验证和优化描述
	result := h.validateAndOptimizeDescription(originalDesc, ctx)

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
	ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc = result.ValidatedDescription

	h.logger.Infof("产品描述验证完成: 长度=%d字符, 质量评分=%d/100",
		result.Length, result.QualityScore)
	return nil
}

// validateAndOptimizeDescription 验证和优化产品描述
func (h *ProductDescriptionValidator) validateAndOptimizeDescription(description string, ctx *pipeline.TaskContext) *DescriptionValidationResult {
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
	enhanced := h.enhanceDescription(result.ValidatedDescription, ctx, result)
	result.ValidatedDescription = enhanced

	// 5. 计算质量评分
	result.QualityScore = h.calculateQualityScore(result.ValidatedDescription, ctx)

	// 6. 设置最终状态
	result.Length = len(result.ValidatedDescription)
	result.IsValid = len(result.Violations) == 0

	return result
}

// ValidateDescriptionAPI 调用TEMU API验证描述（如果需要）
func (h *ProductDescriptionValidator) ValidateDescriptionAPI(ctx *pipeline.TaskContext, description string) error {
	// 这里可以调用TEMU的违规词汇检查API
	// temu.local.goods.illegal.vocabulary.check

	h.logger.Debugf("TODO: 调用TEMU API验证产品描述，长度: %d字符", len(description))

	// 暂时返回nil，实际实现时需要调用真实的API
	return nil
}
