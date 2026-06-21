package shein

import "time"

// ConfirmFinalSubmissionDraft marks the final submission draft as confirmed for a submit action.
func ConfirmFinalSubmissionDraft(pkg *Package, action string, now time.Time) *FinalDraft {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	if pkg.FinalSubmissionDraft == nil {
		pkg.FinalSubmissionDraft = &FinalDraft{}
	}
	pkg.FinalSubmissionDraft.Confirmed = true
	pkg.FinalSubmissionDraft.ConfirmedAt = &now
	pkg.FinalSubmissionDraft.UpdatedAt = &now
	if pkg.FinalSubmissionDraft.SubmitMode == "" {
		pkg.FinalSubmissionDraft.SubmitMode = action
	}
	return pkg.FinalSubmissionDraft
}
