package extractor

import (
	"strings"
	"task-processor/internal/common/amazon/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// CategoriesExtractor 分类信息提取器
type CategoriesExtractor struct{}

// Extract 提取分类信息
func (ce *CategoriesExtractor) Extract(page playwright.Page, product *model.Product) error {
	categories, err := ce.getCategories(page)
	if err != nil {
		logrus.Infof("提取分类信息失败: %v", err)
		return err
	}
	product.Categories = categories
	return nil
}

// getCategories 获取产品分类
func (ce *CategoriesExtractor) getCategories(page playwright.Page) ([]string, error) {
	var categories []string

	// Amazon 分类信息的常见选择器
	selectors := []string{
		"#wayfinding-breadcrumbs_feature_div ul li a",
		"#wayfinding-breadcrumbs_feature_div ul li span",
		".a-unordered-list.a-horizontal.a-size-small li a",
		"[data-feature-name='breadcrumbs'] a",
		".breadcrumb a",
	}

	for _, selector := range selectors {
		elements, err := page.QuerySelectorAll(selector)
		if err != nil {
			continue
		}

		for _, element := range elements {
			text, err := element.TextContent()
			if err != nil {
				continue
			}

			text = strings.TrimSpace(text)
			if text != "" && ce.isValidCategory(text) {
				categories = append(categories, text)
			}
		}

		if len(categories) > 0 {
			break
		}
	}

	// 去重和清理
	categories = ce.cleanCategories(categories)

	return categories, nil
}

// isValidCategory 检查文本是否是有效的分类
func (ce *CategoriesExtractor) isValidCategory(text string) bool {
	text = strings.ToLower(text)

	// 排除无效的分类文本
	invalidTexts := []string{
		"", "›", ">", "&gt;", "home", "amazon", "amazon.com",
		"see all", "more", "less", "show", "hide", "view",
	}

	for _, invalid := range invalidTexts {
		if text == invalid {
			return false
		}
	}

	// 长度检查
	if len(text) < 2 || len(text) > 100 {
		return false
	}

	return true
}

// cleanCategories 清理和去重分类列表
func (ce *CategoriesExtractor) cleanCategories(categories []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, category := range categories {
		category = strings.TrimSpace(category)
		if category != "" && !seen[category] && ce.isValidCategory(category) {
			seen[category] = true
			result = append(result, category)
		}
	}

	// 限制分类数量
	if len(result) > 10 {
		result = result[:10]
	}

	return result
}
