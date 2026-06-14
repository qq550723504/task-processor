package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildEditorProgress(pkg *sheinpub.Package, checklistTotal int) *EditorProgress {
	return sheinmarketplace.BuildEditorProgress(pkg, checklistTotal)
}
