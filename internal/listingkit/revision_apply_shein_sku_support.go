package listingkit

import (
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func applySheinSKCRevisionPatches(pkg *sheinpub.Package, patches []SheinSKCRevisionPatch) {
	sheinworkspace.ApplySKCRevisionPatches(pkg, patches)
}
