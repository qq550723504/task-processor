package listingkit

import (
	"context"
	"fmt"
	"strings"
)

func (s *service) decorateSheinCookieAvailabilityPreview(ctx context.Context, task *Task, preview *ListingKitPreview) {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil || preview == nil || preview.Shein == nil {
		return
	}

	note := s.resolveSheinCookieAvailabilityNote(ctx, task)
	if strings.TrimSpace(note) == "" {
		return
	}

	pkg := *task.Result.Shein
	pkg.ReviewNotes = append([]string(nil), task.Result.Shein.ReviewNotes...)
	refreshSheinReviewState(&pkg, note)

	rebuilt := buildSheinPreviewPayload(
		&pkg,
		task.Result.CanonicalProduct,
		task.Result.AssetBundle,
		preview.Shein.RenderPreviews,
	)
	if rebuilt == nil {
		return
	}
	preview.Shein = rebuilt
	preview.NeedsReview = preview.NeedsReview || rebuilt.NeedsReview
}

func (s *service) resolveSheinCookieAvailabilityNote(ctx context.Context, task *Task) string {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil {
		return ""
	}
	if sheinCookieUnavailable(task.Result.Shein) {
		return ""
	}
	if s.sheinStoreCatalog == nil || s.sheinAPIClientFactory == nil {
		return ""
	}

	apiClient, _, err := s.newSheinAPIClient(ctx, task)
	if err != nil {
		return fmt.Sprintf("SHEIN 店铺 cookie 不可用，在线类目、属性和销售属性解析受阻：%v", err)
	}
	if apiClient.HasCookies() {
		return ""
	}
	if err := apiClient.ForceRefreshCookies(); err != nil {
		return fmt.Sprintf("SHEIN 店铺 cookie 不可用，在线类目、属性和销售属性解析受阻：%v", err)
	}
	if !apiClient.HasCookies() {
		return "SHEIN 店铺 cookie 不可用，在线类目、属性和销售属性解析受阻：刷新后仍未获取到有效 cookie"
	}
	return ""
}
