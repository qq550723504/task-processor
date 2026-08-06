package shein

import (
	"strconv"
	"strings"
)

// DraftPayloadIssue describes a structural problem in the SHEIN draft
// payload. Platform template resolution and submit-policy errors are reported
// by their respective layers; this type only covers the draft shape.
type DraftPayloadIssue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// DraftPayloadOf returns the semantic SHEIN draft after normalizing the
// historical request_draft alias.
func DraftPayloadOf(pkg *Package) *RequestDraft {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil
	}
	return pkg.DraftPayload
}

// EnsureDraftPayload returns the semantic SHEIN draft, creating it when a
// package needs to start building one. The historical alias is kept in sync
// for old JSON readers and writers.
func EnsureDraftPayload(pkg *Package) *RequestDraft {
	if pkg == nil {
		return nil
	}
	NormalizePackageSemanticFields(pkg)
	if pkg.DraftPayload == nil {
		pkg.DraftPayload = &RequestDraft{}
		pkg.RequestDraft = pkg.DraftPayload
	}
	return pkg.DraftPayload
}

// SetDraftPayload makes the semantic draft the single new write target while
// mirroring it to the legacy JSON compatibility field.
func SetDraftPayload(pkg *Package, draft *RequestDraft) *Package {
	if pkg == nil {
		return nil
	}
	pkg.DraftPayload = draft
	pkg.RequestDraft = draft
	return pkg
}

// ValidateDraftPayload checks only the platform draft's structural shape.
// Category, attribute, sale-attribute, image, price, and submission checks
// belong to the SHEIN workspace and publishing policies.
func ValidateDraftPayload(draft *RequestDraft) []DraftPayloadIssue {
	if draft == nil {
		return []DraftPayloadIssue{{
			Path:    "draft_payload",
			Code:    "draft_payload_required",
			Message: "SHEIN draft payload is required",
		}}
	}

	issues := make([]DraftPayloadIssue, 0)
	if len(draft.SKCList) == 0 {
		issues = append(issues, DraftPayloadIssue{
			Path:    "skc_list",
			Code:    "skc_list_required",
			Message: "SHEIN draft must contain at least one SKC",
		})
		return issues
	}

	for skcIndex, skc := range draft.SKCList {
		path := "skc_list[" + formatIndex(skcIndex) + "]"
		if len(skc.SKUList) == 0 {
			issues = append(issues, DraftPayloadIssue{
				Path:    path + ".sku_list",
				Code:    "skc_sku_list_required",
				Message: "each SHEIN SKC must contain at least one SKU",
			})
			continue
		}
		for skuIndex, sku := range skc.SKUList {
			if strings.TrimSpace(sku.SupplierSKU) == "" {
				issues = append(issues, DraftPayloadIssue{
					Path:    path + ".sku_list[" + formatIndex(skuIndex) + "].supplier_sku",
					Code:    "supplier_sku_required",
					Message: "each SHEIN SKU must contain a supplier SKU",
				})
			}
		}
	}
	return issues
}

func formatIndex(index int) string {
	return strconv.Itoa(index)
}
