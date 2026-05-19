package listingkit

import (
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
)

func buildSummary(task *Task, canonical *canonical.Product, image *productimage.ImageProcessResult) *GenerationSummary {
	summary := &GenerationSummary{}
	if task != nil && task.Request != nil {
		summary.SourceType = detectSourceType(task.Request)
		summary.ImageCount = len(task.Request.ImageURLs)
	}
	if canonical != nil {
		summary.VariantCount = len(canonical.Variants)
		summary.NeedsReview = canonical.NeedsReview
	}
	if image != nil && image.Review != nil && image.Review.NeedsReview {
		summary.NeedsReview = true
		summary.Warnings = append(summary.Warnings, image.Review.Reasons...)
	}
	return summary
}
