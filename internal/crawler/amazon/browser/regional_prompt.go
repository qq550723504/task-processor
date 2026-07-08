package browser

import (
	"net/url"
	"strings"
	"time"

	"task-processor/internal/core/logger"

	"github.com/mxschmitt/playwright-go"
)

func regionalPromptTextSelectors() []string {
	return []string{
		"text=Visiting from Singapore",
		"text=Visiting from singapore",
		"text=Choosing your Amazon website",
		"text=Visit Amazon.sg",
		"text=Stay on Amazon.sg",
		"text=Go to Amazon.com",
		"text=Continue shopping on Amazon.com",
		"text=Continue shopping on Amazon",
	}
}

func regionalPromptActionSelectors(targetURL string) []string {
	host := normalizedAmazonHost(targetURL)
	selectors := []string{
		"a[href*='mr_donotredirect']",
		"button:has-text('Go to Amazon.com')",
		"a:has-text('Go to Amazon.com')",
		"button:has-text('Continue shopping on Amazon.com')",
		"a:has-text('Continue shopping on Amazon.com')",
	}
	if host != "" {
		hostWithoutWWW := strings.TrimPrefix(host, "www.")
		selectors = append(selectors,
			"a[href*='"+host+"']",
			"a[href*='"+hostWithoutWWW+"']",
			"button:has-text('Go to "+host+"')",
			"a:has-text('Go to "+host+"')",
			"button:has-text('Go to "+hostWithoutWWW+"')",
			"a:has-text('Go to "+hostWithoutWWW+"')",
		)
	}
	selectors = append(selectors,
		"input[value*='Continue shopping on Amazon.com']",
		"button:has-text('Continue shopping on Amazon')",
		"a:has-text('Continue shopping on Amazon')",
		"button:has-text('Continue shopping')",
		"a:has-text('Continue shopping')",
		"button[data-action='a-popover-close']",
		"button[aria-label='Close']",
		".a-popover-header button",
		".a-button-close",
	)
	return selectors
}

func looksLikeRegionalPrompt(page playwright.Page) bool {
	for _, selector := range regionalPromptTextSelectors() {
		locator := page.Locator(selector).First()
		count, err := locator.Count()
		if err != nil || count == 0 {
			continue
		}
		if visible, err := locator.IsVisible(); err == nil && visible {
			return true
		}
	}
	return false
}

func DismissRegionalPrompt(page playwright.Page, targetURL string) bool {
	if page == nil || page.IsClosed() || !looksLikeRegionalPrompt(page) {
		return false
	}

	log := logger.GetGlobalLogger("crawler/amazon")
	log.Info("检测到跨站点访问提示弹窗，尝试关闭或继续留在当前 Amazon 站点")

	for _, selector := range regionalPromptActionSelectors(targetURL) {
		if page.IsClosed() {
			return false
		}

		locator := page.Locator(selector).First()
		count, err := locator.Count()
		if err != nil || count == 0 {
			continue
		}

		visible, err := locator.IsVisible()
		if err != nil || !visible {
			continue
		}

		if err := locator.Click(playwright.LocatorClickOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			log.Infof("点击跨站点访问提示弹窗按钮失败: selector=%s err=%v", selector, err)
			continue
		}

		time.Sleep(1500 * time.Millisecond)
		log.Infof("已处理跨站点访问提示弹窗: %s", selector)
		return true
	}

	targetHost := normalizedAmazonHost(targetURL)
	// JS 兜底: 优先点击跳回目标 Amazon 站点的链接
	_, _ = page.Evaluate(`([targetHost]) => {
		const candidates = Array.from(document.querySelectorAll('button, a, input[type="button"], input[type="submit"]'));
		for (const el of candidates) {
			const text = (el.innerText || el.value || '').toLowerCase().trim();
			const href = (el.getAttribute && el.getAttribute('href') || '').toLowerCase().trim();
			if (href.includes('mr_donotredirect')) {
				el.click();
				return true;
			}
			if (targetHost && (href.includes(targetHost) || text.includes('go to ' + targetHost) || text.includes('continue shopping on ' + targetHost))) {
				el.click();
				return true;
			}
			if (text.includes('go to amazon.com') || text.includes('continue shopping on amazon.com') || text.includes('continue shopping on amazon')) {
				el.click();
				return true;
			}
		}
		return false;
	}`, targetHost)
	time.Sleep(1500 * time.Millisecond)
	stillVisible := looksLikeRegionalPrompt(page)
	if !stillVisible {
		log.Info("已通过脚本兜底处理跨站点访问提示弹窗")
		return true
	}

	return false
}

func ContainsRegionalPromptText(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	patterns := []string{
		"visiting from singapore",
		"choosing your amazon website",
		"visit amazon.sg",
		"stay on amazon.sg",
		"go to amazon.com",
		"continue shopping on amazon.com",
	}
	for _, pattern := range patterns {
		if strings.Contains(normalized, pattern) {
			return true
		}
	}
	return false
}

func normalizedAmazonHost(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Host))
	if host == "" || !strings.Contains(host, "amazon.") {
		return ""
	}

	return host
}
