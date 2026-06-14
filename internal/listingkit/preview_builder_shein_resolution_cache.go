package listingkit

import sheinworkspace "task-processor/internal/listingkit/workspace/shein"

func buildSheinResolutionCacheSummary(pkg *SheinPackage) *SheinResolutionCacheSummary {
	return sheinworkspace.BuildResolutionCacheSummary(pkg)
}
