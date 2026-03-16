// Package product 提供TEMU平台产品描述验证器的增强功能
package product

import (
	"strings"

	"task-processor/internal/pipeline"
)

// 这个文件包含ProductDescriptionValidator的增强功能
// 注意：主要方法定义在 product_description_validator.go 中
// 这里只包含辅助功能，避免方法重复定义

// enhanceDescriptionWithKeywords 使用关键词增强描述
func (h *ProductDescriptionValidator) enhanceDescriptionWithKeywords(description string, keywords []string) string {
	if len(keywords) == 0 {
		return description
	}

	// 检查描述中是否已包含关键词
	lowerDesc := strings.ToLower(description)
	missingKeywords := make([]string, 0)

	for _, keyword := range keywords {
		if !strings.Contains(lowerDesc, strings.ToLower(keyword)) {
			missingKeywords = append(missingKeywords, keyword)
		}
	}

	// 如果有缺失的关键词，添加到描述末尾
	if len(missingKeywords) > 0 {
		enhanced := description
		if !strings.HasSuffix(enhanced, ".") && !strings.HasSuffix(enhanced, "。") {
			enhanced += "."
		}
		enhanced += " Features: " + strings.Join(missingKeywords, ", ") + "."
		return enhanced
	}

	return description
}

// extractProductFeatures 从产品信息中提取特性
func (h *ProductDescriptionValidator) extractProductFeatures(ctx pipeline.TaskContext) []string {
	features := make([]string, 0)

	// 从Amazon产品中提取特性
	if amazonCtx, ok := ctx.(pipeline.AmazonContext); ok {
		amazonProduct := amazonCtx.GetAmazonProduct()
		if amazonProduct != nil {
			// 从标题中提取关键词
			if amazonProduct.Title != "" {
				titleWords := strings.Fields(amazonProduct.Title)
				for _, word := range titleWords {
					if len(word) > 3 && !isCommonWord(word) {
						features = append(features, word)
					}
				}
			}

			// 从特性列表中提取
			for _, feature := range amazonProduct.Features {
				if feature != "" && len(feature) < 50 {
					features = append(features, feature)
				}
			}
		}
	}

	return features
}

// isCommonWord 检查是否为常见词汇
func isCommonWord(word string) bool {
	commonWords := map[string]bool{
		"the": true, "and": true, "for": true, "with": true,
		"this": true, "that": true, "from": true, "your": true,
		"are": true, "you": true, "all": true, "can": true,
	}
	return commonWords[strings.ToLower(word)]
}
