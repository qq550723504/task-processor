package rules

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// BulletPointsValidator 产品要点验证器
type BulletPointsValidator struct {
	logger    *logrus.Entry
	optimizer *BulletPointsOptimizer
}

// BulletPointValidationResult 要点验证结果
type BulletPointValidationResult struct {
	OriginalPoints  []string `json:"original_points"`
	ValidatedPoints []string `json:"validated_points"`
	Violations      []string `json:"violations"`
	Suggestions     []string `json:"suggestions"`
	TotalLength     int      `json:"total_length"`
	IsValid         bool     `json:"is_valid"`
}

// NewBulletPointsValidator 创建新的产品要点验证器
func NewBulletPointsValidator() *BulletPointsValidator {
	return &BulletPointsValidator{
		logger:    logrus.WithField("handler", "BulletPointsValidator"),
		optimizer: NewBulletPointsOptimizer(),
	}
}

// Name 返回处理器名称
func (h *BulletPointsValidator) Name() string {
	return "产品要点验证处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *BulletPointsValidator) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *BulletPointsValidator) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始验证和优化产品要点")

	// 从强类型上下文获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	temuProduct := temuCtx.TemuProduct

	originalPoints := temuProduct.GoodsExtensionInfo.BulletPoints
	if len(originalPoints) == 0 {
		return fmt.Errorf("TEMU产品卖点为空")
	}

	// 验证和优化要点
	result := h.validateAndOptimizeBulletPoints(originalPoints)

	// 使用高级优化器进一步优化
	productName := temuProduct.GoodsBasic.GoodsName
	optimizedPoints, _ := h.optimizer.OptimizeForTemu(result.ValidatedPoints, productName)

	// 更新产品要点
	temuProduct.GoodsExtensionInfo.BulletPoints = optimizedPoints

	// 计算最终长度
	finalLength := 0
	for _, point := range optimizedPoints {
		finalLength += len(point)
	}

	h.logger.Infof("产品要点验证完成: %d个要点, 总长度: %d字符",
		len(optimizedPoints), finalLength)
	return nil
}

// validateAndOptimizeBulletPoints 验证和优化产品要点
func (h *BulletPointsValidator) validateAndOptimizeBulletPoints(points []string) *BulletPointValidationResult {
	result := &BulletPointValidationResult{
		OriginalPoints:  points,
		ValidatedPoints: []string{},
		Violations:      []string{},
		Suggestions:     []string{},
		IsValid:         true,
	}

	// 1. 检查要点数量限制
	if len(points) > 6 {
		result.Violations = append(result.Violations, fmt.Sprintf("要点数量超过限制: %d > 6", len(points)))
		points = points[:6] // 截取前6个
	}

	// 2. 处理每个要点
	for i, point := range points {
		cleanedPoint := h.cleanAndValidatePoint(point, i, result)
		if cleanedPoint != "" {
			result.ValidatedPoints = append(result.ValidatedPoints, cleanedPoint)
		}
	}

	// 3. 检查总长度
	totalLength := h.calculateTotalLength(result.ValidatedPoints)
	result.TotalLength = totalLength

	if totalLength > 700 {
		result.Violations = append(result.Violations, fmt.Sprintf("总字符数超过限制: %d > 700", totalLength))
		result.ValidatedPoints = h.truncateToLimit(result.ValidatedPoints, 700)
		result.TotalLength = h.calculateTotalLength(result.ValidatedPoints)
	}

	// 4. 检查要点质量
	h.validatePointsQuality(result)

	// 5. 设置验证状态
	result.IsValid = len(result.Violations) == 0

	return result
}

// cleanAndValidatePoint 清理和验证单个要点
func (h *BulletPointsValidator) cleanAndValidatePoint(point string, index int, result *BulletPointValidationResult) string {
	// 移除首尾空格
	point = strings.TrimSpace(point)

	if point == "" {
		result.Violations = append(result.Violations, fmt.Sprintf("要点[%d]为空", index+1))
		return ""
	}

	// 移除不支持的字符（只保留英文字母、数字和符号）
	cleaned := h.removeUnsupportedChars(point)
	if cleaned != point {
		result.Violations = append(result.Violations, fmt.Sprintf("要点[%d]包含不支持的字符", index+1))
		point = cleaned
	}

	// 优化要点格式
	optimized := h.optimizePointFormat(point)
	if optimized != point {
		result.Suggestions = append(result.Suggestions, fmt.Sprintf("要点[%d]格式已优化", index+1))
		point = optimized
	}

	// 检查长度（单个要点建议不超过120字符）
	if len(point) > 120 {
		result.Suggestions = append(result.Suggestions, fmt.Sprintf("要点[%d]过长，建议简化", index+1))
	}

	return point
}

// removeUnsupportedChars 移除不支持的字符（包括中文）
func (h *BulletPointsValidator) removeUnsupportedChars(text string) string {
	var result strings.Builder

	for _, r := range text {
		// 跳过中文字符
		if r >= 0x4e00 && r <= 0x9fff {
			continue
		}

		// 保留英文字母、数字、空格和基本符号（仅ASCII范围）
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			unicode.IsSpace(r) || strings.ContainsRune(".,!?()-[]/:;\"'&%@+=$", r) {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// optimizePointFormat 优化要点格式
func (h *BulletPointsValidator) optimizePointFormat(point string) string {
	// 确保要点以大写字母开头
	if len(point) > 0 {
		if len(point) == 1 {
			point = strings.ToUpper(point)
		} else {
			point = strings.ToUpper(string(point[0])) + point[1:]
		}
	}

	// 移除末尾的句号（要点通常不需要句号）
	point = strings.TrimSuffix(point, ".")

	// 清理多余的空格
	spacePattern := regexp.MustCompile(`\s+`)
	point = spacePattern.ReplaceAllString(point, " ")

	// 移除逗号前的空格（TEMU要求：逗号前不能有空格）
	point = regexp.MustCompile(`\s+,`).ReplaceAllString(point, ",")

	// 移除其他标点符号前的空格
	point = regexp.MustCompile(`\s+([.!?;:])`).ReplaceAllString(point, "$1")

	// 确保左括号前有空格（TEMU要求：左括号前必须有空格）
	point = regexp.MustCompile(`(\S)\(`).ReplaceAllString(point, "$1 (")

	// 确保右括号后有空格（如果后面还有字符的话）
	point = regexp.MustCompile(`\)(\S)`).ReplaceAllString(point, ") $1")

	// 移除首尾空格
	point = strings.TrimSpace(point)

	return point
}

// calculateTotalLength 计算总长度
func (h *BulletPointsValidator) calculateTotalLength(points []string) int {
	total := 0
	for _, point := range points {
		total += len(point)
	}
	return total
}

// truncateToLimit 截断到指定长度限制
func (h *BulletPointsValidator) truncateToLimit(points []string, limit int) []string {
	var result []string
	currentLength := 0

	for _, point := range points {
		if currentLength+len(point) <= limit {
			result = append(result, point)
			currentLength += len(point)
		} else {
			// 尝试截断当前要点
			remainingLength := limit - currentLength
			if remainingLength > 20 { // 至少保留20个字符才有意义
				truncated := point[:remainingLength-3] + "..."
				result = append(result, truncated)
			}
			break
		}
	}

	return result
}

// validatePointsQuality 验证要点质量
func (h *BulletPointsValidator) validatePointsQuality(result *BulletPointValidationResult) {
	points := result.ValidatedPoints

	// 检查是否有重复要点
	seen := make(map[string]bool)
	for i, point := range points {
		normalized := strings.ToLower(strings.TrimSpace(point))
		if seen[normalized] {
			result.Suggestions = append(result.Suggestions, fmt.Sprintf("要点[%d]可能重复", i+1))
		}
		seen[normalized] = true
	}

	// 检查要点是否过短
	for i, point := range points {
		if len(strings.TrimSpace(point)) < 10 {
			result.Suggestions = append(result.Suggestions, fmt.Sprintf("要点[%d]过短，建议增加描述", i+1))
		}
	}

	// 检查是否包含关键特性
	hasFeatures := h.checkForKeyFeatures(points)
	if !hasFeatures {
		result.Suggestions = append(result.Suggestions, "建议添加产品的关键特性和优势")
	}
}

// checkForKeyFeatures 检查是否包含关键特性
func (h *BulletPointsValidator) checkForKeyFeatures(points []string) bool {
	keyFeatureWords := []string{
		"ergonomic", "comfortable", "durable", "adjustable", "high-quality",
		"premium", "professional", "easy", "convenient", "safe", "reliable",
		"efficient", "lightweight", "portable", "waterproof", "breathable",
	}

	allText := strings.ToLower(strings.Join(points, " "))

	for _, word := range keyFeatureWords {
		if strings.Contains(allText, word) {
			return true
		}
	}

	return false
}

// arePointsEqual 比较两个要点列表是否相等
func (h *BulletPointsValidator) arePointsEqual(points1, points2 []string) bool {
	if len(points1) != len(points2) {
		return false
	}

	for i, point1 := range points1 {
		if point1 != points2[i] {
			return false
		}
	}

	return true
}

// ValidateBulletPointsAPI 调用TEMU API验证要点（如果需要）
func (h *BulletPointsValidator) ValidateBulletPointsAPI(ctx pipeline.TaskContext, bulletPoints []string) error {
	// 这里可以调用TEMU的违规词汇检查API
	// temu.local.goods.illegal.vocabulary.check

	h.logger.Debugf("TODO: 调用TEMU API验证产品要点: %v", bulletPoints)

	// 暂时返回nil，实际实现时需要调用真实的API
	return nil
}
