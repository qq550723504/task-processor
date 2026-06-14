package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

func BuildRestoreDraftFromSkeleton(reason string, skeleton *EditorRevisionSkeleton) *EditorRevisionSkeleton {
	return sheinmarketplace.BuildRestoreDraftFromSkeleton(reason, skeleton)
}

func CloneEditorRevisionSkeleton(src *EditorRevisionSkeleton) *EditorRevisionSkeleton {
	return sheinmarketplace.CloneEditorRevisionSkeleton(src)
}

func CloneRevisionInput(src *RevisionInput) *RevisionInput {
	return sheinmarketplace.CloneRevisionInput(src)
}
