package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

func BuildRevisionInputFromEditorContext(ctx *EditorContext) *RevisionInput {
	return sheinmarketplace.BuildRevisionInputFromEditorContext(ctx)
}
