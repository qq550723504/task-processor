package extractor

import (
	"regexp"
	"strconv"
	"strings"
	"task-processor/internal/model"

	"github.com/mxschmitt/playwright-go"
)

type BestsellerExtractor struct{}

func NewBestsellerExtractor() *BestsellerExtractor {
	return &BestsellerExtractor{}
}

func (e *BestsellerExtractor) Extract(page playwright.Page, product *model.Product) error {

	// 提取bestseller排名信息
	bsRank, bsCategory, rootBsRank, rootBsCategory := e.extractBestsellerRank(page)

	product.BsRank = bsRank
	product.BsCategory = bsCategory
	product.RootBsRank = rootBsRank
	product.RootBsCategory = rootBsCategory

	return nil
}

func (e *BestsellerExtractor) extractBestsellerRank(page playwright.Page) (int, string, int, string) {
	// 尝试多种选择器来查找bestseller排名信息
	selectors := []string{
		"#detailBullets_feature_div ul li span",
		"#productDetails_detailBullets_sections1 tr td",
		"#feature-bullets ul li span",
		".a-unordered-list .a-list-item",
		"#detailBulletsWrapper_feature_div ul li span",
	}

	var bsRank, rootBsRank int
	var bsCategory, rootBsCategory string

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
			if strings.Contains(strings.ToLower(text), "best sellers rank") {
				rank, category, rootRank, rootCategory := e.parseBestsellerText(text)
				if rank > 0 {
					bsRank = rank
					bsCategory = category
				}
				if rootRank > 0 {
					rootBsRank = rootRank
					rootBsCategory = rootCategory
				}
			}
		}
	}

	// 如果没有找到，尝试从页面文本中查找
	if bsRank == 0 && rootBsRank == 0 {
		pageText, err := page.TextContent("body")
		if err == nil {
			rank, category, rootRank, rootCategory := e.parseBestsellerText(pageText)
			if rank > 0 {
				bsRank = rank
				bsCategory = category
			}
			if rootRank > 0 {
				rootBsRank = rootRank
				rootBsCategory = rootCategory
			}
		}
	}

	return bsRank, bsCategory, rootBsRank, rootBsCategory
}
func (e *BestsellerExtractor) parseBestsellerText(text string) (int, string, int, string) {
	var bsRank, rootBsRank int
	var bsCategory, rootBsCategory string

	// 匹配 "Best Sellers Rank: #18,247 in Clothing, Shoes & Jewelry (See Top 100 in Clothing, Shoes & Jewelry) #4 in Men's Link Bracelets"
	// 或者 "Amazon Best Sellers Rank: #1,234 in Category"

	// 正则表达式匹配主要排名
	mainRankRegex := regexp.MustCompile(`(?i)best\s+sellers?\s+rank[:\s]*#?([\d,]+)\s+in\s+([^(#]+?)(?:\s*\(|$|#)`)
	matches := mainRankRegex.FindStringSubmatch(text)
	if len(matches) >= 3 {
		rankStr := strings.ReplaceAll(matches[1], ",", "")
		if rank, err := strconv.Atoi(rankStr); err == nil {
			rootBsRank = rank
			rootBsCategory = strings.TrimSpace(matches[2])
		}
	}

	// 正则表达式匹配子分类排名
	subRankRegex := regexp.MustCompile(`#(\d+)\s+in\s+([^#\n\r]+?)(?:\s*$|\s*\n|\s*\r|$)`)
	subMatches := subRankRegex.FindAllStringSubmatch(text, -1)

	for _, match := range subMatches {
		if len(match) >= 3 {
			rankStr := strings.ReplaceAll(match[1], ",", "")
			if rank, err := strconv.Atoi(rankStr); err == nil {
				category := strings.TrimSpace(match[2])
				// 如果这是第一个子分类排名，或者看起来更具体
				if bsRank == 0 || len(category) > len(bsCategory) {
					bsRank = rank
					bsCategory = category
				}
			}
		}
	}

	return bsRank, bsCategory, rootBsRank, rootBsCategory
}
