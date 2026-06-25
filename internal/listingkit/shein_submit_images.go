package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
)

func sheinImageUploadCache(pkg *SheinPackage) map[string]string {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.FinalSubmissionDraft == nil {
		return nil
	}
	return pkg.FinalSubmissionDraft.SheinImageUploadCache
}
