// Adapter-only bridge. Keep domain rules in internal/marketplace/shein/workspace.
package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

type SheinRepairCenter = sheinworkspace.RepairCenter[SheinReadinessReason, SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]
type SheinRepairCenterStats = sheinworkspace.RepairCenterStats
type SheinRepairCenterSection = sheinworkspace.RepairCenterSection
type SheinRepairCenterAction = sheinworkspace.RepairCenterAction[SheinReadinessReason, SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]
type SheinRepairPlan = sheinworkspace.RepairPlan
type SheinRepairPlanStep = sheinworkspace.RepairPlanStep
type SheinRepairApplyQueue = sheinworkspace.RepairApplyQueue[ApplyRevisionRequest, SheinRepairValidationPreview]
type SheinRepairApplyQueueItem = sheinworkspace.RepairApplyQueueItem[ApplyRevisionRequest, SheinRepairValidationPreview]
type SheinRepairSession = sheinworkspace.RepairSession
type SheinRepairResumeState = sheinworkspace.RepairResumeState
type SheinRepairCompletionSnapshot = sheinworkspace.RepairCompletionSnapshot
type SheinRepairRunbookStep = sheinworkspace.RepairRunbookStep
