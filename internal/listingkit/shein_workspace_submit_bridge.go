package listingkit

import sheinworkspace "task-processor/internal/workspace/shein"

type SheinSubmitReadiness = sheinworkspace.SubmitReadiness[SheinReadinessReason, SheinRepairHint]
type SheinReadinessItem = sheinworkspace.ReadinessItem[SheinReadinessReason, SheinRepairHint]
type SheinReadinessCheck = sheinworkspace.ReadinessCheck[SheinReadinessReason, SheinRepairHint]
type SheinSubmitChecklist = sheinworkspace.SubmitChecklist[SheinReadinessReason, SheinRepairHint]
type SheinChecklistGroupItem = sheinworkspace.ChecklistGroupItem[SheinReadinessReason, SheinRepairHint]

func buildSheinSubmitChecklist(readiness *SheinSubmitReadiness) *SheinSubmitChecklist {
	return sheinworkspace.BuildSubmitChecklist(readiness, checklistGroupForCheck)
}

func checklistGroupForCheck(key string) string {
	switch key {
	case "category", "category_review", "attributes", "attribute_review", "sale_attributes", "images", "variants":
		return "required"
	case "request_draft", "preview_product":
		return "recommended"
	default:
		return "optional"
	}
}
