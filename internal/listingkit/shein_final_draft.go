package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func applySheinFinalImageDraft(pkg *sheinpub.Package) {
	sheinpub.ApplyFinalImageDraft(pkg)
}

func ensureSheinFinalDraftSKCImages(pkg *sheinpub.Package, main string, order []string, deleted map[string]struct{}) {
	sheinpub.EnsureFinalDraftSKCImages(pkg, main, order, deleted)
}

func ensureSheinFinalPreviewSKCImages(pkg *sheinpub.Package) {
	sheinpub.EnsureFinalPreviewSKCImages(pkg)
}
