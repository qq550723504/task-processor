package shein

import "time"

// ApplyImageUploadCache stores the SHEIN image upload cache on the final submission draft.
func ApplyImageUploadCache(pkg *Package, uploadCache map[string]string, now time.Time) bool {
	if len(uploadCache) == 0 {
		return false
	}
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	if pkg.FinalSubmissionDraft == nil {
		pkg.FinalSubmissionDraft = &FinalDraft{}
	}
	pkg.FinalSubmissionDraft.SheinImageUploadCache = uploadCache
	pkg.FinalSubmissionDraft.UpdatedAt = &now
	return true
}
