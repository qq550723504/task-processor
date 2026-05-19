// Adapter-only bridge. Keep domain rules in internal/workspace/shein.
package listingkit

import sheinworkspace "task-processor/internal/workspace/shein"

type SheinSubmitReadiness = sheinworkspace.SubmitReadiness[SheinReadinessReason, SheinRepairHint]
type SheinReadinessItem = sheinworkspace.ReadinessItem[SheinReadinessReason, SheinRepairHint]
type SheinReadinessCheck = sheinworkspace.ReadinessCheck[SheinReadinessReason, SheinRepairHint]
type SheinSubmitChecklist = sheinworkspace.SubmitChecklist[SheinReadinessReason, SheinRepairHint]
type SheinChecklistGroupItem = sheinworkspace.ChecklistGroupItem[SheinReadinessReason, SheinRepairHint]

func buildSheinSubmitChecklist(readiness *SheinSubmitReadiness) *SheinSubmitChecklist {
	return sheinworkspace.BuildSubmitChecklist(readiness, sheinworkspace.SubmitChecklistGroupForKey)
}
