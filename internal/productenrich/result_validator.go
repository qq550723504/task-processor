// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"
	"task-processor/internal/core/logger"
)

// ResultValidator 结果验证器接口
type ResultValidator interface {
	// ValidateResult 验证生成结果
	ValidateResult(ctx context.Context, input *ParsedInput, result *ProductJSON) (*ResultValidation, error)
	// CheckImageConsistency 检查图片一致性
	CheckImageConsistency(inputImages []string, resultImages []string) bool
	// CheckKeywordMatch 检查关键词匹配度
	CheckKeywordMatch(inputText string, resultTitle string, resultDescription string) (float64, error)
	// CheckCompleteness 检查完整性
	CheckCompleteness(result *ProductJSON) (*CompletenessReport, error)
}

// resultValidator 结果验证器实现
type resultValidator struct {
}

// NewResultValidator 创建新的结果验证器
func NewResultValidator() ResultValidator {
	return &resultValidator{}
}

// ValidateResult 验证生成结果
func (v *resultValidator) ValidateResult(ctx context.Context, input *ParsedInput, result *ProductJSON) (*ResultValidation, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}
	if result == nil {
		return nil, fmt.Errorf("result cannot be nil")
	}

	validation := &ResultValidation{
		IsValid: true,
		Issues:  make([]ValidationIssue, 0),
	}

	// 检查图片一致性
	validation.ImageConsistency = v.CheckImageConsistency(input.Images, result.Images)
	if !validation.ImageConsistency {
		validation.Issues = append(validation.Issues, ValidationIssue{
			Field:    "images",
			Severity: SeverityWarning,
			Message:  "生成结果缺少部分输入图片",
			Code:     "IMAGE_COUNT_MISMATCH",
		})
	}

	// 检查关键词匹配度
	keywordScore, err := v.CheckKeywordMatch(input.Text, result.Title, result.Description)
	if err != nil {
		logrus.WithError(err).Error("failed to check keyword match")
	} else {
		validation.KeywordMatchScore = keywordScore
		if keywordScore < 0.3 {
			validation.Issues = append(validation.Issues, ValidationIssue{
				Field:    "content",
				Severity: SeverityWarning,
				Message:  "生成的内容与原始文本关键词匹配度较低",
				Code:     "LOW_KEYWORD_MATCH",
			})
		}
	}

	// 检查完整性
	completeness, err := v.CheckCompleteness(result)
	if err != nil {
		logrus.WithError(err).Error("failed to check completeness")
	} else {
		validation.CompletenessScore = completeness.Score
		if len(completeness.MissingRequired) > 0 {
			validation.IsValid = false
			for _, field := range completeness.MissingRequired {
				validation.Issues = append(validation.Issues, ValidationIssue{
					Field:    field,
					Severity: SeverityError,
					Message:  fmt.Sprintf("必需字段 %s 缺失或为空", field),
					Code:     "MISSING_REQUIRED_FIELD",
				})
			}
		}
	}

	logger.GetGlobalLogger("productenrich/result_validator.go").WithFields(logrus.Fields{
		"is_valid":            validation.IsValid,
		"keyword_match_score": validation.KeywordMatchScore,
		"completeness_score":  validation.CompletenessScore,
		"issues_count":        len(validation.Issues),
	}).Info("result validation completed")

	return validation, nil
}

// CheckImageConsistency 检查图片一致性：结果图片应包含所有输入图片
func (v *resultValidator) CheckImageConsistency(inputImages []string, resultImages []string) bool {
	if len(inputImages) == 0 {
		return true
	}
	resultSet := make(map[string]struct{}, len(resultImages))
	for _, u := range resultImages {
		resultSet[u] = struct{}{}
	}
	for _, u := range inputImages {
		if _, ok := resultSet[u]; !ok {
			return false
		}
	}
	return true
}

// CheckKeywordMatch 检查关键词匹配度
func (v *resultValidator) CheckKeywordMatch(inputText string, resultTitle string, resultDescription string) (float64, error) {
	if inputText == "" {
		// 如果没有输入文本，返回 1.0（无需匹配）
		return 1.0, nil
	}

	// 提取输入文本的关键词
	inputKeywords := v.extractKeywords(inputText)
	if len(inputKeywords) == 0 {
		return 1.0, nil
	}

	// 合并标题和描述
	resultText := resultTitle + " " + resultDescription
	resultTextLower := strings.ToLower(resultText)

	// 计算匹配的关键词数量
	matchCount := 0
	for _, keyword := range inputKeywords {
		if strings.Contains(resultTextLower, strings.ToLower(keyword)) {
			matchCount++
		}
	}

	// 计算匹配度
	matchScore := float64(matchCount) / float64(len(inputKeywords))

	return matchScore, nil
}

// extractKeywords 提取关键词
func (v *resultValidator) extractKeywords(text string) []string {
	if looksLikeCJK(text) {
		return extractCJKKeywords(text)
	}

	// 简单实现：按空格分割并过滤短词
	words := strings.Fields(text)
	keywords := make([]string, 0)

	for _, word := range words {
		// 过滤长度小于 2 的词
		if len(word) >= 2 {
			keywords = append(keywords, word)
		}
	}

	// 最多返回 20 个关键词
	if len(keywords) > 20 {
		keywords = keywords[:20]
	}

	return keywords
}

func looksLikeCJK(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func extractCJKKeywords(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	keywords := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if utf8RuneCount(field) < 2 {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		keywords = append(keywords, field)
		if len(keywords) >= 20 {
			break
		}
	}
	return keywords
}

func utf8RuneCount(s string) int {
	count := 0
	for range s {
		count++
	}
	return count
}

// CheckCompleteness 检查完整性
func (v *resultValidator) CheckCompleteness(result *ProductJSON) (*CompletenessReport, error) {
	if result == nil {
		return nil, fmt.Errorf("result cannot be nil")
	}

	report := &CompletenessReport{
		RequiredFields:  make(map[string]bool),
		OptionalFields:  make(map[string]bool),
		MissingRequired: make([]string, 0),
		MissingOptional: make([]string, 0),
	}

	// 检查必需字段（images 为可选，取决于输入是否包含图片）
	report.RequiredFields["title"] = result.Title != ""
	report.RequiredFields["category"] = len(result.Category) > 0
	report.RequiredFields["description"] = result.Description != ""

	// 检查可选字段
	report.OptionalFields["images"] = len(result.Images) > 0
	report.OptionalFields["attributes"] = len(result.Attributes) > 0
	report.OptionalFields["selling_points"] = len(result.SellingPoints) > 0
	report.OptionalFields["seo_keywords"] = len(result.SEOKeywords) > 0
	report.OptionalFields["specifications"] = result.Specifications != nil
	report.OptionalFields["variants"] = len(result.Variants) > 0

	// 收集缺失的字段
	for field, exists := range report.RequiredFields {
		if !exists {
			report.MissingRequired = append(report.MissingRequired, field)
		}
	}

	for field, exists := range report.OptionalFields {
		if !exists {
			report.MissingOptional = append(report.MissingOptional, field)
		}
	}

	// 计算完整性评分
	totalFields := len(report.RequiredFields) + len(report.OptionalFields)
	presentFields := 0

	for _, exists := range report.RequiredFields {
		if exists {
			presentFields++
		}
	}

	for _, exists := range report.OptionalFields {
		if exists {
			presentFields++
		}
	}

	report.Score = float64(presentFields) / float64(totalFields) * 100

	return report, nil
}
