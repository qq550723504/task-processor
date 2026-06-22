package listingkit

import (
	"context"
	"strings"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
)

func (s *service) decorateSheinCookieAvailabilityPreview(ctx context.Context, task *Task, preview *ListingKitPreview) {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil || preview == nil || preview.Shein == nil {
		return
	}

	pkg := *task.Result.Shein
	pkg.ReviewNotes = append([]string(nil), task.Result.Shein.ReviewNotes...)
	sheinworkspace.StripCookieUnavailableReviewNotes(&pkg)

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
