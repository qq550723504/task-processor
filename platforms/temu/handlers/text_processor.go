package handlers

import (
	"regexp"
	"strings"
	"task-processor/common/utils"
)

// TextProcessor 文本处理器
type TextProcessor struct{}

// NewTextProcessor 创建新的文本处理器
func NewTextProcessor() *TextProcessor {
	return &TextProcessor{}
}

// ProcessProductTitle 处理产品标题
func (tp *TextProcessor) ProcessProductTitle(title string) string {
	if title == "" {
		return ""
	}

	// 使用通用文本清理器移除中文字符和特殊符号
	title = utils.CleanProductTitle(title)

	// 清理标题，移除多余的空格和特殊字符
	title = strings.TrimSpace(title)
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")

	// 限制标题长度（TEMU通常限制在100字符以内）
	if len(title) > 100 {
		title = title[:97] + "..."
	}

	return title
}

// ProcessDescription 处理产品描述
func (tp *TextProcessor) ProcessDescription(description string) string {
	if description == "" {
		return "High quality product with excellent features."
	}

	// 移除HTML标签（如果有）
	description = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(description, "")

	// 使用通用文本清理器移除中文字符和特殊符号
	description = utils.CleanProductTitle(description)

	// 清理描述内容
	description = strings.TrimSpace(description)
	description = regexp.MustCompile(`\s+`).ReplaceAllString(description, " ")

	// 限制描述长度
	if len(description) > 2000 {
		description = description[:1997] + "..."
	}

	return description
}

// ProcessBulletPoints 处理要点描述
func (tp *TextProcessor) ProcessBulletPoints(features []string) []string {
	var bulletPoints []string

	for _, feature := range features {
		if feature = strings.TrimSpace(feature); feature != "" {
			// 使用通用文本清理器移除中文字符和特殊符号
			feature = utils.CleanProductTitle(feature)

			// 清理要点内容
			feature = regexp.MustCompile(`\s+`).ReplaceAllString(feature, " ")

			// 限制单个要点长度
			if len(feature) > 200 {
				feature = feature[:197] + "..."
			}

			bulletPoints = append(bulletPoints, feature)
		}

		// 限制要点数量
		if len(bulletPoints) >= 5 {
			break
		}
	}

	return bulletPoints
}

// GetDefaultBulletPoints 获取默认要点描述
func (tp *TextProcessor) GetDefaultBulletPoints() []string {
	return []string{
		"High quality materials and construction",
		"Excellent value for money",
		"Fast and reliable shipping",
		"Professional customer service",
		"Satisfaction guaranteed",
	}
}
