package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

type EditorDirtyHints = sheinmarketplace.EditorDirtyHints
type EditorDirtyHintSection = sheinmarketplace.EditorDirtyHintSection

func BuildEditorDirtyHints(pkg *sheinpub.Package) *EditorDirtyHints {
	return sheinmarketplace.BuildEditorDirtyHints(pkg)
}
