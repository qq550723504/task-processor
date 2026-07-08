package sherr

import (
	"errors"
	"strings"
)

func isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var re *retryableError
	if errors.As(err, &re) {
		return !re.IsRetryable()
	}
	var fe *FilteredError
	if errors.As(err, &fe) {
		return true
	}
	var cookieErr *CookieLoadError
	if errors.As(err, &cookieErr) {
		return true
	}

	notFoundPatterns := []string{
		"不是有效的产品页面", "产品页面不存在", "产品页面缺少必要元素",
		"页面不存在(404)", "页面不存在", "页面未准备就绪: 页面不存在",
		"product not found", "Product not found", "404", "not found", "Not Found",
		"卖家SKU重复", "变体ASIN数量过多",
	}
	for err != nil {
		msg := err.Error()
		for _, p := range notFoundPatterns {
			if strings.Contains(msg, p) {
				return true
			}
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
		} else {
			break
		}
	}
	return false
}
