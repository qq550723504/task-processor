package shein

import (
	"strings"
	"time"

	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
)

// FinalDraftUpdate describes admin/user edits to a SHEIN final submission draft.
type FinalDraftUpdate struct {
	Confirmed            *bool
	SubmitMode           string
	ManualPriceOverrides map[string]float64
	FinalImageOrder      *[]string
	MainImageURL         string
	DeletedImageURLs     *[]string
	ImageRoleOverrides   map[string]string
}

// ApplyFinalDraftUpdate applies admin/user edits to the package final submission draft.
func ApplyFinalDraftUpdate(pkg *Package, update FinalDraftUpdate, now time.Time) *FinalDraft {
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
	draft := pkg.FinalSubmissionDraft
	if update.SubmitMode != "" {
		if mode := sheinmarketpub.NormalizeFinalDraftSubmitMode(update.SubmitMode); mode != "" {
			draft.SubmitMode = mode
		}
	}
	if len(update.ManualPriceOverrides) > 0 {
		draft.ManualPriceOverrides = clonePricingOverrides(update.ManualPriceOverrides)
	}
	if update.FinalImageOrder != nil {
		draft.FinalImageOrder = uniqueNonEmptyFinalDraftStrings(*update.FinalImageOrder)
	}
	if value := strings.TrimSpace(update.MainImageURL); value != "" {
		draft.MainImageURL = value
	}
	if update.DeletedImageURLs != nil {
		draft.DeletedImageURLs = uniqueNonEmptyFinalDraftStrings(*update.DeletedImageURLs)
	}
	if len(update.ImageRoleOverrides) > 0 {
		draft.ImageRoleOverrides = NormalizeImageRoleOverrides(update.ImageRoleOverrides)
	}
	if update.Confirmed != nil {
		draft.Confirmed = *update.Confirmed
		if *update.Confirmed {
			draft.ConfirmedAt = &now
		} else {
			draft.ConfirmedAt = nil
		}
	}
	draft.UpdatedAt = &now
	return draft
}

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
