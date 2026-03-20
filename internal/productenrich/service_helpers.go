// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// parseInput 解析输入
func (s *productService) parseInput(ctx context.Context, task *Task) (*ParsedInput, error) {
	logrus.WithField("task_id", task.ID).Info("step 1: parsing input")

	if s.inputParser != nil {
		parsedInput, err := s.inputParser.ParseInput(ctx, task.Request)
		if err != nil {
			logrus.WithField("task_id", task.ID).WithError(err).Error("failed to parse input")
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
		logrus.WithFields(logrus.Fields{
			"task_id":     task.ID,
			"images":      len(parsedInput.Images),
			"text_length": len(parsedInput.Text),
		}).Info("input parsed successfully")
		return parsedInput, nil
	}

	// 如果没有 InputParser，创建一个简单的 ParsedInput
	logrus.WithField("task_id", task.ID).Warn("input parser not configured, using simple parsing")
	return &ParsedInput{
		Images: task.Request.ImageURLs,
		Text:   task.Request.Text,
	}, nil
}

// validateAndSelectStrategy 验证输入并选择策略
func (s *productService) validateAndSelectStrategy(ctx context.Context, task *Task, parsedInput *ParsedInput) (ProcessingStrategy, error) {
	if s.inputValidator == nil || s.qualityScorer == nil || s.strategySelector == nil {
		logrus.WithField("task_id", task.ID).Warn("validation components not configured, using full strategy")
		return StrategyFull, nil
	}

	logrus.WithField("task_id", task.ID).Info("step 2: validating input data")

	// 验证输入
	validationResult, err := s.inputValidator.Validate(ctx, parsedInput)
	if err != nil {
		logrus.WithField("task_id", task.ID).WithError(err).Error("failed to validate input")
		if dbErr := s.taskRepo.UpdateTaskError(ctx, task.ID, fmt.Sprintf("input validation failed: %v", err)); dbErr != nil {
			logrus.WithField("task_id", task.ID).WithError(dbErr).Error("failed to persist task error")
		}
		return "", fmt.Errorf("failed to validate input: %w", err)
	}

	// 计算质量评分
	qualityScore, err := s.qualityScorer.CalculateScore(ctx, validationResult)
	if err != nil {
		logrus.WithField("task_id", task.ID).WithError(err).Error("failed to calculate quality score")
		if dbErr := s.taskRepo.UpdateTaskError(ctx, task.ID, fmt.Sprintf("quality scoring failed: %v", err)); dbErr != nil {
			logrus.WithField("task_id", task.ID).WithError(dbErr).Error("failed to persist task error")
		}
		return "", fmt.Errorf("failed to calculate quality score: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"task_id":     task.ID,
		"score":       qualityScore,
		"image_score": validationResult.ImageScore,
		"text_score":  validationResult.TextScore,
	}).Info("quality score calculated")

	// 选择处理策略
	strategy, err := s.strategySelector.SelectStrategy(ctx, qualityScore)
	if err != nil {
		logrus.WithField("task_id", task.ID).WithError(err).Error("failed to select strategy")
		if dbErr := s.taskRepo.UpdateTaskError(ctx, task.ID, fmt.Sprintf("strategy selection failed: %v", err)); dbErr != nil {
			logrus.WithField("task_id", task.ID).WithError(dbErr).Error("failed to persist task error")
		}
		return "", fmt.Errorf("failed to select strategy: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"task_id":  task.ID,
		"strategy": string(strategy),
	}).Info("processing strategy selected")

	// 如果策略是拒绝，生成改进建议并返回错误
	if strategy == StrategyReject {
		return s.handleRejection(ctx, task, validationResult)
	}

	return strategy, nil
}

// handleRejection 处理拒绝策略，返回 errNoRetry 标记不可重试
func (s *productService) handleRejection(ctx context.Context, task *Task, validationResult *ValidationResult) (ProcessingStrategy, error) {
	var errorMsg string
	if s.enhancementSuggester != nil {
		suggestion, err := s.enhancementSuggester.GenerateSuggestions(ctx, validationResult)
		if err != nil {
			logrus.WithField("task_id", task.ID).WithError(err).Error("failed to generate suggestions")
		} else {
			errorMsg = s.buildRejectionMessage(validationResult, suggestion)
		}
	}
	if errorMsg == "" {
		errorMsg = fmt.Sprintf("数据质量不足（评分: %.2f），无法生成产品信息", validationResult.QualityScore)
	}
	if dbErr := s.taskRepo.UpdateTaskError(ctx, task.ID, errorMsg); dbErr != nil {
		logrus.WithField("task_id", task.ID).WithError(dbErr).Error("failed to persist rejection error")
	}
	return "", &errNoRetry{cause: fmt.Errorf("%s", errorMsg)}
}

// analyzeProduct 分析产品
func (s *productService) analyzeProduct(ctx context.Context, task *Task, parsedInput *ParsedInput) (*ProductAnalysis, error) {
	logrus.WithField("task_id", task.ID).Info("step 3: analyzing product")

	if s.productUnderstanding != nil {
		analysis, err := s.productUnderstanding.AnalyzeProduct(ctx, parsedInput)
		if err != nil {
			logrus.WithField("task_id", task.ID).WithError(err).Error("failed to analyze product")
			return nil, fmt.Errorf("failed to analyze product: %w", err)
		}
		logrus.WithField("task_id", task.ID).Info("product analyzed successfully")
		return analysis, nil
	}

	// 如果没有 ProductUnderstanding，创建一个简单的分析结果
	logrus.WithField("task_id", task.ID).Warn("product understanding not configured, using simple analysis")
	return &ProductAnalysis{
		Representation: &ProductRepresentation{
			ProductType: "Unknown Product",
			Attributes:  make(map[string]string),
			Features:    []string{},
		},
	}, nil
}

// generateProductJSON 生成产品 JSON
func (s *productService) generateProductJSON(ctx context.Context, task *Task, analysis *ProductAnalysis, strategy ProcessingStrategy) (*ProductJSON, error) {
	logrus.WithField("task_id", task.ID).Info("step 4: generating product JSON")

	// 根据策略控制生成范围：
	//   full    → 规格 + 变体
	//   basic   → 规格，跳过变体
	//   minimal → 跳过规格和变体
	variantGen := s.variantGenerator
	skipVariants := false
	switch strategy {
	case StrategyBasic:
		skipVariants = true // 保留规格生成，跳过变体
	case StrategyMinimal:
		variantGen = nil // 规格和变体都跳过
	}

	if s.jsonGenerator != nil {
		productJSON, err := s.jsonGenerator.GenerateJSON(ctx, analysis, variantGen, skipVariants)
		if err != nil {
			logrus.WithField("task_id", task.ID).WithError(err).Error("failed to generate JSON")
			return nil, fmt.Errorf("failed to generate JSON: %w", err)
		}
		logrus.WithField("task_id", task.ID).Info("product JSON generated successfully")
		return productJSON, nil
	}

	// 如果没有 JSONGenerator，创建一个简单的结果
	logrus.WithField("task_id", task.ID).Warn("JSON generator not configured, using simple generation")
	return &ProductJSON{
		Title:         "Sample Product",
		Category:      []string{"General", "Product"},
		Attributes:    make(map[string]string),
		SellingPoints: []string{"High Quality", "Great Value"},
		SEOKeywords:   []string{"product", "quality", "value"},
		Description:   "This is a sample product.",
		Images:        task.Request.ImageURLs,
	}, nil
}

// validateResult 验证生成结果，若结果无效则返回错误阻止保存。
func (s *productService) validateResult(ctx context.Context, task *Task, parsedInput *ParsedInput, productJSON *ProductJSON) error {
	if s.resultValidator == nil {
		return nil
	}

	logrus.WithField("task_id", task.ID).Info("step 5: validating result")
	resultValidation, err := s.resultValidator.ValidateResult(ctx, parsedInput, productJSON)
	if err != nil {
		logrus.WithField("task_id", task.ID).WithError(err).Error("failed to validate result")
		return fmt.Errorf("result validation error: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"task_id":      task.ID,
		"is_valid":     resultValidation.IsValid,
		"issues_count": len(resultValidation.Issues),
	}).Info("result validation completed")

	// 记录所有验证问题
	for _, issue := range resultValidation.Issues {
		logrus.WithFields(logrus.Fields{
			"task_id":  task.ID,
			"field":    issue.Field,
			"severity": issue.Severity,
			"message":  issue.Message,
		}).Warn("result validation issue")
	}

	if !resultValidation.IsValid {
		return &errNoRetry{cause: fmt.Errorf("生成结果验证失败，产品数据不完整，请补充输入后重试")}
	}
	return nil
}

// buildRejectionMessage 构建拒绝处理的错误消息
func (s *productService) buildRejectionMessage(validation *ValidationResult, suggestion *EnhancementSuggestion) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "数据质量不足（评分: %.2f/100），无法生成产品信息。\n\n", validation.QualityScore)

	if len(suggestion.RequiredActions) > 0 {
		sb.WriteString("必需改进操作：\n")
		for i, action := range suggestion.RequiredActions {
			fmt.Fprintf(&sb, "%d. %s\n", i+1, action)
		}
		sb.WriteString("\n")
	}

	if len(suggestion.OptionalActions) > 0 {
		sb.WriteString("可选改进操作：\n")
		for i, action := range suggestion.OptionalActions {
			fmt.Fprintf(&sb, "%d. %s\n", i+1, action)
		}
		sb.WriteString("\n")
	}

	if suggestion.EstimatedQuality != "" {
		fmt.Fprintf(&sb, "改进后预期质量：%s\n", suggestion.EstimatedQuality)
	}

	return sb.String()
}
