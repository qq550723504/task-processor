package listingkit

import (
	"fmt"

	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
)

func buildRevisionHistoryRestoreDraft(record *ListingKitRevisionRecord) *SheinEditorRevisionSkeleton {
	if record == nil {
		return nil
	}
	switch record.Platform {
	case "shein":
		return buildSheinRestoreDraft(record)
	default:
		return nil
	}
}

func buildSheinRestoreDraft(record *ListingKitRevisionRecord) *SheinEditorRevisionSkeleton {
	if record == nil || record.EditorContext == nil {
		return nil
	}
	if record.EditorContext.RevisionSkeleton != nil {
		return sheinworkspace.BuildRestoreDraftFromSkeleton(buildRevisionHistoryRestoreReason(record), record.EditorContext.RevisionSkeleton)
	}

	restore := &SheinEditorRevisionSkeleton{
		Platform: "shein",
		Actor:    "desktop-client",
		Reason:   buildRevisionHistoryRestoreReason(record),
		Shein:    sheinworkspace.BuildRevisionInputFromEditorContext(record.EditorContext),
	}
	if restore.Shein == nil {
		return nil
	}
	return restore
}

func buildRevisionHistoryRestoreReason(record *ListingKitRevisionRecord) string {
	if record == nil {
		return "restore from revision history"
	}
	if record.Reason != "" {
		return fmt.Sprintf("restore: %s", record.Reason)
	}
	return "restore from revision history"
}

func cloneSheinEditorRevisionSkeleton(src *SheinEditorRevisionSkeleton) *SheinEditorRevisionSkeleton {
	return sheinworkspace.CloneEditorRevisionSkeleton(src)
}

func cloneHistorySheinRevisionInput(src *SheinRevisionInput) *SheinRevisionInput {
	return sheinworkspace.CloneRevisionInput(src)
}

func cloneHistoryStringPointer(src *string) *string {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}
