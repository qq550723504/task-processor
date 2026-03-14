// Package productjson 提供产品JSON生成的应用层实现
package productjson

import (
	"context"
	"fmt"
	"sort"
	"strings"

	domain "task-processor/internal/domain/productjson"

	"github.com/sirupsen/logrus"
)

// EnhancementSuggester 增强建议器接口
type EnhancementSuggester interface {
	// GenerateSuggestions 生成数据增强建议
	GenerateSuggestions(ctx context.Context, validation *domain.ValidationResult) (*domain.EnhancementSuggestion, error)
	// PrioritizeSuggestions 对建议进行优先级排序
	PrioritizeSuggestions(suggestions []string) []string
}

// enhancementSuggester 增强建议器实现
type enhancementSuggester struct{}

// NewEnhancementSuggester 创建新的增强建议器
func NewEnhancementSuggester() EnhancementSuggester {
	return &enhancementSuggester{}
}

// GenerateSuggestions 生成数据增强建议
func (s *enhancementSuggester) GenerateSuggestions(ctx context.Context, validation *domain.ValidationResult) (*domain.EnhancementSuggestion, error) {
	if validation == nil {
		return nil, fmt.Errorf("validation result cannot be nil")
	}

	suggestion := &domain.EnhancementSuggestion{
		RequiredActions: make([]string, 0),
		OptionalActions: make([]string, 0),
	}

	// 分析图片质量
	if validation.ImageScore < 60 {
		suggestion.RequiredActions = append(suggestion.RequiredActions, "添加至少 3 张高质量产品图片")
	} else if validation.ImageScore < 80 {
		suggestion.OptionalActions = append(suggestion.OptionalActions, "添加更多产品图片（建议 3-4 张）")
	} else if validation.ImageScore < 100 {
		suggestion.OptionalActions = append(suggestion.OptionalActions, "添加更多产品图片（建议 5-8 张）以获得最佳效果")
	}

	// 分析文本质量
	if validation.TextScore < 30 {
		suggestion.RequiredActions = append(suggestion.RequiredActions, "提供至少 50 字符的产品描述")
	} else if validation.TextScore < 60 {
		suggestion.OptionalActions = append(suggestion.OptionalActions, "扩充产品描述至 100 字符以上")
	} else if validation.TextScore < 80 {
		suggestion.OptionalActions = append(suggestion.OptionalActions, "扩充产品描述至 200 字符以上以获得更好效果")
	}

	// 分析抓取数据质量
	if validation.ScrapedScore > 0 && validation.ScrapedScore < 60 {
		suggestion.OptionalActions = append(suggestion.OptionalActions, "提供更完整的产品链接或补充产品规格信息")
	}

	// 检查验证问题
	for _, issue := range validation.Issues {
		if issue.Severity == domain.SeverityError {
			suggestion.RequiredActions = append(suggestion.RequiredActions, fmt.Sprintf("修复错误: %s", issue.Message))
		}
	}

	// 对建议进行优先级排序
	suggestion.RequiredActions = s.PrioritizeSuggestions(suggestion.RequiredActions)
	suggestion.OptionalActions = s.PrioritizeSuggestions(suggestion.OptionalActions)

	// 估算改进后的质量等级
	suggestion.EstimatedQuality = s.estimateQualityAfterImprovement(validation)

	logrus.WithFields(logrus.Fields{
		"required_actions":  len(suggestion.RequiredActions),
		"optional_actions":  len(suggestion.OptionalActions),
		"estimated_quality": suggestion.EstimatedQuality,
	}).Info("enhancement suggestions generated")

	return suggestion, nil
}

// PrioritizeSuggestions 对建议进行优先级排序
func (s *enhancementSuggester) PrioritizeSuggestions(suggestions []string) []string {
	if len(suggestions) <= 1 {
		return suggestions
	}

	// 创建带优先级的建议列表
	type prioritizedSuggestion struct {
		text     string
		priority int
	}

	prioritized := make([]prioritizedSuggestion, len(suggestions))
	for i, suggestion := range suggestions {
		priority := s.calculatePriority(suggestion)
		prioritized[i] = prioritizedSuggestion{
			text:     suggestion,
			priority: priority,
		}
	}

	// 按优先级排序（降序）
	sort.Slice(prioritized, func(i, j int) bool {
		return prioritized[i].priority > prioritized[j].priority
	})

	// 提取排序后的建议
	sorted := make([]string, len(prioritized))
	for i, p := range prioritized {
		sorted[i] = p.text
	}

	return sorted
}

// calculatePriority 计算建议的优先级
func (s *enhancementSuggester) calculatePriority(suggestion string) int {
	priority := 0
	if strings.Contains(suggestion, "图片") {
		priority += 100
	}
	if strings.Contains(suggestion, "修复错误") {
		priority += 90
	}
	if strings.Contains(suggestion, "至少") || strings.Contains(suggestion, "提供") {
		priority += 80
	}
	if strings.Contains(suggestion, "描述") {
		priority += 70
	}
	if strings.Contains(suggestion, "建议") || strings.Contains(suggestion, "更好") {
		priority += 50
	}
	return priority
}

// estimateQualityAfterImprovement 估算改进后的质量等级
func (s *enhancementSuggester) estimateQualityAfterImprovement(validation *domain.ValidationResult) string {
	estimatedScore := validation.QualityScore + 20.0
	if estimatedScore >= 80 {
		return "高质量（完整处理）"
	} else if estimatedScore >= 60 {
		return "中等质量（基础处理）"
	} else if estimatedScore >= 50 {
		return "基础质量（最小处理）"
	}
	return "质量不足（可能仍被拒绝）"
}
