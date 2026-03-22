package extractor

import (
	"task-processor/internal/core/logger"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"task-processor/internal/model"
	"task-processor/internal/pkg/timeout"

	"github.com/playwright-community/playwright-go"
)

// RatingExtractor 评分提取器
type RatingExtractor struct{}

// Extract 提取评分和评论数据（并行优化版本）
func (e *RatingExtractor) Extract(page playwright.Page, product *model.Product) error {
	ctx, cancel := timeout.WithAIShortTimeout(context.Background())
	defer cancel()

	// 并行提取评分和评论数
	var wg sync.WaitGroup
	wg.Add(2)

	// 并行提取评分
	go func() {
		defer wg.Done()
		if err := e.extractRating(ctx, page, product); err != nil {
			logger.GetGlobalLogger("crawler/amazon").Warnf("提取评分失败: %v", err)
		}
	}()

	// 并行提取评论数
	go func() {
		defer wg.Done()
		if err := e.extractReviewsCount(ctx, page, product); err != nil {
			logger.GetGlobalLogger("crawler/amazon").Warnf("提取评论数失败: %v", err)
		}
	}()

	wg.Wait()

	logger.GetGlobalLogger("crawler/amazon").Infof("评分提取结果: Rating=%.1f, ReviewsCount=%d", product.Rating, product.ReviewsCount)
	return nil
}

// waitForPageLoad 等待页面关键元素加载
func (e *RatingExtractor) waitForPageLoad(ctx context.Context, page playwright.Page) error {
	// 只等待最快的选择器，减少超时时间
	selector := "#reviewsMedley"

	if err := page.Locator(selector).First().WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(3000), // 从5秒减少到3秒
	}); err == nil {
		return nil
	}

	// 如果主选择器失败，尝试备用选择器
	backupSelector := "#acrPopover"
	if err := page.Locator(backupSelector).First().WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(2000), // 2秒超时
	}); err == nil {
		return nil
	}

	return fmt.Errorf("页面评分元素加载超时")
}

// extractRating 提取产品评分
func (e *RatingExtractor) extractRating(_ context.Context, page playwright.Page, product *model.Product) error {
	// 2024年最新的Amazon评分选择器（按性能优化排序）
	ratingSelectors := []string{
		// 最快的选择器（0.0-0.1ms）
		"#reviewsMedley [data-hook='rating-out-of-text']",
		".a-popover-trigger .a-icon-alt",

		// 次快选择器（0.3-0.6ms）
		"#acrPopover .a-icon-alt",
		".a-icon-star .a-icon-alt",
		"[data-hook='average-star-rating'] .a-icon-alt",
		"span[data-hook='rating-out-of-text']",

		// 备用选择器（1.0ms+）
		"[data-hook='rating-out-of-text']",
		"#acrPopover [class*='a-icon-alt']",
	}

	for _, selector := range ratingSelectors {
		locator := page.Locator(selector).First()

		// 检查元素是否存在（不等待）
		count, err := locator.Count()
		if err != nil || count == 0 {
			continue
		}

		// 获取文本内容（设置短超时）
		text, err := locator.TextContent(playwright.LocatorTextContentOptions{
			Timeout: playwright.Float(500), // 500ms超时
		})
		if err != nil || text == "" {
			continue
		}

		// 解析评分
		if rating := e.parseRating(text); rating > 0 {
			product.Rating = rating
			logger.GetGlobalLogger("crawler/amazon").Infof("成功提取评分: %.1f", rating)
			return nil
		}
	}

	// 尝试从页面源码中提取评分
	if rating := e.extractRatingFromPageSource(page); rating > 0 {
		product.Rating = rating
		logger.GetGlobalLogger("crawler/amazon").Infof("从页面源码提取评分: %.1f", rating)
		return nil
	}

	return fmt.Errorf("未找到有效的评分数据")
}

// extractReviewsCount 提取评论数量
func (e *RatingExtractor) extractReviewsCount(_ context.Context, page playwright.Page, product *model.Product) error {
	// 2024年最新的Amazon评论数选择器（按性能优化排序）
	reviewSelectors := []string{
		// 最快的选择器（0.1-0.3ms）
		"#reviewsMedley [data-hook='total-review-count']",
		"span[data-hook='total-review-count']",

		// 次快选择器（0.7-0.9ms）
		"[data-hook='total-review-count']",
		"#acrCustomerReviewText",
	}

	for _, selector := range reviewSelectors {
		locator := page.Locator(selector).First()

		// 检查元素是否存在（不等待）
		count, err := locator.Count()
		if err != nil || count == 0 {
			continue
		}

		// 获取文本内容（设置短超时）
		text, err := locator.TextContent(playwright.LocatorTextContentOptions{
			Timeout: playwright.Float(500), // 500ms超时
		})
		if err != nil || text == "" {
			continue
		}

		// 解析评论数
		if reviewCount := e.parseReviewsCount(text); reviewCount > 0 {
			product.ReviewsCount = reviewCount
			logger.GetGlobalLogger("crawler/amazon").Infof("成功提取评论数: %d", reviewCount)
			return nil
		}
	}

	// 尝试从页面源码中提取评论数
	if reviewCount := e.extractReviewsCountFromPageSource(page); reviewCount > 0 {
		product.ReviewsCount = reviewCount
		logger.GetGlobalLogger("crawler/amazon").Infof("从页面源码提取评论数: %d", reviewCount)
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
