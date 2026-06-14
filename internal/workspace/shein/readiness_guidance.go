package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type ReadinessReasonSpec = sheinmarketplace.ReadinessReasonSpec
type ReadinessHintSpec = sheinmarketplace.ReadinessHintSpec
type ReadinessGuidanceSpec = sheinmarketplace.ReadinessGuidanceSpec

func BuildReadinessGuidanceSpec(key string, warningOnly bool) *ReadinessGuidanceSpec {
	return sheinmarketplace.BuildReadinessGuidanceSpec(key, warningOnly)
}
