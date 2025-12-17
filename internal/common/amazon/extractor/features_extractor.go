package extractor

import (
	"fmt"
	"regexp"
	"strings"
	"task-processor/internal/common/amazon/model"

	"github.com/playwright-community/playwright-go"
)

// FeaturesExtractor 特性提取器
type FeaturesExtractor struct{}

func (e *FeaturesExtractor) Extract(page playwright.Page, product *model.Product) error {
	var features []string

	// 首先尝试从"About this item"部分提取特性
	aboutSelectors := []string{
		"#feature-bullets ul li span.a-list-item",
		"#feature-bullets ul li",
		"[data-feature-name='aboutThisItem'] ul li",
		"div[data-feature-name='featurebullets'] ul li",
		"#feature-bullets ul li span",
		"[data-feature-name='aboutThisItem'] ul li span",
		"div[data-feature-name='featurebullets'] ul li span",
	}

	for _, selector := range aboutSelectors {
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
			if e.isValidFeature(text) {
				features = append(features, text)
			}
		}

		if len(features) > 0 {
			break
		}
	}

	// 如果没有找到特性，尝试从产品详情表格中提取
	if len(features) == 0 {
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

				if key != "" && value != "" {
					feature := fmt.Sprintf("%s: %s", key, value)
					if e.isValidFeature(feature) {
						features = append(features, feature)
					}
				}
			}

			if len(features) > 0 {
				break
			}
		}
	}

	// 去重并赋值
	if len(features) > 0 {
		product.Features = e.deduplicateFeatures(features)
	}

	return nil
}

// isValidFeature 检查是否为有效的产品特性
func (e *FeaturesExtractor) isValidFeature(text string) bool {
	if text == "" || len(text) < 10 || len(text) > 500 {
		return false
	}

	// 过滤掉明显不是特性的文本
	invalidPatterns := []string{
		"see more", "show more", "read more", "click here",
		"customer reviews", "add to cart", "buy now",
		"\\$[0-9]+\\.[0-9]+",
		"out\\s+of\\s+5\\s+stars",
		"Reviewed\\s+in\\s+the\\s+United",
	}

	lowerText := strings.ToLower(text)
	for _, pattern := range invalidPatterns {
		matched, _ := regexp.MatchString("(?i)"+pattern, lowerText)
		if matched {
			return false
		}
	}

	return true
}

// deduplicateFeatures 去重特性列表
func (e *FeaturesExtractor) deduplicateFeatures(features []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, feature := range features {
		normalized := strings.ToLower(strings.TrimSpace(feature))
		if !seen[normalized] && normalized != "" {
			seen[normalized] = true
			result = append(result, feature)
		}
	}

	return result
}
