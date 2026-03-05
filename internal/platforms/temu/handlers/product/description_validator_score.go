// Package handlers 提供TEMU平台产品描述验证器的评分功能
package product

import (
	"fmt"
	"strings"

	"task-processor/internal/domain/model"
	"task-processor/internal/pipeline"
)

// 这个文件包含ProductDescriptionValidator的评分功能
// 注意：主要方法定义在 product_description_validator.go 中
// 这里只包含评分相关的辅助功能，避免方法重复定义

// calculateDetailedQualityScore 计算详细质量评分
func (h *ProductDescriptionValidator) calculateDetailedQualityScore(description string, ctx pipeline.TaskContext) int {
	score := 0

	// 长度评分 (0-25分)
	length := len(description)
	if length >= 50 && length <= 500 {
		score += 25
	} else if length >= 20 && length < 50 {
		score += 15
	} else if length > 500 && length <= 1000 {
		score += 20
	} else if length > 1000 {
		score += 10
	}

	// 内容丰富度评分 (0-25分)
	score += h.calculateContentRichness(description)

	// 格式规范性评分 (0-25分)
	score += h.calculateFormatScore(description)

	// 关键词相关性评分 (0-25分)
	score += h.calculateKeywordRelevance(description, ctx)

	// 确保分数在0-100范围内
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return score
}

// calculateContentRichness 计算内容丰富度
func (h *ProductDescriptionValidator) calculateContentRichness(description string) int {
	score := 0

	// 句子数量
	sentences := strings.Split(description, ".")
	sentenceCount := 0
	for _, sentence := range sentences {
		if strings.TrimSpace(sentence) != "" {
			sentenceCount++
		}
	}

	if sentenceCount >= 3 {
		score += 10
	} else if sentenceCount >= 2 {
		score += 7
	} else if sentenceCount >= 1 {
		score += 5
	}

	// 词汇多样性
	words := strings.Fields(description)
	uniqueWords := make(map[string]bool)
	for _, word := range words {
		uniqueWords[strings.ToLower(word)] = true
	}

	diversityRatio := float64(len(uniqueWords)) / float64(len(words))
	if diversityRatio > 0.8 {
		score += 15
	} else if diversityRatio > 0.6 {
		score += 10
	} else if diversityRatio > 0.4 {
		score += 5
	}

	return score
}

// calculateFormatScore 计算格式评分
func (h *ProductDescriptionValidator) calculateFormatScore(description string) int {
	score := 25 // 基础分数

	// 检查大小写混合
	hasUpper := strings.ToUpper(description) != description
	hasLower := strings.ToLower(description) != description
	if hasUpper && hasLower {
		score += 0 // 保持基础分数
	} else {
		score -= 10 // 全大写或全小写扣分
	}

	// 检查标点符号使用
	if strings.Contains(description, ".") || strings.Contains(description, "!") || strings.Contains(description, "?") {
		score += 0 // 保持基础分数
	} else {
		score -= 5 // 缺少标点符号扣分
	}

	// 检查过度使用标点符号
	exclamationCount := strings.Count(description, "!")
	if exclamationCount > 5 {
		score -= 10
	}

	return score
}

// calculateKeywordRelevance 计算关键词相关性
func (h *ProductDescriptionValidator) calculateKeywordRelevance(description string, ctx pipeline.TaskContext) int {
	score := 0

	// 获取产品信息
	var amazonProduct *model.Product
	if amazonCtx, ok := ctx.(pipeline.AmazonContext); ok {
		amazonProduct = amazonCtx.GetAmazonProduct()
	}

	if amazonProduct == nil {
		return 10 // 默认分数
	}

	lowerDesc := strings.ToLower(description)

	// 检查产品标题中的关键词
	if amazonProduct.Title != "" {
		titleWords := strings.Fields(strings.ToLower(amazonProduct.Title))
		matchCount := 0
		for _, word := range titleWords {
			if len(word) > 3 && strings.Contains(lowerDesc, word) {
				matchCount++
			}
		}
		if matchCount > 0 {
			score += min(15, matchCount*3)
		}
	}

	// 检查品牌名称
	if amazonProduct.Brand != "" && strings.Contains(lowerDesc, strings.ToLower(amazonProduct.Brand)) {
		score += 5
	}

	// 检查分类相关词汇（从其他字段推断）
	if amazonProduct.Title != "" {
		// 从标题中提取可能的分类词汇
		titleWords := strings.Fields(strings.ToLower(amazonProduct.Title))
		for _, word := range titleWords {
			if len(word) > 3 && strings.Contains(lowerDesc, word) {
				score += 2
				break
			}
		}
	}

	return score
}

// generateScoreReport 生成评分报告
func (h *ProductDescriptionValidator) generateScoreReport(description string, score int, ctx pipeline.TaskContext) string {
	report := fmt.Sprintf("描述质量评分: %d/100\n", score)

	if score >= 80 {
		report += "评级: 优秀 ✅\n"
	} else if score >= 60 {
		report += "评级: 良好 ⚠️\n"
	} else if score >= 40 {
		report += "评级: 一般 ⚠️\n"
	} else {
		report += "评级: 需要改进 ❌\n"
	}

	report += fmt.Sprintf("描述长度: %d 字符\n", len(description))

	words := strings.Fields(description)
	report += fmt.Sprintf("词汇数量: %d\n", len(words))

	return report
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
