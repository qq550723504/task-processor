package amazon

import (
	"regexp"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// ParentAsinExtractor 父级ASIN提取器
type ParentAsinExtractor struct{}

// NewParentAsinExtractor 创建父级ASIN提取器实例
func NewParentAsinExtractor() *ParentAsinExtractor {
	return &ParentAsinExtractor{}
}

// Extract 提取父级ASIN
func (pae *ParentAsinExtractor) Extract(page playwright.Page, product *Product) error {
	parentAsin, err := pae.getParentAsin(page)
	if err != nil {
		logrus.Infof("提取父级ASIN失败: %v", err)
		return err
	}
	product.ParentAsin = parentAsin
	return nil
}

// getParentAsin 获取父级ASIN
func (pae *ParentAsinExtractor) getParentAsin(page playwright.Page) (string, error) {
	// 尝试从各种位置提取父级ASIN
	parentAsin := pae.extractFromVariations(page)
	if parentAsin != "" {
		logrus.Infof("从变体信息中找到父级ASIN: %s", parentAsin)
		return parentAsin, nil
	}

	parentAsin = pae.extractFromPageSource(page)
	if parentAsin != "" {
		logrus.Infof("从页面源码中找到父级ASIN: %s", parentAsin)
		return parentAsin, nil
	}

	parentAsin = pae.extractFromMetadata(page)
	if parentAsin != "" {
		logrus.Infof("从元数据中找到父级ASIN: %s", parentAsin)
		return parentAsin, nil
	}

	logrus.Infof("未找到父级ASIN")
	return "", nil
}

// extractFromVariations 从变体信息中提取父级ASIN
func (pae *ParentAsinExtractor) extractFromVariations(page playwright.Page) string {
	// 变体相关的选择器
	selectors := []string{
		"[data-parent-asin]",
		"[data-asin-parent]",
		"#variation_parent_asin",
		"input[name='parentAsin']",
		"input[name='parent-asin']",
		".a-button-selected[data-asin]",
		"#twister [data-asin]",
	}

	for _, selector := range selectors {
		elements, err := page.QuerySelectorAll(selector)
		if err != nil {
			continue
		}

		for _, element := range elements {
			// 检查 data-parent-asin 属性
			if parentAsin, err := element.GetAttribute("data-parent-asin"); err == nil && parentAsin != "" {
				if pae.isValidAsin(parentAsin) {
					return parentAsin
				}
			}

			// 检查 value 属性
			if value, err := element.GetAttribute("value"); err == nil && value != "" {
				if pae.isValidAsin(value) {
					return value
				}
			}
		}
	}

	return ""
}

// extractFromPageSource 从页面源码中提取父级ASIN
func (pae *ParentAsinExtractor) extractFromPageSource(page playwright.Page) string {
	// 获取页面HTML内容
	content, err := page.Content()
	if err != nil {
		return ""
	}

	// 正则表达式匹配父级ASIN
	patterns := []string{
		`"parentAsin"\s*:\s*"([A-Z0-9]{10})"`,
		`"parent_asin"\s*:\s*"([A-Z0-9]{10})"`,
		`parentAsin\s*:\s*"([A-Z0-9]{10})"`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 && pae.isValidAsin(matches[1]) {
			return matches[1]
		}
	}

	return ""
}

// extractFromMetadata 从页面元数据中提取父级ASIN
func (pae *ParentAsinExtractor) extractFromMetadata(page playwright.Page) string {
	// 元数据选择器
	selectors := []string{
		`meta[name="parent-asin"]`,
		`meta[property="parent-asin"]`,
		`meta[name="parentAsin"]`,
		`meta[property="parentAsin"]`,
	}

	for _, selector := range selectors {
		element, err := page.QuerySelector(selector)
		if err != nil || element == nil {
			continue
		}

		content, err := element.GetAttribute("content")
		if err != nil || content == "" {
			continue
		}

		if pae.isValidAsin(content) {
			return content
		}
	}

	return ""
}

// isValidAsin 检查是否是有效的ASIN
func (pae *ParentAsinExtractor) isValidAsin(asin string) bool {
	if len(asin) != 10 {
		return false
	}

	// ASIN 通常是10位字母数字组合
	matched, _ := regexp.MatchString(`^[A-Z0-9]{10}$`, asin)
	return matched
}
