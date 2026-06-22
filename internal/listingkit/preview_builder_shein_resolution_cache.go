package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinResolutionCacheSummary(pkg *SheinPackage) *SheinResolutionCacheSummary {
	return sheinworkspace.BuildResolutionCacheSummary(pkg)
}
