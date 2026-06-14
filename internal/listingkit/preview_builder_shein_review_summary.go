package listingkit

import sheinworkspace "task-processor/internal/listingkit/workspace/shein"

func buildSheinPreviewReviewSummary(pkg *SheinPackage) (bool, []string) {
	return sheinworkspace.BuildPreviewReviewSummary(pkg)
}
