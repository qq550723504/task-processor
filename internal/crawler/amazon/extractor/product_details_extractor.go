package extractor

import (
	"strings"
	"task-processor/internal/model"

	"github.com/playwright-community/playwright-go"
)

// ProductDetailsExtractor 产品详情提取器
type ProductDetailsExtractor struct{}

func (e *ProductDetailsExtractor) Extract(page playwright.Page, product *model.Product) error {
	var details []model.ProductDetail

	// 产品详情表格选择器
	tableSelectors := []string{
		"#technicalSpecifications_section_1 tr",
		"table.a-keyvalue tr",
		"#productDetails_techSpec_section_1 tr",
		"#productDetails_detailBullets_sections1 tr",
	}

	for _, selector := range tableSelectors {
		rows, err := page.QuerySelectorAll(selector)
		if err != nil {
			continue
		}

		for _, row := range rows {
			cells, err := row.QuerySelectorAll("th, td")
			if err != nil || len(cells) < 2 {
				continue
			}

			keyText, err := cells[0].TextContent()
			if err != nil {
				continue
			}
			valueText, err := cells[1].TextContent()
			if err != nil {
				continue
			}

			key := strings.TrimSpace(keyText)
			value := strings.TrimSpace(valueText)

			// 清理value中的JavaScript代码和多余空白
			value = cleanProductDetailValue(value)

			if key != "" && value != "" {
				details = append(details, model.ProductDetail{
					Type:  key,
					Value: value,
				})
			}
		}

		if len(details) > 0 {
			break
		}
	}

	product.ProductDetails = details
	return nil
}

// cleanProductDetailValue 清理产品详情值，移除JavaScript代码和多余空白
func cleanProductDetailValue(value string) string {
	// 1. 移除JavaScript代码块（以var、function、P.when等开头的行）
	lines := strings.Split(value, "\n")
	var cleanedLines []string

	skipMode := false
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// 检测JavaScript代码的开始
		if strings.HasPrefix(trimmedLine, "var ") ||
			strings.HasPrefix(trimmedLine, "function") ||
			strings.HasPrefix(trimmedLine, "P.when(") ||
			strings.HasPrefix(trimmedLine, "A.declarative(") ||
			strings.Contains(trimmedLine, "window.ue") {
			skipMode = true
			continue
		}

		// 检测JavaScript代码块的结束
		if skipMode && (strings.Contains(trimmedLine, "});") || trimmedLine == "}") {
			skipMode = false
			continue
		}

		// 跳过JavaScript代码行
		if skipMode {
			continue
		}

		// 跳过空行
		if trimmedLine == "" {
			continue
		}

		// 保留有效内容
		cleanedLines = append(cleanedLines, trimmedLine)
	}

	// 2. 合并清理后的行
	result := strings.Join(cleanedLines, " ")

	// 3. 移除多余的空白字符
	result = strings.Join(strings.Fields(result), " ")

	// 4. 特殊处理：如果是评分信息，只保留评分数字
	if strings.Contains(result, "out of 5 stars") {
		// 提取评分数字（如 "4.7 out of 5 stars"）
		parts := strings.Split(result, "out of 5 stars")
		if len(parts) > 0 {
			// 查找第一个数字
			fields := strings.Fields(parts[0])
			for _, field := range fields {
				// 检查是否是评分格式（如 4.7）
				if strings.Contains(field, ".") && len(field) <= 4 {
					result = field + " out of 5 stars"
					break
				}
			}
		}
	}

	return result
}

