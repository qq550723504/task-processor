// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// QualityScorer 质量评分器接口
type QualityScorer interface {
	// CalculateScore 计算总体质量评分
	CalculateScore(ctx context.Context, validation *ValidationResult) (float64, error)
	// ScoreImages 评估图片质量
	ScoreImages(ctx context.Context, imageValidation *ImageValidation) (float64, error)
	// ScoreText 评估文本质量
	ScoreText(ctx context.Context, textValidation *TextValidation) (float64, error)
	// ScoreScrapedData 评估抓取数据质量
	ScoreScrapedData(ctx context.Context, scrapedValidation *ScrapedDataValidation) (float64, error)
}

// qualityScorer 质量评分器实现
type qualityScorer struct {
	imageWeight   float64
	textWeight    float64
	scrapedWeight float64
	llmScorer     LLMScorer
	enableLLM     bool
	metrics       MetricsCollector
}

// QualityScorerConfig 质量评分器配置
type QualityScorerConfig struct {
	ImageWeight   float64          // 图片权重
	TextWeight    float64          // 文本权重
	ScrapedWeight float64          // 抓取数据权重
	LLMScorer     LLMScorer        // LLM 评分器（可选）
	EnableLLM     bool             // 是否启用 LLM 评分
	Metrics       MetricsCollector // 指标收集器
}

// NewQualityScorer 创建新的质量评分器
func NewQualityScorer(config *QualityScorerConfig) QualityScorer {
	if config == nil {
		config = &QualityScorerConfig{
			ImageWeight:   0.4,
			TextWeight:    0.3,
			ScrapedWeight: 0.3,
		}
	}

	// 验证权重总和
	totalWeight := config.ImageWeight + config.TextWeight + config.ScrapedWeight
	if totalWeight == 0 {
		// 使用默认权重
		config.ImageWeight = 0.4
		config.TextWeight = 0.3
		config.ScrapedWeight = 0.3
	}

	return &qualityScorer{
		imageWeight:   config.ImageWeight,
		textWeight:    config.TextWeight,
		scrapedWeight: config.ScrapedWeight,
		llmScorer:     config.LLMScorer,
		enableLLM:     config.EnableLLM && config.LLMScorer != nil,
		metrics:       config.Metrics,
	}
}

// CalculateScore 计算总体质量评分
func (s *qualityScorer) CalculateScore(ctx context.Context, validation *ValidationResult) (float64, error) {
	if validation == nil {
		return 0, fmt.Errorf("validation result cannot be nil")
	}

	// 使用配置的权重
	imageWeight := s.imageWeight
	textWeight := s.textWeight
	scrapedWeight := s.scrapedWeight

	// 如果没有抓取数据，重新分配权重
	if validation.ScrapedScore == 0 && scrapedWeight > 0 {
		// 将抓取数据的权重按比例分配给图片和文本
		totalWeight := imageWeight + textWeight
		if totalWeight > 0 {
			imageWeight = (imageWeight / totalWeight) * (imageWeight + textWeight + scrapedWeight)
			textWeight = (textWeight / totalWeight) * (s.imageWeight + s.textWeight + scrapedWeight)
		} else {
			// 如果图片和文本权重都为0，使用默认分配
			imageWeight = 0.6
			textWeight = 0.4
		}
		scrapedWeight = 0
	}

	// 计算加权总分
	totalScore := validation.ImageScore*imageWeight +
		validation.TextScore*textWeight +
		validation.ScrapedScore*scrapedWeight

	// 确保评分在0-100 范围内
	if totalScore < 0 {
		totalScore = 0
	}
	if totalScore > 100 {
		totalScore = 100
	}

	validation.QualityScore = totalScore

	// 记录质量评分指标
	if s.metrics != nil {
		s.metrics.RecordCacheOperation("quality_score", "calculated")
	}

	logrus.WithFields(logrus.Fields{
		"total_score":    totalScore,
		"image_score":    validation.ImageScore,
		"text_score":     validation.TextScore,
		"scraped_score":  validation.ScrapedScore,
		"image_weight":   imageWeight,
		"text_weight":    textWeight,
		"scraped_weight": scrapedWeight,
	}).Info("quality score calculated")

	return totalScore, nil
}

// ScoreImages 评估图片质量
func (s *qualityScorer) ScoreImages(ctx context.Context, imageValidation *ImageValidation) (float64, error) {
	if imageValidation == nil {
		return 0, fmt.Errorf("image validation cannot be nil")
	}

	// 基础评分：根据有效图片数量
	baseScore := s.calculateImageBaseScore(imageValidation.ValidCount)

	// 质量调整：根据有效率调整评分
	if imageValidation.TotalCount > 0 {
		validRate := float64(imageValidation.ValidCount) / float64(imageValidation.TotalCount)
		// 如果有效率低于80%，降低评分
		if validRate < 0.8 {
			baseScore = baseScore * validRate
		}
	}

	// 如果启用 LLM 且有有效图片，使用 LLM 进行智能评分
	finalScore := baseScore
	if s.enableLLM && s.llmScorer != nil && len(imageValidation.ValidImages) > 0 {
		// 使用第一张图片进行评分
		firstImageURL := imageValidation.ValidImages[0].URL
		llmScore, err := s.llmScorer.ScoreImage(ctx, firstImageURL, baseScore)
		if err != nil {
			logrus.WithError(err).Warn("LLM image scoring failed, using base score")
			finalScore = baseScore
		} else {
			finalScore = llmScore
		}
	}

	logrus.WithFields(logrus.Fields{
		"total_count": imageValidation.TotalCount,
		"valid_count": imageValidation.ValidCount,
		"base_score":  baseScore,
		"final_score": finalScore,
		"llm_enabled": s.enableLLM,
	}).Info("image quality scored")

	return finalScore, nil
}

// calculateImageBaseScore 根据图片数量计算基础评分
func (s *qualityScorer) calculateImageBaseScore(validCount int) float64 {
	if validCount == 0 {
		return 0
	}
	if validCount <= 2 {
		return 60
	}
	if validCount <= 4 {
		return 80
	}
	// 5-8 张图片
	if validCount <= 8 {
		return 100
	}
	// 超过 8 张，仍然是 100 分
	return 100
}

// ScoreText 评估文本质量
func (s *qualityScorer) ScoreText(ctx context.Context, textValidation *TextValidation) (float64, error) {
	if textValidation == nil {
		return 0, fmt.Errorf("text validation cannot be nil")
	}

	// 基础评分：根据文本长度
	baseScore := s.calculateTextBaseScore(textValidation.Length)

	// 质量调整：如果有关键词，略微提升评分
	if textValidation.HasKeywords && len(textValidation.Keywords) > 0 {
		keywordBonus := float64(len(textValidation.Keywords)) * 0.5
		if keywordBonus > 5 {
			keywordBonus = 5 // 最多加 5 分
		}
		baseScore += keywordBonus
	}

	// 确保评分不超过100
	if baseScore > 100 {
		baseScore = 100
	}

	// 如果启用 LLM 且文本长度足够，使用 LLM 进行智能评分
	finalScore := baseScore
	if s.enableLLM && s.llmScorer != nil && textValidation.Length >= 20 {
		// 重构文本（从关键词重建，如果原始文本不可用）
		text := strings.Join(textValidation.Keywords, " ")
		llmScore, err := s.llmScorer.ScoreText(ctx, text, baseScore)
		if err != nil {
			logrus.WithError(err).Warn("LLM text scoring failed, using base score")
			finalScore = baseScore
		} else {
			finalScore = llmScore
		}
	}

	logrus.WithFields(logrus.Fields{
		"length":        textValidation.Length,
		"keyword_count": len(textValidation.Keywords),
		"base_score":    baseScore,
		"final_score":   finalScore,
		"llm_enabled":   s.enableLLM,
	}).Info("text quality scored")

	return finalScore, nil
}

// calculateTextBaseScore 根据文本长度计算基础评分
func (s *qualityScorer) calculateTextBaseScore(length int) float64 {
	if length == 0 {
		return 0
	}
	if length < 50 {
		return 30
	}
	if length < 100 {
		return 60
	}
	if length < 200 {
		return 80
	}
	// 200 字符以上
	return 100
}

// ScoreScrapedData 评估抓取数据质量
func (s *qualityScorer) ScoreScrapedData(ctx context.Context, scrapedValidation *ScrapedDataValidation) (float64, error) {
	if scrapedValidation == nil {
		return 0, fmt.Errorf("scraped data validation cannot be nil")
	}

	score := 0.0
	fieldCount := 0
	presentCount := 0

	// 评估各个字段（每个字段20 分）
	fields := []struct {
		name    string
		present bool
		weight  float64
	}{
		{"title", scrapedValidation.HasTitle, 20},
		{"description", scrapedValidation.HasDescription, 20},
		{"images", scrapedValidation.HasImages, 20},
		{"specs", scrapedValidation.HasSpecs, 20},
		{"price", scrapedValidation.HasPrice, 20},
	}

	for _, field := range fields {
		fieldCount++
		if field.present {
			score += field.weight
			presentCount++
		}
	}

	// 如果有图片，根据图片数量给予额外加分
	if scrapedValidation.HasImages && scrapedValidation.ImageCount > 0 {
		imageBonus := 0.0
		if scrapedValidation.ImageCount >= 3 {
			imageBonus = 5 // 3 张以上图片加 5 分
		} else if scrapedValidation.ImageCount >= 2 {
			imageBonus = 3 // 2 张图片加 3 分
		} else {
			imageBonus = 1 // 1 张图片加 1 分
		}
		score += imageBonus
	}

	// 确保评分不超过100
	if score > 100 {
		score = 100
	}

	logrus.WithFields(logrus.Fields{
		"field_count":   fieldCount,
		"present_count": presentCount,
		"image_count":   scrapedValidation.ImageCount,
		"score":         score,
	}).Info("scraped data quality scored")

	return score, nil
}
