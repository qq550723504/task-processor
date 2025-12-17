package handlers

import (
	"strings"

	"task-processor/internal/common/pipeline"
)

// enhanceDescription 增强描述内容
func (h *ProductDescriptionValidator) enhanceDescription(description string, ctx *pipeline.TaskContext, result *DescriptionValidationResult) string {
	// 如果描述过短，尝试扩展
	if len(description) < 200 {
		result.Suggestions = append(result.Suggestions, "描述过短，建议添加更多产品特性和用途说明")
		enhanced := h.expandDescription(description, ctx)
		if enhanced != description {
			result.Suggestions = append(result.Suggestions, "自动扩展了产品描述")
			return enhanced
		}
	}

	// 优化描述结构
	structured := h.structureDescription(description, result)

	return structured
}

// expandDescription 扩展描述
func (h *ProductDescriptionValidator) expandDescription(description string, ctx *pipeline.TaskContext) string {
	bulletPoints := ctx.TemuProduct.GoodsExtensionInfo.BulletPoints

	var expanded strings.Builder
	expanded.WriteString(description)

	// 如果有要点，基于要点扩展描述
	if len(bulletPoints) > 0 {
		expanded.WriteString("\n\nKey Features:\n")
		for _, point := range bulletPoints {
			expanded.WriteString("• ")
			expanded.WriteString(point)
			expanded.WriteString("\n")
		}
	}

	// 添加通用的产品说明（适用于所有品类）
	if len(description) < 100 && len(bulletPoints) == 0 {
		expanded.WriteString("\n\nThis product is designed with quality and functionality in mind. ")
		expanded.WriteString("Suitable for various applications and built to meet your needs.")
	}

	return expanded.String()
}

// structureDescription 结构化描述
func (h *ProductDescriptionValidator) structureDescription(description string, result *DescriptionValidationResult) string {
	// 检查是否已经有良好的结构
	if h.hasGoodStructure(description) {
		return description
	}

	// 尝试添加结构
	structured := h.addStructureToDescription(description)
	if structured != description {
		result.Suggestions = append(result.Suggestions, "优化了描述结构")
	}

	return structured
}

// hasGoodStructure 检查是否有良好的结构
func (h *ProductDescriptionValidator) hasGoodStructure(description string) bool {
	// 检查是否有段落分隔
	paragraphs := strings.Split(description, "\n\n")
	if len(paragraphs) >= 2 {
		return true
	}

	// 检查是否有列表结构
	if strings.Contains(description, "•") || strings.Contains(description, "-") {
		return true
	}

	return false
}

// addStructureToDescription 为描述添加结构
func (h *ProductDescriptionValidator) addStructureToDescription(description string) string {
	sentences := h.splitIntoSentences(description)
	if len(sentences) <= 2 {
		return description
	}

	var structured strings.Builder

	// 第一句作为开头
	if len(sentences) > 0 {
		structured.WriteString(sentences[0])
		structured.WriteString(".\n\n")
	}

	// 中间句子作为特性描述
	if len(sentences) > 2 {
		structured.WriteString("Features & Benefits:\n")
		for i := 1; i < len(sentences)-1 && i < 4; i++ {
			structured.WriteString("• ")
			structured.WriteString(sentences[i])
			structured.WriteString(".\n")
		}
		structured.WriteString("\n")
	}

	// 最后一句作为总结
	if len(sentences) > 1 {
		structured.WriteString(sentences[len(sentences)-1])
		structured.WriteString(".")
	}

	return structured.String()
}

// truncateDescription 截断描述
func (h *ProductDescriptionValidator) truncateDescription(description string, maxLength int) string {
	if len(description) <= maxLength {
		return description
	}

	// 在句子边界截断
	sentences := h.splitIntoSentences(description)
	var result strings.Builder

	for _, sentence := range sentences {
		if result.Len()+len(sentence)+1 <= maxLength-3 { // 为"..."预留空间
			if result.Len() > 0 {
				result.WriteString(" ")
			}
			result.WriteString(sentence)
			result.WriteString(".")
		} else {
			break
		}
	}

	if result.Len() < len(description) {
		result.WriteString("...")
	}

	return result.String()
}
