package publishing

import "strings"

// FinalReviewRequired reports whether the action requires explicit final review.
func FinalReviewRequired(action string) bool {
	return !strings.EqualFold(strings.TrimSpace(action), "save_draft")
}

// FinalReviewMessage returns the readiness message for final review confirmation.
func FinalReviewMessage(action string) string {
	if !FinalReviewRequired(action) {
		return "保存草稿允许跳过最终确认；正式发布前仍需在最终确认页核对图片、价格、属性和 SKU"
	}
	return "提交前必须在最终确认页核对图片、价格、属性和 SKU 后确认"
}
