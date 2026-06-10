package listingkit

import (
	"context"
	"strings"
)

func (s *service) decorateSheinCookieAvailabilityPreview(ctx context.Context, task *Task, preview *ListingKitPreview) {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil || preview == nil || preview.Shein == nil {
		return
	}

	pkg := *task.Result.Shein
	pkg.ReviewNotes = append([]string(nil), task.Result.Shein.ReviewNotes...)
	stripSheinCookieUnavailableReviewNotes(&pkg)

	note := s.resolveSheinCookieAvailabilityNote(ctx, task)
	if strings.TrimSpace(note) != "" {
		refreshSheinReviewState(&pkg, note)
	} else {
		refreshSheinReviewState(&pkg)
	}

	rebuilt := buildSheinPreviewPayload(
		&pkg,
		task.Result.PodExecution,
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
