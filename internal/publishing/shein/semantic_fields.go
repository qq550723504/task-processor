package shein

import (
	"encoding/json"

	sheinproduct "task-processor/internal/shein/api/product"
)

func NormalizePackageSemanticFields(pkg *Package) *Package {
	if pkg == nil {
		return nil
	}
	// Keep legacy and semantic aliases mirrored so historical task payloads and
	// older JSON consumers continue to work while new business logic uses the
	// semantic field names exclusively.
	if pkg.DraftPayload == nil {
		pkg.DraftPayload = pkg.RequestDraft
	}
	pkg.RequestDraft = pkg.DraftPayload
	if pkg.PreviewPayload == nil {
		pkg.PreviewPayload = pkg.PreviewProduct
	}
	pkg.PreviewProduct = pkg.PreviewPayload
	if pkg.SubmissionState == nil {
		pkg.SubmissionState = pkg.Submission
	}
	pkg.Submission = pkg.SubmissionState
	if pkg.FinalSubmissionDraft == nil {
		pkg.FinalSubmissionDraft = pkg.FinalDraft
	}
	pkg.FinalDraft = pkg.FinalSubmissionDraft
	return pkg
}

// SetPreviewPayload updates both the semantic preview field and the legacy
// compatibility alias so callers do not accidentally leave stale preview data
// behind after rebuilding the preview payload.
func SetPreviewPayload(pkg *Package, preview *sheinproduct.Product) *Package {
	if pkg == nil {
		return nil
	}
	pkg.PreviewPayload = preview
	pkg.PreviewProduct = preview
	return pkg
}

func (p *Package) MarshalJSON() ([]byte, error) {
	type alias Package
	NormalizePackageSemanticFields(p)
	return json.Marshal((*alias)(p))
}

func (p *Package) UnmarshalJSON(data []byte) error {
	type alias Package
	aux := (*alias)(p)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	NormalizePackageSemanticFields(p)
	return nil
}
