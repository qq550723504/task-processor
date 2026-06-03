package listingkit

import sheinworkspace "task-processor/internal/workspace/shein"

func buildSheinSubmitReadinessGuidanceResolver(
	pkg *SheinPackage,
) func(spec sheinworkspace.ReadinessCheckSpec) sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint] {
	return func(spec sheinworkspace.ReadinessCheckSpec) sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint] {
		guidance := buildSheinReadinessGuidance(pkg, spec.Key, spec.FieldPaths, spec.SuggestedAction, spec.WarningOnly)
		return sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint]{
			Reason:      cloneSheinReadinessReason(guidance.reason),
			RepairHints: cloneSheinRepairHints(guidance.repairHints),
		}
	}
}
