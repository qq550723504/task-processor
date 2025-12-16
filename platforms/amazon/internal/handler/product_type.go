// Package handler 提供Amazon产品类型推荐处理器
package handler

import (
	"context"
	"fmt"
	"strings"
	"task-processor/internal/amazon/service"
	"task-processor/platforms/amazon/internal/model"
)

// ProductTypeHandler 产品类型推荐处理器
type ProductTypeHandler struct {
	*BaseHandler
	marketplaceID string
}

// NewProductTypeHandler 创建产品类型推荐处理器
func NewProductTypeHandler() *ProductTypeHandler {
	return &ProductTypeHandler{
		BaseHandler:   NewBaseHandler("产品类型推荐器"),
		marketplaceID: "ATVPDKIKX0DER", // 默认美国市场
	}
}

// Execute 处理逻辑
func (h *ProductTypeHandler) Execute(services *model.Services, data map[string]any) error {
	h.logger.Info("开始推荐产品类型")

	// 验证服务
	if err := h.ValidateServices(services); err != nil {
		return err
	}

	// 获取产品类型缓存服务
	cache := services.GetProductTypeCache()
	if cache == nil {
		return fmt.Errorf("产品类型缓存服务未初始化")
	}

	// 获取原始产品数据
	rawData, exists := data["raw_product_data"]
	if !exists {
		return fmt.Errorf("原始产品数据不存在")
	}

	sourceData, ok := rawData.(map[string]any)
	if !ok {
		return fmt.Errorf("产品数据格式错误")
	}

	// 获取上下文
	ctxValue, exists := data["context"]
	if !exists {
		return fmt.Errorf("上下文不存在")
	}

	ctx, ok := ctxValue.(context.Context)
	if !ok {
		return fmt.Errorf("上下文类型错误")
	}

	// 类型断言获取ProductTypeCache
	productTypeCache, ok := cache.(*service.ProductTypeCache)
	if !ok {
		return fmt.Errorf("产品类型缓存类型错误")
	}

	// 1. 优先使用源数据中指定的产品类型
	if productType := h.getExplicitProductType(sourceData); productType != "" {
		if h.validateProductType(productTypeCache, ctx, productType) {
			h.logger.Infof("使用指定的产品类型: %s", productType)
			h.SetResult(data, "product_type", productType)
			return nil
		}
		h.logger.Warnf("指定的产品类型无效: %s，将自动推荐", productType)
	}

	// 2. 从产品信息推荐产品类型
	productType, err := h.recommendProductType(productTypeCache, ctx, sourceData)
	if err != nil {
		h.logger.Warnf("推荐失败，使用默认类型: %v", err)
		productType = "PRODUCT"
	}

	h.logger.Infof("推荐的产品类型: %s", productType)
	h.SetResult(data, "product_type", productType)

	return nil
}

// getExplicitProductType 获取显式指定的产品类型
func (h *ProductTypeHandler) getExplicitProductType(sourceData map[string]any) string {
	// 检查 product_type 字段
	if pt, ok := sourceData["product_type"].(string); ok && pt != "" {
		return strings.ToUpper(strings.TrimSpace(pt))
	}

	// 检查 productType 字段（驼峰命名）
	if pt, ok := sourceData["productType"].(string); ok && pt != "" {
		return strings.ToUpper(strings.TrimSpace(pt))
	}

	// 检查 amazon_product_type 字段
	if pt, ok := sourceData["amazon_product_type"].(string); ok && pt != "" {
		return strings.ToUpper(strings.TrimSpace(pt))
	}

	return ""
}

// validateProductType 验证产品类型是否有效
func (h *ProductTypeHandler) validateProductType(cache *service.ProductTypeCache, ctx context.Context, productType string) bool {
	return cache.IsValidProductType(ctx, h.marketplaceID, productType)
}

// recommendProductType 根据产品信息推荐产品类型
func (h *ProductTypeHandler) recommendProductType(cache *service.ProductTypeCache, ctx context.Context, sourceData map[string]any) (string, error) {
	// 提取关键词
	keywords := h.extractKeywords(sourceData)
	if len(keywords) == 0 {
		return "PRODUCT", nil
	}

	h.logger.Debugf("提取的关键词: %v", keywords)

	// 用关键词搜索匹配的产品类型
	var bestMatch *service.ProductTypeSummary
	var bestScore int

	for _, keyword := range keywords {
		results, err := cache.SearchByKeyword(ctx, h.marketplaceID, keyword, 10)
		if err != nil {
			continue
		}

		for _, pt := range results {
			score := h.calculateMatchScore(pt, keyword, sourceData)
			if score > bestScore {
				bestScore = score
				ptCopy := pt
				bestMatch = &ptCopy
			}
		}
	}

	if bestMatch != nil && bestScore >= 2 {
		h.logger.Infof("最佳匹配: %s (分数: %d)", bestMatch.ProductType, bestScore)
		return bestMatch.ProductType, nil
	}

	return "PRODUCT", nil
}

// extractKeywords 从产品数据中提取关键词
func (h *ProductTypeHandler) extractKeywords(sourceData map[string]any) []string {
	var keywords []string
	seen := make(map[string]bool)

	addKeyword := func(word string) {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) >= 3 && !seen[word] {
			seen[word] = true
			keywords = append(keywords, word)
		}
	}

	// 从分类提取
	if category, ok := sourceData["category"].(string); ok {
		for _, word := range strings.Fields(category) {
			addKeyword(word)
		}
	}

	// 从标题提取关键词
	if title, ok := sourceData["title"].(string); ok {
		words := h.extractSignificantWords(title)
		for _, word := range words[:min(5, len(words))] {
			addKeyword(word)
		}
	}

	// 从 subject 提取
	if subject, ok := sourceData["subject"].(string); ok {
		words := h.extractSignificantWords(subject)
		for _, word := range words[:min(3, len(words))] {
			addKeyword(word)
		}
	}

	// 限制关键词数量
	if len(keywords) > 8 {
		keywords = keywords[:8]
	}

	return keywords
}

// extractSignificantWords 提取有意义的词
func (h *ProductTypeHandler) extractSignificantWords(text string) []string {
	stopWords := map[string]bool{
		"the": true, "and": true, "or": true, "for": true, "with": true,
		"a": true, "an": true, "of": true, "to": true, "in": true, "on": true,
		"is": true, "are": true, "new": true, "hot": true, "sale": true,
	}

	var words []string
	for _, word := range strings.Fields(strings.ToLower(text)) {
		word = strings.Trim(word, ".,!?;:()[]{}\"'-")
		if len(word) >= 3 && !stopWords[word] {
			words = append(words, word)
		}
	}
	return words
}

// calculateMatchScore 计算匹配分数
func (h *ProductTypeHandler) calculateMatchScore(pt service.ProductTypeSummary, keyword string, sourceData map[string]any) int {
	score := 0
	ptLower := strings.ToLower(pt.ProductType)
	displayLower := strings.ToLower(pt.DisplayName)
	keywordLower := strings.ToLower(keyword)

	// 精确匹配产品类型名称
	if ptLower == keywordLower {
		score += 5
	} else if strings.Contains(ptLower, keywordLower) {
		score += 3
	}

	// 匹配显示名称
	if strings.Contains(displayLower, keywordLower) {
		score += 2
	}

	// 检查分类匹配
	if category, ok := sourceData["category"].(string); ok {
		categoryLower := strings.ToLower(category)
		if strings.Contains(displayLower, categoryLower) || strings.Contains(categoryLower, displayLower) {
			score += 2
		}
	}

	return score
}
