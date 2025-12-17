package extractor

import (
	"strconv"
	"strings"
	"task-processor/internal/common/amazon/model"

	"github.com/playwright-community/playwright-go"
)

// RatingExtractor 评分提取器
type RatingExtractor struct{}

func (e *RatingExtractor) Extract(page playwright.Page, product *model.Product) error {
	// 评分
	ratingSelectors := []string{
		"#acrPopover",
		".a-icon-star .a-icon-alt",
	}

	for _, selector := range ratingSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.TextContent()
			ratingStr := strings.Split(text, " ")[0]
			if rating, err := strconv.ParseFloat(ratingStr, 64); err == nil {
				product.Rating = rating
				break
			}
		}
	}

	// 评论数
	reviewSelectors := []string{
		"#acrCustomerReviewText",
		"span[data-hook='total-review-count']",
	}

	for _, selector := range reviewSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.TextContent()
			reviewStr := strings.ReplaceAll(text, ",", "")
			reviewStr = strings.Split(reviewStr, " ")[0]
			if count, err := strconv.Atoi(reviewStr); err == nil {
				product.ReviewsCount = count
				break
			}
		}
	}

	return nil
}
