package productenrich

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/logger"
)

func (s *productService) parseInput(ctx context.Context, task *Task) (*ParsedInput, error) {
	log := logger.GetGlobalLogger("productenrich/service_helpers.go").WithField("task_id", task.ID)
	log.Info("step 1: parsing input")

	if s.inputParser != nil {
		parsedInput, err := s.inputParser.ParseInput(ctx, task.Request)
		if err != nil {
			log.WithError(err).Error("failed to parse input")
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
		log.WithFields(logrus.Fields{
			"task_id":     task.ID,
			"images":      len(parsedInput.Images),
			"text_length": len(parsedInput.Text),
		}).Info("input parsed successfully")
		return parsedInput, nil
	}

	if !s.capabilities.AllowSimpleInputParsing {
		return nil, fmt.Errorf("input parser is not configured in %s mode", s.capabilities.Mode)
	}

	log.Warn("input parser not configured, using simple parsing")
	return &ParsedInput{
		Images: task.Request.ImageURLs,
		Text:   task.Request.Text,
	}, nil
}

func (s *productService) validateAndSelectStrategy(ctx context.Context, task *Task, parsedInput *ParsedInput) (ProcessingStrategy, *ValidationResult, error) {
	log := logger.GetGlobalLogger("productenrich/service_helpers.go").WithField("task_id", task.ID)

	if s.inputValidator == nil || s.qualityScorer == nil || s.strategySelector == nil {
		if !s.capabilities.AllowDefaultValidationStrategy {
			return "", nil, fmt.Errorf("validation pipeline is not fully configured in %s mode", s.capabilities.Mode)
		}
		log.Warn("validation components not configured, using full strategy")
		return StrategyFull, nil, nil
	}

	log.Info("step 2: validating input data")

	validationResult, err := s.inputValidator.Validate(ctx, parsedInput)
	if err != nil {
		log.WithError(err).Error("failed to validate input")
		if dbErr := s.taskRepo.MarkFailed(ctx, task.ID, fmt.Sprintf("input validation failed: %v", err)); dbErr != nil {
			log.WithError(dbErr).Error("failed to persist task error")
		}
		return "", nil, fmt.Errorf("failed to validate input: %w", err)
	}

	qualityScore, err := s.qualityScorer.CalculateScore(ctx, validationResult)
	if err != nil {
		log.WithError(err).Error("failed to calculate quality score")
		if dbErr := s.taskRepo.MarkFailed(ctx, task.ID, fmt.Sprintf("quality scoring failed: %v", err)); dbErr != nil {
			log.WithError(dbErr).Error("failed to persist task error")
		}
		return "", validationResult, fmt.Errorf("failed to calculate quality score: %w", err)
	}

	log.WithFields(logrus.Fields{
		"task_id":     task.ID,
		"score":       qualityScore,
		"image_score": validationResult.ImageScore,
		"text_score":  validationResult.TextScore,
	}).Info("quality score calculated")

	strategy, err := s.strategySelector.SelectStrategy(ctx, qualityScore)
	if err != nil {
		log.WithError(err).Error("failed to select strategy")
		if dbErr := s.taskRepo.MarkFailed(ctx, task.ID, fmt.Sprintf("strategy selection failed: %v", err)); dbErr != nil {
			log.WithError(dbErr).Error("failed to persist task error")
		}
		return "", validationResult, fmt.Errorf("failed to select strategy: %w", err)
	}

	log.WithFields(logrus.Fields{
		"task_id":  task.ID,
		"strategy": string(strategy),
	}).Info("processing strategy selected")

	if strategy == StrategyReject {
		rejectedStrategy, rejectErr := s.handleRejection(ctx, task, validationResult)
		return rejectedStrategy, validationResult, rejectErr
	}

	return strategy, validationResult, nil
}

func (s *productService) handleRejection(ctx context.Context, task *Task, validationResult *ValidationResult) (ProcessingStrategy, error) {
	var errorMsg string
	if s.enhancementSuggester != nil {
		suggestion, err := s.enhancementSuggester.GenerateSuggestions(ctx, validationResult)
		if err != nil {
			logger.GetGlobalLogger("productenrich/service_helpers.go").WithField("task_id", task.ID).WithError(err).Error("failed to generate suggestions")
		} else {
			errorMsg = s.buildRejectionMessage(validationResult, suggestion)
		}
	}
	if errorMsg == "" {
		errorMsg = fmt.Sprintf("数据质量不足（评分: %.2f），无法生成产品信息", validationResult.QualityScore)
	}
	if dbErr := s.taskRepo.MarkFailed(ctx, task.ID, errorMsg); dbErr != nil {
		logger.GetGlobalLogger("productenrich/service_helpers.go").WithField("task_id", task.ID).WithError(dbErr).Error("failed to persist rejection error")
	}
	return "", &errNoRetry{cause: fmt.Errorf("%s", errorMsg)}
}

func (s *productService) analyzeProduct(ctx context.Context, task *Task, parsedInput *ParsedInput) (*ProductAnalysis, error) {
	log := logger.GetGlobalLogger("productenrich/service_helpers.go").WithField("task_id", task.ID)
	log.Info("step 3: analyzing product")

	if s.productUnderstanding != nil {
		analysis, err := s.productUnderstanding.AnalyzeProduct(ctx, parsedInput)
		if err != nil {
			log.WithError(err).Error("failed to analyze product")
			return nil, fmt.Errorf("failed to analyze product: %w", err)
		}
		log.Info("product analyzed successfully")
		return analysis, nil
	}

	if !s.capabilities.AllowSimpleAnalysis {
		return nil, fmt.Errorf("product understanding is not configured in %s mode", s.capabilities.Mode)
	}

	log.Warn("product understanding not configured, using simple analysis")
	return &ProductAnalysis{
		Representation: &ProductRepresentation{
			ProductType: "Unknown Product",
			Attributes:  make(map[string]string),
			Features:    []string{},
		},
	}, nil
}

func (s *productService) generateProductJSON(ctx context.Context, task *Task, analysis *ProductAnalysis, strategy ProcessingStrategy) (*ProductJSON, error) {
	log := logger.GetGlobalLogger("productenrich/service_helpers.go").WithField("task_id", task.ID)
	log.Info("step 4: generating product JSON")

	variantGen := s.variantGenerator
	skipVariants := false
	switch strategy {
	case StrategyBasic:
		skipVariants = true
	case StrategyMinimal:
		variantGen = nil
	}

	if s.jsonGenerator != nil {
		productJSON, err := s.jsonGenerator.GenerateJSON(ctx, analysis, variantGen, skipVariants)
		if err != nil {
			log.WithError(err).Error("failed to generate JSON")
			return nil, fmt.Errorf("failed to generate JSON: %w", err)
		}
		log.Info("product JSON generated successfully")
		return productJSON, nil
	}

	if !s.capabilities.AllowSimpleGeneration {
		return nil, fmt.Errorf("JSON generator is not configured in %s mode", s.capabilities.Mode)
	}

	log.Warn("JSON generator not configured, using simple generation")
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

func (s *productService) validateResult(ctx context.Context, task *Task, parsedInput *ParsedInput, productJSON *ProductJSON) error {
	log := logger.GetGlobalLogger("productenrich/service_helpers.go").WithField("task_id", task.ID)

	if s.resultValidator == nil {
		if s.capabilities.AllowMissingResultValidator {
			return nil
		}
		return fmt.Errorf("result validator is not configured in %s mode", s.capabilities.Mode)
	}

	log.Info("step 5: validating result")
	resultValidation, err := s.resultValidator.ValidateResult(ctx, parsedInput, productJSON)
	if err != nil {
		log.WithError(err).Error("failed to validate result")
		return fmt.Errorf("result validation error: %w", err)
	}

	log.WithFields(logrus.Fields{
		"task_id":      task.ID,
		"is_valid":     resultValidation.IsValid,
		"issues_count": len(resultValidation.Issues),
	}).Info("result validation completed")

	for _, issue := range resultValidation.Issues {
		log.WithFields(logrus.Fields{
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
