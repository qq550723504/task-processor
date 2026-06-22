package listingkit

import (
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func buildSheinFinalReviewImages(draft *SheinRequestDraft, finalDraft *sheinpub.FinalDraft, product *sheinproduct.Product) []SheinFinalReviewImage {
	return sheinworkspace.BuildFinalReviewImages(draft, finalDraft, product)
}
