// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"fmt"
	"strings"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// QualityScorer 质量评分器接口
type QualityScorer interface {
	// CalculateScore 计算总体质量评分
	CalculateScore(ctx context.Context, validation *ValidationResult) (float64, error)
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

// CalculateScore 计算总体质量评分。
// 若启用了 LLMScorer，会用 LLM 对图片和文本分项重新评分后再加权，
// 否则直接使用 inputValidator 算出的基础分。
func (s *qualityScorer) CalculateScore(ctx context.Context, validation *ValidationResult) (float64, error) {
	if validation == nil {
		return 0, fmt.Errorf("validation result cannot be nil")
	}

	imageScore := validation.ImageScore
	textScore := validation.TextScore

	// 用 LLM 对各分项重新评分（仅在有原始验证对象时才能调用）
	if s.enableLLM && s.llmScorer != nil {
		if validation.ImageValidation != nil && len(validation.ImageValidation.ValidImages) > 0 {
			firstURL := validation.ImageValidation.ValidImages[0].URL
			if llmScore, err := s.llmScorer.ScoreImage(ctx, firstURL, imageScore); err != nil {
				logrus.WithError(err).Warn("LLM image scoring failed, using base score")
			} else {
				imageScore = llmScore
			}
		}
		if validation.TextValidation != nil && validation.TextValidation.Length > 0 {
			// 优先使用原始文本，降级到关键词拼接
			text := validation.TextValidation.RawText
			if text == "" {
				text = joinKeywords(validation.TextValidation.Keywords)
			}
			if text != "" {
				if llmScore, err := s.llmScorer.ScoreText(ctx, text, textScore); err != nil {
					logrus.WithError(err).Warn("LLM text scoring failed, using base score")
				} else {
					textScore = llmScore
				}
			}
		}
	}

	// 权重动态分配：无抓取数据时把 scrapedWeight 按比例分给 image/text
	imageWeight := s.imageWeight
	textWeight := s.textWeight
	scrapedWeight := s.scrapedWeight

	if validation.ScrapedScore == 0 && scrapedWeight > 0 {
		total := imageWeight + textWeight
		freed := scrapedWeight
		if total > 0 {
			imageWeight = imageWeight + freed*(imageWeight/total)
			textWeight = textWeight + freed*(textWeight/total)
		} else {
			imageWeight = 0.6
			textWeight = 0.4
		}
		scrapedWeight = 0
	}

	totalScore := imageScore*imageWeight + textScore*textWeight + validation.ScrapedScore*scrapedWeight
	if totalScore < 0 {
		totalScore = 0
	}
	if totalScore > 100 {
		totalScore = 100
	}

	validation.QualityScore = totalScore

	if s.metrics != nil {
		s.metrics.RecordCacheOperation("quality_score", "calculated")
	}

	logger.GetGlobalLogger("productenrich/scorer.go").WithFields(logrus.Fields{
		"total_score":    totalScore,
		"image_score":    imageScore,
		"text_score":     textScore,
		"scraped_score":  validation.ScrapedScore,
		"image_weight":   imageWeight,
		"text_weight":    textWeight,
		"scraped_weight": scrapedWeight,
		"llm_enabled":    s.enableLLM,
	}).Info("quality score calculated")

	return totalScore, nil
}

// joinKeywords 将关键词列表拼接为空格分隔的字符串，供 LLM 评分使用。
func joinKeywords(keywords []string) string {
	return strings.Join(keywords, " ")
}
