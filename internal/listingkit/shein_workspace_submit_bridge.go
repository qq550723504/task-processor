// Adapter-only bridge. Keep domain rules in internal/marketplace/shein/workspace.
package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

type SheinSubmitReadiness = sheinworkspace.SubmitReadiness[SheinReadinessReason, SheinRepairHint]
type SheinReadinessItem = sheinworkspace.ReadinessItem[SheinReadinessReason, SheinRepairHint]
type SheinReadinessCheck = sheinworkspace.ReadinessCheck[SheinReadinessReason, SheinRepairHint]
type SheinSubmitChecklist = sheinworkspace.SubmitChecklist[SheinReadinessReason, SheinRepairHint]
type SheinChecklistGroupItem = sheinworkspace.ChecklistGroupItem[SheinReadinessReason, SheinRepairHint]
