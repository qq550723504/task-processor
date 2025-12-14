package extractor

import (
	"fmt"
	"strings"
	"task-processor/common/amazon/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// TitleExtractor 标题提取器
type TitleExtractor struct{}

func (e *TitleExtractor) Extract(page playwright.Page, product *model.Product) error {
	// 尝试多个可能的标题选择器
	titleSelectors := []string{
		"#productTitle",
		"#title",
		"h1[id*='title']",
		"h1.product-title",
		"span#productTitle",
		"div#title_feature_div h1",
		"[data-feature-name='title'] h1",
	}

	for _, selector := range titleSelectors {
		title, err := page.TextContent(selector)
		if err == nil && strings.TrimSpace(title) != "" {
			product.Title = strings.TrimSpace(title)
			logrus.Debugf("成功提取标题: selector=%s, title=%s", selector, product.Title)
			return nil
		}
	}

	return fmt.Errorf("未找到产品标题")
}
