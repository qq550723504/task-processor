package studiobatch

import (
	"fmt"
	"strings"
)

func EvaluateGate(input GateInput) GateResult {
	if result := evaluateGateDesign(input); !result.Eligible {
		return result
	}
	if result := evaluateGateSelection(input); !result.Eligible {
		return result
	}
	if result := evaluateGateCompatibility(input); !result.Eligible {
		return result
	}
	return GateResult{Eligible: true}
}

func evaluateGateDesign(input GateInput) GateResult {
	designID := strings.TrimSpace(input.Candidate.Design.ID)
	for _, design := range input.Designs {
		if strings.TrimSpace(design.ID) != designID {
			continue
		}
		if strings.TrimSpace(design.BatchID) != strings.TrimSpace(input.BatchID) ||
			strings.TrimSpace(input.Candidate.Design.BatchID) != strings.TrimSpace(input.BatchID) ||
			strings.TrimSpace(design.ItemID) != strings.TrimSpace(input.Candidate.Item.ID) ||
			strings.TrimSpace(input.Candidate.Design.ItemID) != strings.TrimSpace(input.Candidate.Item.ID) {
			return rejectGate("design_target_mismatch", fmt.Sprintf("design %s does not belong to the requested batch item", designID))
		}
		if !design.Approved || !input.Candidate.Design.Approved {
			return rejectGate("design_not_approved", fmt.Sprintf("design %s is not approved", designID))
		}
		if strings.TrimSpace(design.ImageURL) == "" || strings.TrimSpace(input.Candidate.Design.ImageURL) == "" {
			return rejectGate("design_image_missing", fmt.Sprintf("design %s is missing an image URL", designID))
		}
		return GateResult{Eligible: true}
	}
	return rejectGate("design_not_found", "design was not found for batch task creation")
}

func evaluateGateCompatibility(input GateInput) GateResult {
	candidate := input.Candidate
	selectionID := strings.TrimSpace(candidate.SelectionID)
	fingerprint := compatibilityFingerprint(candidate.SelectionSnapshot)
	if fingerprint == "" || !compatibilityComplete(candidate.SelectionSnapshot) {
		return rejectGate("compatibility_incomplete", fmt.Sprintf("selection %s has an incomplete compatibility fingerprint", selectionID))
	}
	groupMode := firstNonEmpty(candidate.Item.GroupMode, input.BatchGroupMode)
	if groupMode != "per_product" && len(input.ItemSelections) > 1 {
		for _, grouped := range input.ItemSelections {
			otherID := firstNonEmpty(grouped.SelectionID, selectionIDForSnapshot(grouped.Selection))
			otherFingerprint := compatibilityFingerprint(grouped.Selection)
			if otherFingerprint == "" || !compatibilityComplete(grouped.Selection) {
				return rejectGate("compatibility_incomplete", fmt.Sprintf("selection %s has an incomplete compatibility fingerprint", otherID))
			}
			if otherFingerprint != fingerprint {
				return rejectGate("compatibility_mismatch", fmt.Sprintf("selection %s is incompatible with item %s", otherID, strings.TrimSpace(candidate.Item.ID)))
			}
		}
	}
	if target := strings.TrimSpace(candidate.Design.TargetGroupKey); target != "" && strings.TrimSpace(candidate.Item.TargetGroupKey) != "" && target != strings.TrimSpace(candidate.Item.TargetGroupKey) {
		return rejectGate("design_target_mismatch", fmt.Sprintf("design %s target does not match item %s", strings.TrimSpace(candidate.Design.ID), strings.TrimSpace(candidate.Item.ID)))
	}
	return GateResult{Eligible: true}
}

func evaluateGateSelection(input GateInput) GateResult {
	selectionID := strings.TrimSpace(input.Candidate.SelectionID)
	if selectionID == "" {
		return rejectGate("selection_identity_incomplete", "selection identity is incomplete")
	}
	grouped, ok := input.SelectionByID[selectionID]
	if !ok {
		return rejectGate("selection_not_in_batch", fmt.Sprintf("selection %s is not in the batch snapshot", selectionID))
	}
	owned := false
	for _, value := range input.Candidate.Item.SelectionIDs {
		if strings.TrimSpace(value) == selectionID {
			owned = true
			break
		}
	}
	if !owned {
		return rejectGate("selection_not_in_item", fmt.Sprintf("selection %s is not owned by item %s", selectionID, strings.TrimSpace(input.Candidate.Item.ID)))
	}
	if groupedID := strings.TrimSpace(grouped.SelectionID); groupedID != "" && groupedID != selectionID {
		return rejectGate("selection_not_in_batch", fmt.Sprintf("selection %s does not match the batch snapshot", selectionID))
	}
	selection := input.Candidate.SelectionSnapshot
	if selection.ParentProductID <= 0 || selection.PrototypeGroupID <= 0 || selection.VariantID <= 0 ||
		strings.TrimSpace(selection.LayerID) == "" || strings.TrimSpace(selection.DesignType) == "" {
		return rejectGate("selection_identity_incomplete", fmt.Sprintf("selection %s is missing required SDS identity fields", selectionID))
	}
	if len(selection.SelectedVariantIDs) > 0 {
		known := make(map[int64]struct{}, len(selection.Variants)+1)
		known[selection.VariantID] = struct{}{}
		for _, variant := range selection.Variants {
			if variant.VariantID > 0 {
				known[variant.VariantID] = struct{}{}
			}
		}
		for _, selectedID := range selection.SelectedVariantIDs {
			if selectedID <= 0 {
				return rejectGate("selection_variant_incompatible", fmt.Sprintf("selection %s has incompatible variant surface metadata", selectionID))
			}
			if _, ok := known[selectedID]; !ok && len(selection.Variants) > 0 {
				return rejectGate("selection_variant_incompatible", fmt.Sprintf("selection %s has incompatible variant surface metadata", selectionID))
			}
		}
	}
	for _, variant := range selection.Variants {
		if variant.VariantID != selection.VariantID {
			continue
		}
		if variant.PrototypeGroupID > 0 && variant.PrototypeGroupID != selection.PrototypeGroupID ||
			strings.TrimSpace(variant.LayerID) != "" && strings.TrimSpace(variant.LayerID) != strings.TrimSpace(selection.LayerID) ||
			strings.TrimSpace(variant.TemplateImageURL) != "" && strings.TrimSpace(selection.TemplateImageURL) != "" && strings.TrimSpace(variant.TemplateImageURL) != strings.TrimSpace(selection.TemplateImageURL) ||
			strings.TrimSpace(variant.MaskImageURL) != "" && strings.TrimSpace(selection.MaskImageURL) != "" && strings.TrimSpace(variant.MaskImageURL) != strings.TrimSpace(selection.MaskImageURL) {
			return rejectGate("selection_variant_incompatible", fmt.Sprintf("selection %s has incompatible variant surface metadata", selectionID))
		}
	}
	return GateResult{Eligible: true}
}

func rejectGate(reasonCode, message string) GateResult {
	return GateResult{ReasonCode: reasonCode, Message: message}
}
