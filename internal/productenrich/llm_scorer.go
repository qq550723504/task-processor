// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/jsonx"

	"github.com/sirupsen/logrus"
)

var llmScorePattern = regexp.MustCompile(`"score"\s*:\s*([0-9]+(?:\.[0-9]+)?)`)

// LLMScorer LLM 智能评分器接口
type LLMScorer interface {
	// ScoreText 对文本进行智能评分
	ScoreText(ctx context.Context, text string, baseScore float64) (float64, error)
	// ScoreImage 对图片进行智能评分
	ScoreImage(ctx context.Context, imageURL string, baseScore float64) (float64, error)
}

type llmScorerWithObservability interface {
	scoreTextResult(ctx context.Context, text string, baseScore float64) (*llmScoreResult, error)
	scoreImageResult(ctx context.Context, imageURL string, baseScore float64) (*llmScoreResult, error)
}

type llmScoreResult struct {
	Score  float64
	Prompt *PromptObservability
}

type rawLLMScoreResult struct {
	Score  float64
	Prompt *PromptObservability
}

// llmScorer LLM 智能评分器实现
type llmScorer struct {
	llmManager     LLMManager
	scoreCache     LLMScoreCache
	textClient     string
	visionClient   string
	cacheTTL       time.Duration
	maxRetries     int
	fallbackWeight float64 // LLM 评分权重（0-1），基础评分权重为 1-fallbackWeight
}

// LLMScorerConfig LLM 评分器配置
type LLMScorerConfig struct {
	LLMManager     LLMManager
	ScoreCache     LLMScoreCache
	TextClient     string        // 文本评分使用的 LLM 客户端
	VisionClient   string        // 图片评分使用的 LLM 客户端
	CacheTTL       time.Duration // 缓存过期时间
	MaxRetries     int           // 最大重试次数
	FallbackWeight float64       // LLM 评分权重（默认 0.3）
}

// NewLLMScorer 创建 LLM 智能评分器
func NewLLMScorer(config *LLMScorerConfig) LLMScorer {
	if config == nil {
		config = &LLMScorerConfig{
			TextClient:     "fast",
			VisionClient:   "vision",
			CacheTTL:       24 * time.Hour,
			MaxRetries:     2,
			FallbackWeight: 0.3,
		}
	}

	// 设置默认值
	if config.TextClient == "" {
		config.TextClient = "fast"
	}
	if config.VisionClient == "" {
		config.VisionClient = "vision"
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 24 * time.Hour
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 2
	}
	if config.FallbackWeight == 0 {
		config.FallbackWeight = 0.3
	}

	return &llmScorer{
		llmManager:     config.LLMManager,
		scoreCache:     config.ScoreCache,
		textClient:     config.TextClient,
		visionClient:   config.VisionClient,
		cacheTTL:       config.CacheTTL,
		maxRetries:     config.MaxRetries,
		fallbackWeight: config.FallbackWeight,
	}
}

// ScoreText 对文本进行智能评分
func (s *llmScorer) ScoreText(ctx context.Context, text string, baseScore float64) (float64, error) {
	result, err := s.scoreTextResult(ctx, text, baseScore)
	if err != nil {
		return result.Score, err
	}
	return result.Score, nil
}

func (s *llmScorer) scoreTextResult(ctx context.Context, text string, baseScore float64) (*llmScoreResult, error) {
	if text == "" {
		return &llmScoreResult{Score: baseScore}, nil
	}
	var getCached func() (*CachedLLMScore, bool)
	var setCached func(*CachedLLMScore) error
	if s.scoreCache != nil {
		getCached = func() (*CachedLLMScore, bool) { return s.scoreCache.GetTextScoreResult(ctx, text) }
		setCached = func(result *CachedLLMScore) error {
			return s.scoreCache.SetTextScoreResult(ctx, text, result, s.cacheTTL)
		}
	}
	return s.scoreWithCache(ctx, baseScore, getCached, setCached,
		func() (*rawLLMScoreResult, error) { return s.scoreTextWithLLM(ctx, text, baseScore) },
		"text",
	)
}

// ScoreImage 对图片进行智能评分
func (s *llmScorer) ScoreImage(ctx context.Context, imageURL string, baseScore float64) (float64, error) {
	result, err := s.scoreImageResult(ctx, imageURL, baseScore)
	if err != nil {
		return result.Score, err
	}
	return result.Score, nil
}

func (s *llmScorer) scoreImageResult(ctx context.Context, imageURL string, baseScore float64) (*llmScoreResult, error) {
	if imageURL == "" {
		return &llmScoreResult{Score: baseScore}, nil
	}
	var getCached func() (*CachedLLMScore, bool)
	var setCached func(*CachedLLMScore) error
	if s.scoreCache != nil {
		getCached = func() (*CachedLLMScore, bool) { return s.scoreCache.GetImageScoreResult(ctx, imageURL) }
		setCached = func(result *CachedLLMScore) error {
			return s.scoreCache.SetImageScoreResult(ctx, imageURL, result, s.cacheTTL)
		}
	}
	return s.scoreWithCache(ctx, baseScore, getCached, setCached,
		func() (*rawLLMScoreResult, error) { return s.scoreImageWithLLM(ctx, imageURL, baseScore) },
		"image",
	)
}

// scoreWithCache 通用的缓存+LLM评分流程
func (s *llmScorer) scoreWithCache(
	ctx context.Context,
	baseScore float64,
	getCached func() (*CachedLLMScore, bool),
	setCached func(*CachedLLMScore) error,
	callLLM func() (*rawLLMScoreResult, error),
	label string,
) (*llmScoreResult, error) {
	// 检查 context 是否已取消
	if err := ctx.Err(); err != nil {
		return &llmScoreResult{Score: baseScore}, err
	}

	// 检查缓存
	if s.scoreCache != nil {
		if cachedResult, found := getCached(); found && cachedResult != nil {
			finalScore := s.combineScores(baseScore, cachedResult.Score)
			logger.GetGlobalLogger("productenrich/llm_scorer.go").WithFields(logrus.Fields{
				"base_score":   baseScore,
				"cached_score": cachedResult.Score,
				"final_score":  finalScore,
				"has_prompt":   cachedResult.Prompt != nil,
			}).Debugf("using cached %s score", label)
			return &llmScoreResult{
				Score:  finalScore,
				Prompt: cachedResult.Prompt.Clone(),
			}, nil
		}
	}

	// 调用 LLM 评分
	llmResult, err := callLLM()
	if err != nil {
		logrus.WithError(err).Warnf("LLM %s scoring failed, using base score", label)
		return &llmScoreResult{Score: baseScore}, err
	}

	// 缓存评分结果
	if s.scoreCache != nil {
		if err := setCached(&CachedLLMScore{
			Score:  llmResult.Score,
			Prompt: llmResult.Prompt.Clone(),
		}); err != nil {
			logrus.WithError(err).Warnf("failed to cache %s score", label)
		}
	}

	finalScore := s.combineScores(baseScore, llmResult.Score)
	logger.GetGlobalLogger("productenrich/llm_scorer.go").WithFields(logrus.Fields{
		"base_score":  baseScore,
		"llm_score":   llmResult.Score,
		"final_score": finalScore,
	}).Infof("LLM %s scoring completed", label)

	return &llmScoreResult{
		Score:  finalScore,
		Prompt: llmResult.Prompt,
	}, nil
}

// scoreTextWithLLM 使用 LLM 对文本进行评分
func (s *llmScorer) scoreTextWithLLM(ctx context.Context, text string, baseScore float64) (*rawLLMScoreResult, error) {
	if s.llmManager == nil {
		return &rawLLMScoreResult{Score: baseScore}, fmt.Errorf("LLM manager not configured")
	}
	client, err := s.llmManager.GetClient(s.textClient)
	if err != nil {
		return &rawLLMScoreResult{Score: baseScore}, fmt.Errorf("failed to get LLM client: %w", err)
	}
	resolvedPrompt := resolveTextScoringPrompt(text, baseScore)
	response, err := s.retryLLMCall(ctx, s.maxRetries, func() (string, error) {
		return client.Generate(ctx, resolvedPrompt.Text)
	})
	if err != nil {
		return &rawLLMScoreResult{Score: baseScore}, fmt.Errorf("LLM scoring failed after %d attempts: %w", s.maxRetries, err)
	}
	score, err := s.parseLLMScore(response)
	if err != nil {
		return &rawLLMScoreResult{Score: baseScore}, fmt.Errorf("failed to parse LLM score: %w", err)
	}
	return &rawLLMScoreResult{
		Score: score,
		Prompt: &PromptObservability{
			PromptRef:     resolvedPrompt.Key,
			PromptKey:     resolvedPrompt.Key,
			PromptSource:  resolvedPrompt.Source,
			PromptVersion: resolvedPrompt.Version,
		},
	}, nil
}

// scoreImageWithLLM 使用 LLM 对图片进行评分
func (s *llmScorer) scoreImageWithLLM(ctx context.Context, imageURL string, baseScore float64) (*rawLLMScoreResult, error) {
	if s.llmManager == nil {
		return &rawLLMScoreResult{Score: baseScore}, fmt.Errorf("LLM manager not configured")
	}
	client, err := s.llmManager.GetClient(s.visionClient)
	if err != nil {
		return &rawLLMScoreResult{Score: baseScore}, fmt.Errorf("failed to get vision client: %w", err)
	}
	resolvedPrompt := resolveImageScoringPrompt(baseScore)
	response, err := s.retryLLMCall(ctx, s.maxRetries, func() (string, error) {
		return client.AnalyzeImage(ctx, imageURL, resolvedPrompt.Text)
	})
	if err != nil {
		return &rawLLMScoreResult{Score: baseScore}, fmt.Errorf("LLM image scoring failed after %d attempts: %w", s.maxRetries, err)
	}
	score, err := s.parseLLMScore(response)
	if err != nil {
		return &rawLLMScoreResult{Score: baseScore}, fmt.Errorf("failed to parse LLM score: %w", err)
	}
	return &rawLLMScoreResult{
		Score: score,
		Prompt: &PromptObservability{
			PromptRef:     resolvedPrompt.Key,
			PromptKey:     resolvedPrompt.Key,
			PromptSource:  resolvedPrompt.Source,
			PromptVersion: resolvedPrompt.Version,
		},
	}, nil
}

// retryLLMCall 通用重试机制，支持 context 取消。
func (s *llmScorer) retryLLMCall(ctx context.Context, maxRetries int, call func() (string, error)) (string, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		response, err := call()
		if err == nil {
			return response, nil
		}
		lastErr = err
		logrus.WithError(err).WithField("attempt", i+1).Warn("LLM scoring attempt failed")

		wait := time.Duration(i+1) * time.Second
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(wait):
		}
	}
	return "", lastErr
}

// buildTextScoringPrompt 构建文本评分提示词（优化版）
func (s *llmScorer) buildTextScoringPrompt(text string, baseScore float64) string {
	return resolveTextScoringPrompt(text, baseScore).Text
}

// buildImageScoringPrompt 构建图片评分提示词（优化版）
func (s *llmScorer) buildImageScoringPrompt(baseScore float64) string {
	return resolveImageScoringPrompt(baseScore).Text
}

// parseLLMScore 解析 LLM 返回的评分（增强版）
func (s *llmScorer) parseLLMScore(response string) (float64, error) {
	// 清理响应
	response = jsonx.CleanLLMResponse(response)

	// 解析 JSON
	var result struct {
		Score      float64  `json:"score"`
		Reason     string   `json:"reason"`
		Strengths  []string `json:"strengths"`
		Weaknesses []string `json:"weaknesses"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		score, extractErr := extractScoreFromPartialResponse(response)
		if extractErr != nil {
			return 0, fmt.Errorf("failed to parse JSON: %w, response: %s", err, response)
		}

		logger.GetGlobalLogger("productenrich/llm_scorer.go").WithFields(logrus.Fields{
			"score": score,
		}).Warn("parsed LLM score from partial response fallback")
		return score, nil
	}

	// 验证评分范围
	if result.Score < 0 || result.Score > 100 {
		return 0, fmt.Errorf("score out of range: %.2f", result.Score)
	}

	logger.GetGlobalLogger("productenrich/llm_scorer.go").WithFields(logrus.Fields{
		"score":  result.Score,
		"reason": result.Reason,
	}).Debug("parsed LLM score")

	return result.Score, nil
}

func extractScoreFromPartialResponse(response string) (float64, error) {
	matches := llmScorePattern.FindStringSubmatch(response)
	if len(matches) < 2 {
		return 0, fmt.Errorf("score not found in partial response")
	}

	score, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("parse partial score: %w", err)
	}
	if score < 0 || score > 100 {
		return 0, fmt.Errorf("score out of range: %.2f", score)
	}
	return score, nil
}

// combineScores 综合基础评分和 LLM 评分
func (s *llmScorer) combineScores(baseScore, llmScore float64) float64 {
	// 使用配置的权重
	baseWeight := 1.0 - s.fallbackWeight
	finalScore := (baseScore * baseWeight) + (llmScore * s.fallbackWeight)

	// 确保评分在 0-100 范围内
	if finalScore < 0 {
		finalScore = 0
	}
	if finalScore > 100 {
		finalScore = 100
	}

	return finalScore
}
