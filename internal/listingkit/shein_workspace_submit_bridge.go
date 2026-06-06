// Adapter-only bridge. Keep domain rules in internal/workspace/shein.
package listingkit

import listingworkspace "task-processor/internal/listingkit/workspace/shein"

type SheinSubmitReadiness = listingworkspace.SubmitReadiness[SheinReadinessReason, SheinRepairHint]
type SheinReadinessItem = listingworkspace.ReadinessItem[SheinReadinessReason, SheinRepairHint]
type SheinReadinessCheck = listingworkspace.ReadinessCheck[SheinReadinessReason, SheinRepairHint]
type SheinSubmitChecklist = listingworkspace.SubmitChecklist[SheinReadinessReason, SheinRepairHint]
type SheinChecklistGroupItem = listingworkspace.ChecklistGroupItem[SheinReadinessReason, SheinRepairHint]

func buildSheinSubmitChecklist(readiness *SheinSubmitReadiness) *SheinSubmitChecklist {
	return listingworkspace.BuildSubmitChecklist(readiness)
}
