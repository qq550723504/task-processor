package handlers

import (
	"fmt"
	"strings"

	"task-processor/internal/common/pipeline"
)

// calculateQualityScore 计算质量评分
func (h *ProductDescriptionValidator) calculateQualityScore(description string, ctx *pipeline.TaskContext) int {
	score := 0

	// 长度评分 (0-25分)
	length := len(description)
	if length >= 500 && length <= 2000 {
		score += 25
	} else if length >= 200 && length <= 500 {
		score += 20
	} else if length >= 100 && length <= 200 {
		score += 15
	} else if length > 0 {
		score += 10
	}

	// 结构评分 (0-25分)
	if h.hasGoodStructure(description) {
		score += 25
	} else if strings.Contains(description, "\n") {
		score += 15
	} else {
		score += 5
	}

	// 内容丰富度评分 (0-25分)
	sentences := h.splitIntoSentences(description)
	if len(sentences) >= 5 {
		score += 25
	} else if len(sentences) >= 3 {
		score += 20
	} else if len(sentences) >= 2 {
		score += 15
	} else {
		score += 10
	}

	// 关键词覆盖评分 (0-25分)
	keywordScore := h.calculateKeywordScore(description, ctx)
	score += keywordScore

	// 确保评分在0-100范围内
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return score
}

// calculateKeywordScore 计算关键词评分
func (h *ProductDescriptionValidator) calculateKeywordScore(description string, ctx *pipeline.TaskContext) int {
	descLower := strings.ToLower(description)
	productName := strings.ToLower(ctx.TemuProduct.GoodsBasic.GoodsName)

	score := 0

	// 检查产品名称中的关键词
	nameWords := strings.Fields(productName)
	for _, word := range nameWords {
		if len(word) > 3 && strings.Contains(descLower, word) {
			score += 3
		}
	}

	// 检查通用质量词汇
	qualityWords := []string{
		"quality", "durable", "comfortable", "ergonomic", "premium",
		"professional", "reliable", "efficient", "easy", "convenient",
	}
	for _, word := range qualityWords {
		if strings.Contains(descLower, word) {
			score += 2
		}
	}

	// 限制最大评分
	if score > 25 {
		score = 25
	}

	return score
}

// generateDefaultDescription 生成默认描述
func (h *ProductDescriptionValidator) generateDefaultDescription(ctx *pipeline.TaskContext) string {
	productName := ctx.TemuProduct.GoodsBasic.GoodsName
	bulletPoints := ctx.TemuProduct.GoodsExtensionInfo.BulletPoints

	var desc strings.Builder

	// 基于产品名称生成通用开头
	desc.WriteString(fmt.Sprintf("Introducing the %s, ", productName))
	desc.WriteString("a high-quality product designed to meet your needs. ")
	desc.WriteString("Crafted with attention to detail and built to last.")

	// 添加要点作为特性说明
	if len(bulletPoints) > 0 {
		desc.WriteString("\n\nKey Features:\n")
		for _, point := range bulletPoints {
			desc.WriteString("• ")
			desc.WriteString(point)
			desc.WriteString("\n")
		}
	}

	// 添加通用结尾
	desc.WriteString("\nPerfect for both personal and professional use, ")
	desc.WriteString("this product offers excellent value and reliable performance.")

	return desc.String()
}
