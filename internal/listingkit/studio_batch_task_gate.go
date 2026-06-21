package listingkit

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/tenantbridge"
)

type StudioBatchBaselineReadinessChecker interface {
	CheckStudioBatchBaselineReadiness(ctx context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineCacheEntry, error)
}

type StudioBatchStoreValidator interface {
	ValidateStudioBatchStore(ctx context.Context, tenantID string, storeID int64) (studioBatchStoreValidationResult, error)
}

type studioBatchStoreValidationResult struct {
	Exists    bool
	Valid     bool
	Available bool
	Message   string
}

type studioBatchTaskGateResult struct {
	Eligible   bool
	ReasonCode string
	Message    string
}

type studioBatchTaskGateEvaluation struct {
	Batch           *StudioBatchRecord
	Candidate       studioBatchTaskCandidate
	DesignsByID     map[string]StudioMaterializedDesignRecord
	SelectionsByID  map[string]SheinStudioGroupedSelection
	ItemSelections  []SheinStudioGroupedSelection
	BaselineChecker StudioBatchBaselineReadinessChecker
	StoreValidator  StudioBatchStoreValidator
}

type studioBatchTaskGate struct {
	baselineChecker StudioBatchBaselineReadinessChecker
	storeValidator  StudioBatchStoreValidator
	baselineCache   map[string]studioBatchTaskGateBaselineCacheEntry
	storeCache      map[string]studioBatchTaskGateStoreCacheEntry
	compatCache     map[string]string
}

type studioBatchTaskGateBaselineCacheEntry struct {
	entry *SDSBaselineCacheEntry
	err   error
}

type studioBatchTaskGateStoreCacheEntry struct {
	result studioBatchStoreValidationResult
	err    error
}

func newStudioBatchTaskGate(
	baselineChecker StudioBatchBaselineReadinessChecker,
	storeValidator StudioBatchStoreValidator,
) *studioBatchTaskGate {
	return &studioBatchTaskGate{
		baselineChecker: baselineChecker,
		storeValidator:  storeValidator,
		baselineCache:   make(map[string]studioBatchTaskGateBaselineCacheEntry),
		storeCache:      make(map[string]studioBatchTaskGateStoreCacheEntry),
		compatCache:     make(map[string]string),
	}
}

func (g *studioBatchTaskGate) Evaluate(ctx context.Context, eval *studioBatchTaskGateEvaluation) (studioBatchTaskGateResult, error) {
	if eval == nil {
		return rejectStudioBatchTaskGate("design_not_found", "task candidate is missing"), nil
	}
	if eval.BaselineChecker != nil {
		g.baselineChecker = eval.BaselineChecker
	}
	if eval.StoreValidator != nil {
		g.storeValidator = eval.StoreValidator
	}
	if result := g.evaluateDesign(eval); !result.Eligible {
		return result, nil
	}
	if result := g.evaluateSelection(eval); !result.Eligible {
		return result, nil
	}
	if result := g.evaluateCompatibility(eval); !result.Eligible {
		return result, nil
	}
	if result, err := g.evaluateStore(ctx, eval); err != nil || !result.Eligible {
		return result, err
	}
	if result := g.evaluateBaseline(ctx, eval); !result.Eligible {
		return result, nil
	}
	return studioBatchTaskGateResult{Eligible: true}, nil
}

func (g *studioBatchTaskGate) evaluateDesign(eval *studioBatchTaskGateEvaluation) studioBatchTaskGateResult {
	candidate := eval.Candidate
	designID := strings.TrimSpace(candidate.Design.ID)
	design, ok := eval.DesignsByID[designID]
	if designID == "" || !ok {
		return rejectStudioBatchTaskGate("design_not_found", "design was not found for batch task creation")
	}
	batchID := ""
	if eval.Batch != nil {
		batchID = strings.TrimSpace(eval.Batch.ID)
	}
	if strings.TrimSpace(design.BatchID) != batchID ||
		strings.TrimSpace(candidate.Design.BatchID) != batchID ||
		strings.TrimSpace(design.ItemID) != strings.TrimSpace(candidate.Item.ID) ||
		strings.TrimSpace(candidate.Design.ItemID) != strings.TrimSpace(candidate.Item.ID) {
		return rejectStudioBatchTaskGate("design_target_mismatch", fmt.Sprintf("design %s does not belong to the requested batch item", designID))
	}
	if design.ReviewStatus != StudioMaterializedDesignReviewStatusApproved ||
		candidate.Design.ReviewStatus != StudioMaterializedDesignReviewStatusApproved {
		return rejectStudioBatchTaskGate("design_not_approved", fmt.Sprintf("design %s is not approved", designID))
	}
	if strings.TrimSpace(design.ImageURL) == "" || strings.TrimSpace(candidate.Design.ImageURL) == "" {
		return rejectStudioBatchTaskGate("design_image_missing", fmt.Sprintf("design %s is missing an image URL", designID))
	}
	return studioBatchTaskGateResult{Eligible: true}
}

func (g *studioBatchTaskGate) evaluateSelection(eval *studioBatchTaskGateEvaluation) studioBatchTaskGateResult {
	candidate := eval.Candidate
	selectionID := strings.TrimSpace(candidate.SelectionID)
	if selectionID == "" {
		return rejectStudioBatchTaskGate("selection_identity_incomplete", "selection identity is incomplete")
	}
	grouped, ok := eval.SelectionsByID[selectionID]
	if !ok {
		return rejectStudioBatchTaskGate("selection_not_in_batch", fmt.Sprintf("selection %s is not in the batch snapshot", selectionID))
	}
	if !studioBatchTaskItemOwnsSelection(candidate.Item, selectionID) {
		return rejectStudioBatchTaskGate("selection_not_in_item", fmt.Sprintf("selection %s is not owned by item %s", selectionID, strings.TrimSpace(candidate.Item.ID)))
	}
	selection := candidate.SelectionSnapshot
	if selection.ParentProductID <= 0 || selection.PrototypeGroupID <= 0 || selection.VariantID <= 0 ||
		strings.TrimSpace(selection.LayerID) == "" || strings.TrimSpace(selection.DesignType) == "" {
		return rejectStudioBatchTaskGate("selection_identity_incomplete", fmt.Sprintf("selection %s is missing required SDS identity fields", selectionID))
	}
	if !studioBatchSelectionVariantsCompatible(selection) {
		return rejectStudioBatchTaskGate("selection_variant_incompatible", fmt.Sprintf("selection %s has incompatible variant surface metadata", selectionID))
	}
	if strings.TrimSpace(grouped.SelectionID) != "" && strings.TrimSpace(grouped.SelectionID) != selectionID {
		return rejectStudioBatchTaskGate("selection_not_in_batch", fmt.Sprintf("selection %s does not match the batch snapshot", selectionID))
	}
	return studioBatchTaskGateResult{Eligible: true}
}

func (g *studioBatchTaskGate) evaluateCompatibility(eval *studioBatchTaskGateEvaluation) studioBatchTaskGateResult {
	candidate := eval.Candidate
	selectionID := strings.TrimSpace(candidate.SelectionID)
	fingerprint := g.compatibilityFingerprint(selectionID, candidate.SelectionSnapshot)
	if fingerprint == "" || !studioBatchCompatibilityFingerprintComplete(candidate.SelectionSnapshot) {
		return rejectStudioBatchTaskGate("compatibility_incomplete", fmt.Sprintf("selection %s has an incomplete compatibility fingerprint", selectionID))
	}
	groupMode := studioBatchTaskCandidateGroupMode(eval.Batch, candidate.Item)
	if groupMode != "per_product" && len(eval.ItemSelections) > 1 {
		for _, grouped := range eval.ItemSelections {
			otherID := strings.TrimSpace(grouped.SelectionID)
			if otherID == "" {
				otherID = selectionIDForStudioSelection(grouped.Selection)
			}
			otherFingerprint := g.compatibilityFingerprint(otherID, grouped.Selection)
			if otherFingerprint == "" || !studioBatchCompatibilityFingerprintComplete(grouped.Selection) {
				return rejectStudioBatchTaskGate("compatibility_incomplete", fmt.Sprintf("selection %s has an incomplete compatibility fingerprint", otherID))
			}
			if otherFingerprint != fingerprint {
				return rejectStudioBatchTaskGate("compatibility_mismatch", fmt.Sprintf("selection %s is incompatible with item %s", otherID, strings.TrimSpace(candidate.Item.ID)))
			}
		}
	}
	if target := strings.TrimSpace(candidate.Design.TargetGroupKey); target != "" && strings.TrimSpace(candidate.Item.TargetGroupKey) != "" && target != strings.TrimSpace(candidate.Item.TargetGroupKey) {
		return rejectStudioBatchTaskGate("design_target_mismatch", fmt.Sprintf("design %s target does not match item %s", strings.TrimSpace(candidate.Design.ID), strings.TrimSpace(candidate.Item.ID)))
	}
	return studioBatchTaskGateResult{Eligible: true}
}

func (g *studioBatchTaskGate) evaluateStore(ctx context.Context, eval *studioBatchTaskGateEvaluation) (studioBatchTaskGateResult, error) {
	storeID := eval.Candidate.SheinStoreID
	if storeID <= 0 {
		return rejectStudioBatchTaskGate("store_missing", "a positive SHEIN store id is required"), nil
	}
	if g.storeValidator == nil {
		return studioBatchTaskGateResult{Eligible: true}, nil
	}
	tenantID := studioBatchTaskGateTenantID(ctx, eval.Batch)
	cacheKey := tenantID + "|" + strconv.FormatInt(storeID, 10)
	cached, ok := g.storeCache[cacheKey]
	if !ok {
		result, err := g.storeValidator.ValidateStudioBatchStore(ctx, tenantID, storeID)
		cached = studioBatchTaskGateStoreCacheEntry{result: result, err: err}
		g.storeCache[cacheKey] = cached
	}
	if cached.err != nil {
		return studioBatchTaskGateResult{}, cached.err
	}
	if !cached.result.Exists || !cached.result.Valid {
		return rejectStudioBatchTaskGate("store_invalid", firstNonEmpty(cached.result.Message, fmt.Sprintf("SHEIN store %d is invalid", storeID))), nil
	}
	if !cached.result.Available {
		return rejectStudioBatchTaskGate("store_not_available", firstNonEmpty(cached.result.Message, fmt.Sprintf("SHEIN store %d is not available", storeID))), nil
	}
	return studioBatchTaskGateResult{Eligible: true}, nil
}

func (g *studioBatchTaskGate) evaluateBaseline(ctx context.Context, eval *studioBatchTaskGateEvaluation) studioBatchTaskGateResult {
	if g.baselineChecker == nil {
		return studioBatchTaskGateResult{Eligible: true}
	}
	selection := eval.Candidate.SelectionSnapshot
	tenantID := studioBatchTaskGateTenantID(ctx, eval.Batch)
	query := &SDSBaselineReadinessQuery{
		TenantID:           tenantID,
		ParentProductID:    selection.ParentProductID,
		PrototypeGroupID:   selection.PrototypeGroupID,
		VariantID:          selection.VariantID,
		SelectedVariantIDs: append([]int64(nil), selection.SelectedVariantIDs...),
	}
	baselineKey := sdsBaselineKey(tenantID, query.BaselineOptions())
	if strings.TrimSpace(baselineKey) == "" {
		return rejectStudioBatchTaskGate("baseline_missing", "baseline identity is missing")
	}
	cached, ok := g.baselineCache[baselineKey]
	if !ok {
		entry, err := g.baselineChecker.CheckStudioBatchBaselineReadiness(ctx, query)
		cached = studioBatchTaskGateBaselineCacheEntry{entry: entry, err: err}
		g.baselineCache[baselineKey] = cached
	}
	if cached.err != nil {
		return rejectStudioBatchTaskGate("baseline_check_unavailable", cached.err.Error())
	}
	readiness := evaluateSDSBaselineReusableReadiness(cached.entry)
	if readiness.Reusable {
		return studioBatchTaskGateResult{Eligible: true}
	}
	switch readiness.ReasonCode {
	case SDSBaselineReasonCodeCacheUnavailable:
		return rejectStudioBatchTaskGate("baseline_missing", readiness.Reason)
	case SDSBaselineReasonCodeCacheVersionUnsupported:
		return rejectStudioBatchTaskGate("baseline_stale", readiness.Reason)
	case SDSBaselineReasonCodeCachePayloadMissing, SDSBaselineReasonCodeCachePayloadInvalid, SDSBaselineReasonCodeCachePayloadEmpty:
		return rejectStudioBatchTaskGate("baseline_invalid", readiness.Reason)
	default:
		if readiness.Err != nil {
			return rejectStudioBatchTaskGate("baseline_invalid", readiness.Err.Error())
		}
		return rejectStudioBatchTaskGate("baseline_not_ready", firstNonEmpty(readiness.Reason, "SDS baseline is not ready"))
	}
}

func (g *studioBatchTaskGate) compatibilityFingerprint(selectionID string, selection SheinStudioSelection) string {
	cacheKey := strings.TrimSpace(selectionID)
	if cacheKey == "" {
		cacheKey = selectionIDForStudioSelection(selection)
	}
	if cacheKey == "" {
		return buildStudioBatchCompatibilityFingerprint(selection)
	}
	if fingerprint, ok := g.compatCache[cacheKey]; ok {
		return fingerprint
	}
	fingerprint := buildStudioBatchCompatibilityFingerprint(selection)
	g.compatCache[cacheKey] = fingerprint
	return fingerprint
}

func rejectStudioBatchTaskGate(reasonCode string, message string) studioBatchTaskGateResult {
	return studioBatchTaskGateResult{
		Eligible:   false,
		ReasonCode: strings.TrimSpace(reasonCode),
		Message:    strings.TrimSpace(message),
	}
}

func studioBatchTaskItemOwnsSelection(item StudioBatchItemRecord, selectionID string) bool {
	selectionID = strings.TrimSpace(selectionID)
	if selectionID == "" {
		return false
	}
	for _, ownedID := range studioBatchTaskItemSelectionIDs(item) {
		if ownedID == selectionID {
			return true
		}
	}
	return false
}

func studioBatchSelectionVariantsCompatible(selection SheinStudioSelection) bool {
	if len(selection.SelectedVariantIDs) > 0 {
		known := make(map[int64]struct{}, len(selection.Variants)+1)
		if selection.VariantID > 0 {
			known[selection.VariantID] = struct{}{}
		}
		for _, variant := range selection.Variants {
			if variant.VariantID > 0 {
				known[variant.VariantID] = struct{}{}
			}
		}
		for _, selectedID := range selection.SelectedVariantIDs {
			if selectedID <= 0 {
				return false
			}
			if _, ok := known[selectedID]; !ok && len(selection.Variants) > 0 {
				return false
			}
		}
	}
	for _, variant := range selection.Variants {
		if variant.VariantID != selection.VariantID {
			continue
		}
		if variant.PrototypeGroupID > 0 && variant.PrototypeGroupID != selection.PrototypeGroupID {
			return false
		}
		if strings.TrimSpace(variant.LayerID) != "" && strings.TrimSpace(variant.LayerID) != strings.TrimSpace(selection.LayerID) {
			return false
		}
		if strings.TrimSpace(variant.TemplateImageURL) != "" && strings.TrimSpace(selection.TemplateImageURL) != "" && strings.TrimSpace(variant.TemplateImageURL) != strings.TrimSpace(selection.TemplateImageURL) {
			return false
		}
		if strings.TrimSpace(variant.MaskImageURL) != "" && strings.TrimSpace(selection.MaskImageURL) != "" && strings.TrimSpace(variant.MaskImageURL) != strings.TrimSpace(selection.MaskImageURL) {
			return false
		}
	}
	return true
}

func studioBatchTaskGateTenantID(ctx context.Context, batch *StudioBatchRecord) string {
	if batch != nil && strings.TrimSpace(batch.TenantID) != "" {
		return strings.TrimSpace(batch.TenantID)
	}
	return strings.TrimSpace(TenantIDFromContext(ctx))
}

type studioBatchBaselineCacheReadinessChecker struct {
	repo SDSBaselineCacheRepository
}

func (c studioBatchBaselineCacheReadinessChecker) CheckStudioBatchBaselineReadiness(ctx context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineCacheEntry, error) {
	if c.repo == nil {
		return nil, nil
	}
	tenantID := resolveSDSBaselineReadinessTenant(ctx, query.TenantID)
	return c.repo.GetSDSBaselineCache(ctx, tenantID, sdsBaselineKey(tenantID, query.BaselineOptions()))
}

type studioBatchStoreProfileValidator struct {
	repo StoreProfileRepository
}

func (v studioBatchStoreProfileValidator) ValidateStudioBatchStore(ctx context.Context, tenantID string, storeID int64) (studioBatchStoreValidationResult, error) {
	if v.repo == nil {
		return studioBatchStoreValidationResult{}, nil
	}
	tenantNumeric, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		value, err := tenantbridge.ResolveLegacyTenantID(ctx, strings.TrimSpace(tenantID))
		if err == nil && value > 0 {
			tenantNumeric = value
			ok = true
		}
	}
	if !ok || tenantNumeric <= 0 {
		return studioBatchStoreValidationResult{Exists: true, Valid: true, Available: true}, nil
	}
	items, err := v.repo.ListByTenant(ctx, tenantNumeric)
	if err != nil {
		return studioBatchStoreValidationResult{}, err
	}
	for _, profile := range items {
		if profile.StoreID != storeID {
			continue
		}
		return studioBatchStoreValidationResult{
			Exists:    true,
			Valid:     true,
			Available: profile.Enabled,
			Message:   firstNonEmpty(profile.Site, fmt.Sprintf("SHEIN store %d", storeID)),
		}, nil
	}
	return studioBatchStoreValidationResult{Exists: false, Valid: false}, nil
}
