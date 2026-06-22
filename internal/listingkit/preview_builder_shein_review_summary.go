package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinPreviewReviewSummary(pkg *SheinPackage) (bool, []string) {
	return sheinworkspace.BuildPreviewReviewSummary(pkg)
}
