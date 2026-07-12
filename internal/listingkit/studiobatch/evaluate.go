package studiobatch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	studiodomain "task-processor/internal/listing/studio"
)

func Evaluate(input EvaluationInput) EvaluationResult {
	selections := append([]GroupedSelection(nil), input.ResolvedSelections...)
	if len(selections) == 0 && !input.ExplicitSelectionOwnership && input.FallbackSelection.Selection.VariantID > 0 {
		selections = []GroupedSelection{input.FallbackSelection}
	}
	if len(selections) == 0 {
		return EvaluationResult{Rejections: []Rejection{rejection(input, "", "selection_not_in_batch", fmt.Sprintf("design %s has no resolved selections for batch task creation", strings.TrimSpace(input.Design.ID)))}}
	}

	groupMode := firstNonEmpty(input.Item.GroupMode, input.BatchGroupMode)
	if groupMode == "per_product" && len(selections) != 1 {
		return EvaluationResult{Rejections: []Rejection{rejection(input, strings.Join(selectionIDs(selections), ","), "selection_cardinality_mismatch", fmt.Sprintf("per-product item %s resolved %d selections; exactly one is required", strings.TrimSpace(input.Item.ID), len(selections)))}}
	}

	result := EvaluationResult{Candidates: make([]Candidate, 0, len(selections))}
	var expectedFingerprint string
	for index, grouped := range selections {
		grouped.Selection.DesignType = normalizeDesignType(input.BatchSelection, grouped.Selection.DesignType)
		selectionID := firstNonEmpty(grouped.SelectionID, input.Item.TargetGroupKey, input.Design.TargetGroupKey, input.Design.ID)
		fingerprint := compatibilityFingerprint(grouped.Selection)
		if groupMode != "per_product" && len(selections) > 1 {
			if !compatibilityComplete(grouped.Selection) {
				return EvaluationResult{Rejections: []Rejection{rejection(input, selectionID, "compatibility_incomplete", fmt.Sprintf("design %s has an incomplete compatibility fingerprint for batch task creation", strings.TrimSpace(input.Design.ID)))}}
			}
			if index == 0 {
				expectedFingerprint = fingerprint
			} else if fingerprint != expectedFingerprint {
				return EvaluationResult{Rejections: []Rejection{rejection(input, selectionID, "compatibility_mismatch", fmt.Sprintf("design %s has incompatible grouped selections for batch task creation", strings.TrimSpace(input.Design.ID)))}}
			}
		}
		storeID := grouped.StoreID
		if storeID <= 0 {
			storeID = input.BatchStoreID
		}
		candidate := Candidate{
			Design:                   input.Design,
			Item:                     input.Item,
			Selection:                grouped,
			SelectionSnapshot:        grouped.Selection,
			SelectionID:              selectionID,
			CompatibilityFingerprint: fingerprint,
			StoreID:                  storeID,
			StyleID:                  strings.Join([]string{strings.TrimSpace(input.BatchID), strings.TrimSpace(input.Item.ID), strings.TrimSpace(input.Design.ID), selectionID}, "-"),
			Title:                    firstNonEmpty(grouped.Selection.VariantLabel, grouped.Selection.ProductName, input.Item.TargetGroupLabel, input.Design.TargetGroupLabel, input.Design.ID),
		}
		candidate.CandidateKey = candidateKey(input.TenantID, input.BatchID, candidate)
		result.Candidates = append(result.Candidates, candidate)
	}
	return result
}

func normalizeDesignType(batch Selection, value string) string {
	if strings.TrimSpace(value) == "" {
		value = batch.DesignType
	}
	return studiodomain.NormalizeBatchDesignType(value)
}

func compatibilityComplete(selection Selection) bool {
	return selection.ParentProductID > 0 && selection.PrototypeGroupID > 0 && strings.TrimSpace(selection.LayerID) != "" && strings.TrimSpace(selection.DesignType) != "" && selection.PrintableWidth > 0 && selection.PrintableHeight > 0 && strings.TrimSpace(selection.TemplateImageURL) != ""
}

func compatibilityFingerprint(selection Selection) string {
	return strings.Join([]string{
		strconv.FormatInt(selection.ParentProductID, 10),
		strconv.FormatInt(selection.PrototypeGroupID, 10),
		strings.TrimSpace(selection.LayerID),
		strings.TrimSpace(selection.DesignType),
		strconv.Itoa(selection.PrintableWidth),
		strconv.Itoa(selection.PrintableHeight),
		strings.TrimSpace(selection.TemplateImageURL),
		strings.TrimSpace(selection.MaskImageURL),
		strings.TrimSpace(selection.ProductSize),
		strings.TrimSpace(selection.PackagingSpec),
	}, "|")
}

func candidateKey(tenantID, batchID string, candidate Candidate) string {
	value := strings.Join([]string{
		strings.TrimSpace(tenantID),
		strings.TrimSpace(batchID),
		strings.TrimSpace(candidate.Item.ID),
		strings.TrimSpace(candidate.Design.ID),
		strings.TrimSpace(candidate.SelectionID),
		candidate.CompatibilityFingerprint,
		strconv.FormatInt(candidate.StoreID, 10),
	}, "|")
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func rejection(input EvaluationInput, selectionID, reasonCode, message string) Rejection {
	return Rejection{DesignID: strings.TrimSpace(input.Design.ID), ItemID: strings.TrimSpace(input.Item.ID), SelectionID: strings.TrimSpace(selectionID), ReasonCode: reasonCode, Message: strings.TrimSpace(message)}
}

func selectionIDs(selections []GroupedSelection) []string {
	result := make([]string, 0, len(selections))
	for _, grouped := range selections {
		if id := strings.TrimSpace(grouped.SelectionID); id != "" {
			result = append(result, id)
		}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
