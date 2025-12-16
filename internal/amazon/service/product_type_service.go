// Package service 提供Amazon产品类型推荐服务
package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// ProductTypeService 产品类型服务
type ProductTypeService struct {
	apiClient *api.Client
	logger    *logrus.Entry
}

// NewProductTypeService 创建产品类型服务
func NewProductTypeService(apiClient *api.Client) *ProductTypeService {
	return &ProductTypeService{
		apiClient: apiClient,
		logger:    logrus.WithField("service", "ProductTypeService"),
	}
}

// ProductTypeRecommendation 产品类型推荐
type ProductTypeRecommendation struct {
	ProductType string  `json:"product_type"`
	DisplayName string  `json:"display_name"`
	Score       float64 `json:"score"`
	Reason      string  `json:"reason"`
}

// RecommendProductType 根据产品信息推荐产品类型
func (s *ProductTypeService) RecommendProductType(ctx context.Context, title, description string, keywords []string) ([]ProductTypeRecommendation, error) {
	s.logger.WithFields(logrus.Fields{
		"title":       title,
		"description": description[:min(50, len(description))] + "...",
		"keywords":    keywords,
	}).Info("开始产品类型推荐")

	// 构建搜索关键词
	searchTerms := s.buildSearchTerms(title, description, keywords)

	var allRecommendations []ProductTypeRecommendation

	// 对每个搜索词进行搜索
	for _, term := range searchTerms {
		recommendations, err := s.searchAndScore(ctx, term, title, description, keywords)
		if err != nil {
			s.logger.WithError(err).Warnf("搜索词 '%s' 失败", term)
			continue
		}
		allRecommendations = append(allRecommendations, recommendations...)
	}

	// 合并和排序推荐结果
	finalRecommendations := s.mergeAndRankRecommendations(allRecommendations)

	s.logger.WithField("count", len(finalRecommendations)).Info("产品类型推荐完成")
	return finalRecommendations, nil
}

// buildSearchTerms 构建搜索关键词
func (s *ProductTypeService) buildSearchTerms(title, description string, keywords []string) []string {
	var terms []string

	// 添加关键词
	terms = append(terms, keywords...)

	// 从标题中提取关键词
	titleWords := s.extractKeywords(title)
	terms = append(terms, titleWords...)

	// 从描述中提取关键词
	descWords := s.extractKeywords(description)
	terms = append(terms, descWords[:min(3, len(descWords))]...) // 只取前3个

	// 去重并过滤
	return s.deduplicateAndFilter(terms)
}

// extractKeywords 从文本中提取关键词
func (s *ProductTypeService) extractKeywords(text string) []string {
	// 停用词列表
	stopWords := map[string]bool{
		"the": true, "and": true, "or": true, "but": true, "in": true, "on": true,
		"at": true, "to": true, "for": true, "of": true, "with": true, "by": true,
		"a": true, "an": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "have": true, "has": true, "had": true, "do": true,
		"does": true, "did": true, "will": true, "would": true, "could": true, "should": true,
	}

	words := strings.Fields(strings.ToLower(text))
	var keywords []string

	for _, word := range words {
		// 清理标点符号
		word = strings.Trim(word, ".,!?;:()[]{}\"'")

		// 过滤停用词和短词
		if len(word) >= 3 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// deduplicateAndFilter 去重并过滤搜索词
func (s *ProductTypeService) deduplicateAndFilter(terms []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, term := range terms {
		term = strings.TrimSpace(strings.ToLower(term))
		if len(term) >= 3 && !seen[term] {
			seen[term] = true
			result = append(result, term)
		}
	}

	// 限制搜索词数量，避免过多API调用
	if len(result) > 5 {
		result = result[:5]
	}

	return result
}

// searchAndScore 搜索并评分产品类型
func (s *ProductTypeService) searchAndScore(ctx context.Context, searchTerm, title, description string, keywords []string) ([]ProductTypeRecommendation, error) {
	// 调用Amazon API搜索
	result, err := s.apiClient.SearchProductTypes(ctx, []string{searchTerm})
	if err != nil {
		return nil, fmt.Errorf("搜索产品类型失败: %w", err)
	}

	var recommendations []ProductTypeRecommendation

	for _, pt := range result {
		score := s.calculateScore(pt, searchTerm, title, description, keywords)
		reason := s.generateReason(pt, searchTerm, score)

		recommendations = append(recommendations, ProductTypeRecommendation{
			ProductType: pt.Name,
			DisplayName: pt.DisplayName,
			Score:       score,
			Reason:      reason,
		})
	}

	return recommendations, nil
}

// calculateScore 计算产品类型匹配分数
func (s *ProductTypeService) calculateScore(pt api.ProductTypeDefinition, searchTerm, title, description string, keywords []string) float64 {
	score := 0.0

	// 基础分数
	score += 1.0

	// 产品类型名称匹配
	if strings.Contains(strings.ToLower(pt.Name), strings.ToLower(searchTerm)) {
		score += 3.0
	}

	// 显示名称匹配
	if strings.Contains(strings.ToLower(pt.DisplayName), strings.ToLower(searchTerm)) {
		score += 2.0
	}

	// 关键词匹配
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(pt.DisplayName), strings.ToLower(keyword)) {
			score += 1.5
		}
		if strings.Contains(strings.ToLower(pt.Name), strings.ToLower(keyword)) {
			score += 1.0
		}
	}

	// 标题匹配
	titleWords := strings.Fields(strings.ToLower(title))
	for _, word := range titleWords {
		if len(word) >= 3 {
			if strings.Contains(strings.ToLower(pt.DisplayName), word) {
				score += 0.5
			}
		}
	}

	return score
}

// generateReason 生成推荐理由
func (s *ProductTypeService) generateReason(pt api.ProductTypeDefinition, searchTerm string, score float64) string {
	if score >= 5.0 {
		return fmt.Sprintf("高度匹配：产品类型与搜索词 '%s' 高度相关", searchTerm)
	} else if score >= 3.0 {
		return fmt.Sprintf("较好匹配：产品类型与搜索词 '%s' 相关", searchTerm)
	} else if score >= 2.0 {
		return fmt.Sprintf("一般匹配：产品类型可能适用于 '%s'", searchTerm)
	} else {
		return fmt.Sprintf("低匹配：基于搜索词 '%s' 的结果", searchTerm)
	}
}

// mergeAndRankRecommendations 合并并排序推荐结果
func (s *ProductTypeService) mergeAndRankRecommendations(recommendations []ProductTypeRecommendation) []ProductTypeRecommendation {
	// 按产品类型合并，取最高分数
	scoreMap := make(map[string]ProductTypeRecommendation)

	for _, rec := range recommendations {
		if existing, exists := scoreMap[rec.ProductType]; exists {
			if rec.Score > existing.Score {
				scoreMap[rec.ProductType] = rec
			}
		} else {
			scoreMap[rec.ProductType] = rec
		}
	}

	// 转换为切片并排序
	var result []ProductTypeRecommendation
	for _, rec := range scoreMap {
		result = append(result, rec)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	// 限制返回数量
	if len(result) > 10 {
		result = result[:10]
	}

	return result
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
