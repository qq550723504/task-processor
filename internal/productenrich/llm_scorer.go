// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// LLMScorer LLM 智能评分器接口
type LLMScorer interface {
	// ScoreText 对文本进行智能评分
	ScoreText(ctx context.Context, text string, baseScore float64) (float64, error)
	// ScoreImage 对图片进行智能评分
	ScoreImage(ctx context.Context, imageURL string, baseScore float64) (float64, error)
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
	if text == "" {
		return baseScore, nil
	}

	// 检查缓存
	if s.scoreCache != nil {
		if cachedScore, found := s.scoreCache.GetTextScore(ctx, text); found {
			finalScore := s.combineScores(baseScore, cachedScore)
			logrus.WithFields(logrus.Fields{
				"base_score":   baseScore,
				"cached_score": cachedScore,
				"final_score":  finalScore,
			}).Debug("using cached text score")
			return finalScore, nil
		}
	}

	// 调用 LLM 评分
	llmScore, err := s.scoreTextWithLLM(ctx, text, baseScore)
	if err != nil {
		logrus.WithError(err).Warn("LLM text scoring failed, using base score")
		return baseScore, err
	}

	// 缓存评分结果
	if s.scoreCache != nil {
		if err := s.scoreCache.SetTextScore(ctx, text, llmScore, s.cacheTTL); err != nil {
			logrus.WithError(err).Warn("failed to cache text score")
		}
	}

	// 综合评分
	finalScore := s.combineScores(baseScore, llmScore)

	logrus.WithFields(logrus.Fields{
		"base_score":  baseScore,
		"llm_score":   llmScore,
		"final_score": finalScore,
	}).Info("LLM text scoring completed")

	return finalScore, nil
}

// ScoreImage 对图片进行智能评分
func (s *llmScorer) ScoreImage(ctx context.Context, imageURL string, baseScore float64) (float64, error) {
	if imageURL == "" {
		return baseScore, nil
	}

	// 检查缓存
	if s.scoreCache != nil {
		if cachedScore, found := s.scoreCache.GetImageScore(ctx, imageURL); found {
			finalScore := s.combineScores(baseScore, cachedScore)
			logrus.WithFields(logrus.Fields{
				"base_score":   baseScore,
				"cached_score": cachedScore,
				"final_score":  finalScore,
			}).Debug("using cached image score")
			return finalScore, nil
		}
	}

	// 调用 LLM 评分
	llmScore, err := s.scoreImageWithLLM(ctx, imageURL, baseScore)
	if err != nil {
		logrus.WithError(err).Warn("LLM image scoring failed, using base score")
		return baseScore, err
	}

	// 缓存评分结果
	if s.scoreCache != nil {
		if err := s.scoreCache.SetImageScore(ctx, imageURL, llmScore, s.cacheTTL); err != nil {
			logrus.WithError(err).Warn("failed to cache image score")
		}
	}

	// 综合评分
	finalScore := s.combineScores(baseScore, llmScore)

	logrus.WithFields(logrus.Fields{
		"base_score":  baseScore,
		"llm_score":   llmScore,
		"final_score": finalScore,
	}).Info("LLM image scoring completed")

	return finalScore, nil
}

// scoreTextWithLLM 使用 LLM 对文本进行评分
func (s *llmScorer) scoreTextWithLLM(ctx context.Context, text string, baseScore float64) (float64, error) {
	if s.llmManager == nil {
		return baseScore, fmt.Errorf("LLM manager not configured")
	}

	client, err := s.llmManager.GetClient(s.textClient)
	if err != nil {
		return baseScore, fmt.Errorf("failed to get LLM client: %w", err)
	}

	prompt := s.buildTextScoringPrompt(text, baseScore)

	// 重试机制
	var response string
	var lastErr error
	for i := 0; i < s.maxRetries; i++ {
		response, lastErr = client.Generate(ctx, prompt)
		if lastErr == nil {
			break
		}
		logrus.WithError(lastErr).WithField("attempt", i+1).Warn("LLM scoring attempt failed")
		time.Sleep(time.Duration(i+1) * time.Second) // 指数退避
	}

	if lastErr != nil {
		return baseScore, fmt.Errorf("LLM scoring failed after %d attempts: %w", s.maxRetries, lastErr)
	}

	// 解析评分
	score, err := s.parseLLMScore(response)
	if err != nil {
		return baseScore, fmt.Errorf("failed to parse LLM score: %w", err)
	}

	return score, nil
}

// scoreImageWithLLM 使用 LLM 对图片进行评分
func (s *llmScorer) scoreImageWithLLM(ctx context.Context, imageURL string, baseScore float64) (float64, error) {
	if s.llmManager == nil {
		return baseScore, fmt.Errorf("LLM manager not configured")
	}

	client, err := s.llmManager.GetClient(s.visionClient)
	if err != nil {
		return baseScore, fmt.Errorf("failed to get vision client: %w", err)
	}

	prompt := s.buildImageScoringPrompt(baseScore)

	// 重试机制
	var response string
	var lastErr error
	for i := 0; i < s.maxRetries; i++ {
		response, lastErr = client.AnalyzeImage(ctx, imageURL, prompt)
		if lastErr == nil {
			break
		}
		logrus.WithError(lastErr).WithField("attempt", i+1).Warn("LLM image scoring attempt failed")
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if lastErr != nil {
		return baseScore, fmt.Errorf("LLM image scoring failed after %d attempts: %w", s.maxRetries, lastErr)
	}

	// 解析评分
	score, err := s.parseLLMScore(response)
	if err != nil {
		return baseScore, fmt.Errorf("failed to parse LLM score: %w", err)
	}

	return score, nil
}

// buildTextScoringPrompt 构建文本评分提示词（优化版）
func (s *llmScorer) buildTextScoringPrompt(text string, baseScore float64) string {
	return fmt.Sprintf(`你是一个专业的产品描述质量评估专家。请对以下产品描述文本进行质量评分（0-100分）。

评分维度：
1. 信息完整性（30分）：是否包含产品名称、类别、主要特性、规格参数等关键信息
2. 描述清晰度（25分）：表达是否清晰、逻辑是否连贯、是否易于理解
3. 专业性（25分）：是否使用准确的专业术语、是否符合行业标准
4. 吸引力（20分）：是否能吸引潜在买家、是否突出产品优势

产品描述文本：
%s

参考评分（基于文本长度）：%.1f 分

评分标准：
- 90-100分：优秀，信息完整、表达专业、极具吸引力
- 80-89分：良好，信息较完整、表达清晰、有一定吸引力
- 70-79分：中等，基本信息完整、表达尚可
- 60-69分：及格，信息不够完整或表达不够清晰
- 0-59分：不及格，信息严重缺失或表达混乱

请以 JSON 格式返回评分结果：
{
  "score": 85,
  "reason": "简要说明评分理由（50字以内）",
  "strengths": ["优点1", "优点2"],
  "weaknesses": ["不足1", "不足2"]
}

只返回 JSON，不要其他内容。`, text, baseScore)
}

// buildImageScoringPrompt 构建图片评分提示词（优化版）
func (s *llmScorer) buildImageScoringPrompt(baseScore float64) string {
	return fmt.Sprintf(`你是一个专业的产品图片质量评估专家。请对这张产品图片进行质量评分（0-100分）。

评分维度：
1. 清晰度（30分）：图片是否清晰、分辨率是否足够、是否有模糊或噪点
2. 专业性（25分）：拍摄角度、光线、背景是否专业、是否符合电商标准
3. 信息完整性（25分）：是否能清楚展示产品细节、是否有遮挡或缺失
4. 吸引力（20分）：构图是否美观、色彩是否协调、是否能吸引买家

参考评分（基于图片数量）：%.1f 分

评分标准：
- 90-100分：优秀，清晰专业、细节完整、极具吸引力
- 80-89分：良好，清晰度好、较专业、有吸引力
- 70-79分：中等，基本清晰、一般专业
- 60-69分：及格，清晰度或专业性不足
- 0-59分：不及格，模糊不清或严重不专业

请以 JSON 格式返回评分结果：
{
  "score": 85,
  "reason": "简要说明评分理由（50字以内）",
  "strengths": ["优点1", "优点2"],
  "weaknesses": ["不足1", "不足2"]
}

只返回 JSON，不要其他内容。`, baseScore)
}

// parseLLMScore 解析 LLM 返回的评分（增强版）
func (s *llmScorer) parseLLMScore(response string) (float64, error) {
	// 清理响应
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// 解析 JSON
	var result struct {
		Score      float64  `json:"score"`
		Reason     string   `json:"reason"`
		Strengths  []string `json:"strengths"`
		Weaknesses []string `json:"weaknesses"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return 0, fmt.Errorf("failed to parse JSON: %w, response: %s", err, response)
	}

	// 验证评分范围
	if result.Score < 0 || result.Score > 100 {
		return 0, fmt.Errorf("score out of range: %.2f", result.Score)
	}

	logrus.WithFields(logrus.Fields{
		"score":  result.Score,
		"reason": result.Reason,
	}).Debug("parsed LLM score")

	return result.Score, nil
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
