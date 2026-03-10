package extractor

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"task-processor/internal/domain/model"
	"task-processor/internal/pkg/contextutil"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// RatingExtractor 评分提取器
type RatingExtractor struct{}

// Extract 提取评分和评论数据
func (e *RatingExtractor) Extract(page playwright.Page, product *model.Product) error {
	ctx, cancel := contextutil.WithAIShortTimeout(context.Background())
	defer cancel()

	// 等待页面加载完成
	if err := e.waitForPageLoad(ctx, page); err != nil {
		logrus.Warnf("等待页面加载失败: %v", err)
	}

	// 提取评分
	if err := e.extractRating(ctx, page, product); err != nil {
		logrus.Warnf("提取评分失败: %v", err)
	}

	// 提取评论数
	if err := e.extractReviewsCount(ctx, page, product); err != nil {
		logrus.Warnf("提取评论数失败: %v", err)
	}

	logrus.Infof("评分提取结果: Rating=%.1f, ReviewsCount=%d", product.Rating, product.ReviewsCount)
	return nil
}

// waitForPageLoad 等待页面关键元素加载
func (e *RatingExtractor) waitForPageLoad(ctx context.Context, page playwright.Page) error {
	// 等待任意一个评分相关元素出现
	selectors := []string{
		"#acrPopover",
		"[data-hook='rating-out-of-text']",
		".a-icon-star",
		"#reviewsMedley",
		"[data-hook='total-review-count']",
	}

	for _, selector := range selectors {
		if err := page.Locator(selector).First().WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(5000),
		}); err == nil {
			return nil
		}
	}

	return fmt.Errorf("页面评分元素加载超时")
}

// extractRating 提取产品评分
func (e *RatingExtractor) extractRating(ctx context.Context, page playwright.Page, product *model.Product) error {
	// 2024年最新的Amazon评分选择器
	ratingSelectors := []string{
		// 主要评分显示区域
		"#acrPopover [class*='a-icon-alt']",
		"#acrPopover .a-icon-alt",
		"[data-hook='rating-out-of-text']",
		".a-icon-star .a-icon-alt",
		"[data-hook='average-star-rating'] .a-icon-alt",

		// 备用选择器
		".cr-original-review-link",
		"#reviewsMedley [data-hook='rating-out-of-text']",
		".a-popover-trigger .a-icon-alt",
		"span[data-hook='rating-out-of-text']",
	}

	for _, selector := range ratingSelectors {
		locator := page.Locator(selector).First()

		// 检查元素是否存在
		count, err := locator.Count()
		if err != nil || count == 0 {
			continue
		}

		// 获取文本内容
		text, err := locator.TextContent()
		if err != nil || text == "" {
			continue
		}

		// 解析评分
		if rating := e.parseRating(text); rating > 0 {
			product.Rating = rating
			logrus.Infof("成功提取评分: %.1f", rating)
			return nil
		}
	}

	// 尝试从页面源码中提取评分
	if rating := e.extractRatingFromPageSource(page); rating > 0 {
		product.Rating = rating
		logrus.Infof("从页面源码提取评分: %.1f", rating)
		return nil
	}

	return fmt.Errorf("未找到有效的评分数据")
}

// extractReviewsCount 提取评论数量
func (e *RatingExtractor) extractReviewsCount(ctx context.Context, page playwright.Page, product *model.Product) error {
	// 2024年最新的Amazon评论数选择器
	reviewSelectors := []string{
		// 主要评论数显示区域
		"#acrCustomerReviewText",
		"[data-hook='total-review-count']",
		"span[data-hook='total-review-count']",
		"#reviewsMedley [data-hook='total-review-count']",

		// 备用选择器
		".cr-original-review-link",
		"a[data-hook='see-all-reviews-link-foot']",
		"[data-hook='cr-filter-info-review-rating-count']",
		".a-size-base.a-link-normal",
	}

	for _, selector := range reviewSelectors {
		locator := page.Locator(selector).First()

		// 检查元素是否存在
		count, err := locator.Count()
		if err != nil || count == 0 {
			continue
		}

		// 获取文本内容
		text, err := locator.TextContent()
		if err != nil || text == "" {
			continue
		}

		// 解析评论数
		if reviewCount := e.parseReviewsCount(text); reviewCount > 0 {
			product.ReviewsCount = reviewCount
			logrus.Infof("成功提取评论数: %d", reviewCount)
			return nil
		}
	}

	// 尝试从页面源码中提取评论数
	if reviewCount := e.extractReviewsCountFromPageSource(page); reviewCount > 0 {
		product.ReviewsCount = reviewCount
		logrus.Infof("从页面源码提取评论数: %d", reviewCount)
		return nil
	}

	return fmt.Errorf("未找到有效的评论数据")
}

// parseRating 解析评分文本
func (e *RatingExtractor) parseRating(text string) float64 {
	// 清理文本
	text = strings.TrimSpace(text)

	// 匹配模式: "4.5 out of 5 stars", "4.5 星", "4.5", "4,5"
	patterns := []string{
		`(\d+[.,]\d+)\s*(?:out\s*of\s*5|星|stars?)`,
		`(\d+[.,]\d+)`,
		`(\d+)\s*(?:out\s*of\s*5|星|stars?)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			ratingStr := strings.ReplaceAll(matches[1], ",", ".")
			if rating, err := strconv.ParseFloat(ratingStr, 64); err == nil && rating >= 0 && rating <= 5 {
				return rating
			}
		}
	}

	return 0
}

// parseReviewsCount 解析评论数文本
func (e *RatingExtractor) parseReviewsCount(text string) int {
	// 清理文本，移除逗号和其他分隔符
	text = strings.ReplaceAll(text, ",", "")
	text = strings.ReplaceAll(text, ".", "")
	text = strings.ReplaceAll(text, " ", "")

	// 匹配数字模式
	patterns := []string{
		`(\d+)\s*(?:ratings?|reviews?|评价|评论)`,
		`(\d+)\s*(?:global\s*ratings?)`,
		`(\d+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			if count, err := strconv.Atoi(matches[1]); err == nil && count > 0 {
				return count
			}
		}
	}

	return 0
}

// extractRatingFromPageSource 从页面源码中提取评分
func (e *RatingExtractor) extractRatingFromPageSource(page playwright.Page) float64 {
	content, err := page.Content()
	if err != nil {
		return 0
	}

	// 在页面源码中搜索评分相关的JSON数据
	patterns := []string{
		`"averageStarRating":\s*(\d+\.?\d*)`,
		`"ratingValue":\s*"(\d+\.?\d*)"`,
		`averageRating['"]\s*:\s*['"]*(\d+\.?\d*)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			if rating, err := strconv.ParseFloat(matches[1], 64); err == nil && rating >= 0 && rating <= 5 {
				return rating
			}
		}
	}

	return 0
}

// extractReviewsCountFromPageSource 从页面源码中提取评论数
func (e *RatingExtractor) extractReviewsCountFromPageSource(page playwright.Page) int {
	content, err := page.Content()
	if err != nil {
		return 0
	}

	// 在页面源码中搜索评论数相关的JSON数据
	patterns := []string{
		`"reviewCount":\s*(\d+)`,
		`"totalReviewCount":\s*(\d+)`,
		`reviewCount['"]\s*:\s*['"]*(\d+)`,
		`(\d+)\s*global\s*ratings?`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			if count, err := strconv.Atoi(matches[1]); err == nil && count > 0 {
				return count
			}
		}
	}

	return 0
}
