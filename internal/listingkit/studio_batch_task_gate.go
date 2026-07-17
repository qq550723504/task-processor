package listingkit

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/listingkit/studiobatch"
	"task-processor/internal/tenantbridge"
)

type StudioBatchBaselineReadinessChecker interface {
	CheckStudioBatchBaselineReadiness(ctx context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error)
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
}

type studioBatchTaskGateBaselineCacheEntry struct {
	readiness *SDSBaselineReadiness
	err       error
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
	if result := studioBatchGateResult(studiobatch.EvaluateGate(studioBatchGateInput(eval))); !result.Eligible {
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
		readiness, err := g.baselineChecker.CheckStudioBatchBaselineReadiness(ctx, query)
		cached = studioBatchTaskGateBaselineCacheEntry{readiness: readiness, err: err}
		g.baselineCache[baselineKey] = cached
	}
	if cached.err != nil {
		return rejectStudioBatchTaskGate("baseline_check_unavailable", cached.err.Error())
	}
	readiness := cached.readiness
	if readiness != nil && readiness.Status == SDSBaselineStatusReady &&
		readiness.ValidationStatus == SDSBaselineValidationStatusReady {
		return studioBatchTaskGateResult{Eligible: true}
	}
	if readiness == nil {
		return rejectStudioBatchTaskGate("baseline_missing", "SDS baseline is not ready")
	}
	switch readiness.ReasonCode {
	case SDSBaselineReasonCodeCacheUnavailable:
		return rejectStudioBatchTaskGate("baseline_missing", readiness.Reason)
	case SDSBaselineReasonCodeCacheVersionUnsupported:
		return rejectStudioBatchTaskGate("baseline_stale", readiness.Reason)
	case SDSBaselineReasonCodeCachePayloadMissing, SDSBaselineReasonCodeCachePayloadInvalid, SDSBaselineReasonCodeCachePayloadEmpty:
		return rejectStudioBatchTaskGate("baseline_invalid", readiness.Reason)
	default:
		return rejectStudioBatchTaskGate("baseline_not_ready", firstNonEmpty(readiness.Reason, "SDS baseline is not ready"))
	}
}

func studioBatchGateInput(eval *studioBatchTaskGateEvaluation) studiobatch.GateInput {
	if eval == nil {
		return studiobatch.GateInput{}
	}
	input := studiobatch.GateInput{
		Candidate: studiobatch.Candidate{
			Design:                   studioBatchGateDesign(eval.Candidate.Design),
			Item:                     studioBatchGateItem(eval.Candidate.Item),
			Selection:                studioBatchGateGroupedSelection(eval.Candidate.Selection),
			SelectionSnapshot:        studioBatchGateSelection(eval.Candidate.SelectionSnapshot),
			SelectionID:              eval.Candidate.SelectionID,
			CompatibilityFingerprint: eval.Candidate.CompatibilityFingerprint,
			CandidateKey:             eval.Candidate.CandidateKey,
			StoreID:                  eval.Candidate.SheinStoreID,
			StyleID:                  eval.Candidate.StyleID,
			Title:                    eval.Candidate.Title,
		},
		SelectionByID:  make(map[string]studiobatch.GroupedSelection, len(eval.SelectionsByID)),
		ItemSelections: make([]studiobatch.GroupedSelection, 0, len(eval.ItemSelections)),
	}
	if eval.Batch != nil {
		input.BatchID = eval.Batch.ID
		input.BatchGroupMode = eval.Batch.GroupedImageMode
	}
	input.Designs = make([]studiobatch.Design, 0, len(eval.DesignsByID))
	for _, design := range eval.DesignsByID {
		input.Designs = append(input.Designs, studioBatchGateDesign(design))
	}
	for id, selection := range eval.SelectionsByID {
		input.SelectionByID[id] = studioBatchGateGroupedSelection(selection)
	}
	for _, selection := range eval.ItemSelections {
		input.ItemSelections = append(input.ItemSelections, studioBatchGateGroupedSelection(selection))
	}
	return input
}

func studioBatchGateDesign(design StudioMaterializedDesignRecord) studiobatch.Design {
	return studiobatch.Design{
		ID:               design.ID,
		BatchID:          design.BatchID,
		ItemID:           design.ItemID,
		TargetGroupKey:   design.TargetGroupKey,
		TargetGroupLabel: design.TargetGroupLabel,
		Approved:         design.ReviewStatus == StudioMaterializedDesignReviewStatusApproved,
		ImageURL:         design.ImageURL,
	}
}

func studioBatchGateItem(item StudioBatchItemRecord) studiobatch.Item {
	return studiobatch.Item{
		ID:               item.ID,
		TargetGroupKey:   item.TargetGroupKey,
		TargetGroupLabel: item.TargetGroupLabel,
		GroupMode:        item.GroupMode,
		SelectionIDs:     append([]string(nil), item.SelectionIDs...),
	}
}

func studioBatchGateGroupedSelection(grouped SheinStudioGroupedSelection) studiobatch.GroupedSelection {
	storeID, _ := strconv.ParseInt(strings.TrimSpace(grouped.SheinStoreID), 10, 64)
	return studiobatch.GroupedSelection{
		SelectionID: grouped.SelectionID,
		StoreID:     storeID,
		Selection:   studioBatchGateSelection(grouped.Selection),
	}
}

func studioBatchGateSelection(selection SheinStudioSelection) studiobatch.Selection {
	variants := make([]studiobatch.VariantSurface, 0, len(selection.Variants))
	for _, variant := range selection.Variants {
		variants = append(variants, studiobatch.VariantSurface{
			VariantID:        variant.VariantID,
			PrototypeGroupID: variant.PrototypeGroupID,
			LayerID:          variant.LayerID,
			TemplateImageURL: variant.TemplateImageURL,
			MaskImageURL:     variant.MaskImageURL,
		})
	}
	return studiobatch.Selection{
		ProductID:          selection.ProductID,
		VariantID:          selection.VariantID,
		ParentProductID:    selection.ParentProductID,
		PrototypeGroupID:   selection.PrototypeGroupID,
		LayerID:            selection.LayerID,
		DesignType:         selection.DesignType,
		PrintableWidth:     selection.PrintableWidth,
		PrintableHeight:    selection.PrintableHeight,
		TemplateImageURL:   selection.TemplateImageURL,
		MaskImageURL:       selection.MaskImageURL,
		ProductSize:        selection.ProductSize,
		PackagingSpec:      selection.PackagingSpecification,
		VariantLabel:       selection.VariantLabel,
		ProductName:        selection.ProductName,
		SelectedVariantIDs: append([]int64(nil), selection.SelectedVariantIDs...),
		Variants:           variants,
	}
}

func studioBatchGateResult(result studiobatch.GateResult) studioBatchTaskGateResult {
	return studioBatchTaskGateResult{
		Eligible:   result.Eligible,
		ReasonCode: result.ReasonCode,
		Message:    result.Message,
	}
}

func rejectStudioBatchTaskGate(reasonCode string, message string) studioBatchTaskGateResult {
	return studioBatchTaskGateResult{
		Eligible:   false,
		ReasonCode: strings.TrimSpace(reasonCode),
		Message:    strings.TrimSpace(message),
	}
}
func studioBatchTaskGateTenantID(ctx context.Context, batch *StudioBatchRecord) string {
	if batch != nil && strings.TrimSpace(batch.TenantID) != "" {
		return strings.TrimSpace(batch.TenantID)
	}
	return strings.TrimSpace(TenantIDFromContext(ctx))
}

type studioBatchBaselineCacheReadinessChecker struct {
	readinessService sdsBaselineReadinessService
}

func (c studioBatchBaselineCacheReadinessChecker) CheckStudioBatchBaselineReadiness(ctx context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {
	if c.readinessService == nil {
		return nil, nil
	}
	return c.readinessService.GetReadiness(ctx, query)
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
