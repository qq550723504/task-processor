package listingkit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

type StudioBatchTaskState struct {
	Session       *SheinStudioSession
	Batch         *StudioBatchRecord
	DesignIDs     []string
	Candidates    []studioBatchTaskCandidate
	RejectedTasks []SheinStudioRejectedTask
	FailedTasks   []SheinStudioFailedTask
}

type studioBatchTaskCandidate struct {
	Design                   StudioMaterializedDesignRecord
	Item                     StudioBatchItemRecord
	Selection                SheinStudioGroupedSelection
	SelectionSnapshot        SheinStudioSelection
	SelectionID              string
	CompatibilityFingerprint string
	CandidateKey             string
	SheinStoreID             int64
	StyleID                  string
	Title                    string
}

type studioBatchTaskStateContextKey struct{}

func withStudioBatchTaskState(ctx context.Context, batchID string, state *StudioBatchTaskState) context.Context {
	if state == nil {
		return ctx
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	if normalizedBatchID == "" {
		return ctx
	}
	states := map[string]*StudioBatchTaskState{normalizedBatchID: state}
	if existing, ok := ctx.Value(studioBatchTaskStateContextKey{}).(map[string]*StudioBatchTaskState); ok && len(existing) > 0 {
		states = make(map[string]*StudioBatchTaskState, len(existing)+1)
		for key, value := range existing {
			states[key] = value
		}
		states[normalizedBatchID] = state
	}
	return context.WithValue(ctx, studioBatchTaskStateContextKey{}, states)
}

func loadStudioBatchTaskStateFromContext(ctx context.Context, batchID string) (*StudioBatchTaskState, bool) {
	states, ok := ctx.Value(studioBatchTaskStateContextKey{}).(map[string]*StudioBatchTaskState)
	if !ok {
		return nil, false
	}
	state, ok := states[strings.TrimSpace(batchID)]
	if !ok || state == nil {
		return nil, false
	}
	return state, true
}

func (s *taskStudioBatchService) buildStudioBatchTaskState(
	ctx context.Context,
	batchID string,
	designIDs []string,
) (*StudioBatchTaskState, error) {
	stateDesignIDs, session, batchDetail, err := s.prepareStudioBatchTaskCreation(ctx, batchID, &CreateStudioBatchTasksRequest{
		DesignIDs: append([]string(nil), designIDs...),
	})
	if err != nil {
		return nil, err
	}
	designs, err := s.repo.ListStudioMaterializedDesignsByIDs(ctx, batchID, stateDesignIDs)
	if err != nil {
		return nil, err
	}
	designs = orderStudioBatchTaskDesignsByRequest(designs, stateDesignIDs)
	candidates, rejectedTasks, err := s.buildStudioBatchTaskCandidates(ctx, session, batchDetail.Batch, batchDetail, designs)
	if err != nil {
		return nil, err
	}
	candidates, gateRejectedTasks, failedTasks := s.evaluateStudioBatchTaskCandidates(ctx, batchDetail.Batch, batchDetail, designs, candidates)
	rejectedTasks = append(rejectedTasks, gateRejectedTasks...)
	return &StudioBatchTaskState{
		Session:       session,
		Batch:         batchDetail.Batch,
		DesignIDs:     stateDesignIDs,
		Candidates:    candidates,
		RejectedTasks: rejectedTasks,
		FailedTasks:   failedTasks,
	}, nil
}

func (s *taskStudioBatchService) evaluateStudioBatchTaskCandidates(
	ctx context.Context,
	batch *StudioBatchRecord,
	detail *StudioBatchDetailGraph,
	designs []StudioMaterializedDesignRecord,
	candidates []studioBatchTaskCandidate,
) ([]studioBatchTaskCandidate, []SheinStudioRejectedTask, []SheinStudioFailedTask) {
	if len(candidates) == 0 {
		return candidates, nil, nil
	}
	gate := newStudioBatchTaskGate(s.baselineChecker, s.storeValidator)
	designsByID := make(map[string]StudioMaterializedDesignRecord, len(designs))
	for _, design := range designs {
		designsByID[strings.TrimSpace(design.ID)] = design
	}
	selectionsByID := studioBatchSelectionSnapshotMap(batch)
	itemSelectionsByID := make(map[string][]SheinStudioGroupedSelection)
	if detail != nil {
		itemSelectionsByID = make(map[string][]SheinStudioGroupedSelection, len(detail.Items))
		for _, item := range detail.Items {
			selections, _, _ := resolveStudioBatchTaskCandidateSelections(batch, item)
			itemSelectionsByID[strings.TrimSpace(item.ID)] = selections
		}
	}
	eligible := make([]studioBatchTaskCandidate, 0, len(candidates))
	rejected := make([]SheinStudioRejectedTask, 0)
	failed := make([]SheinStudioFailedTask, 0)
	for _, candidate := range candidates {
		result, err := gate.Evaluate(ctx, &studioBatchTaskGateEvaluation{
			Batch:          batch,
			Candidate:      candidate,
			DesignsByID:    designsByID,
			SelectionsByID: selectionsByID,
			ItemSelections: itemSelectionsByID[strings.TrimSpace(candidate.Item.ID)],
		})
		if err != nil {
			failed = append(failed, SheinStudioFailedTask{
				DesignID: strings.TrimSpace(candidate.Design.ID),
				Title:    strings.TrimSpace(candidate.Title),
				Message:  err.Error(),
			})
			continue
		}
		if !result.Eligible {
			rejected = append(rejected, SheinStudioRejectedTask{
				DesignID:    strings.TrimSpace(candidate.Design.ID),
				ItemID:      strings.TrimSpace(candidate.Item.ID),
				SelectionID: strings.TrimSpace(candidate.SelectionID),
				ReasonCode:  strings.TrimSpace(result.ReasonCode),
				Message:     strings.TrimSpace(result.Message),
			})
			continue
		}
		eligible = append(eligible, candidate)
	}
	return eligible, rejected, failed
}

func (s *taskStudioBatchService) buildStudioBatchTaskCandidates(
	ctx context.Context,
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	detail *StudioBatchDetailGraph,
	designs []StudioMaterializedDesignRecord,
) ([]studioBatchTaskCandidate, []SheinStudioRejectedTask, error) {
	if batch == nil || detail == nil {
		return nil, nil, gorm.ErrRecordNotFound
	}

	itemByID := make(map[string]StudioBatchItemRecord, len(detail.Items))
	for _, item := range detail.Items {
		itemByID[item.ID] = item
	}

	candidates := make([]studioBatchTaskCandidate, 0, len(designs))
	rejectedTasks := make([]SheinStudioRejectedTask, 0)
	for _, design := range designs {
		item, ok := itemByID[design.ItemID]
		if !ok {
			return nil, nil, gorm.ErrRecordNotFound
		}

		selections, missingSelectionIDs, hasExplicitOwnership := resolveStudioBatchTaskCandidateSelections(batch, item)
		if len(selections) == 0 && !hasExplicitOwnership {
			selections = studioBatchAllGroupedSelections(batch)
		}
		if len(selections) == 0 && !hasExplicitOwnership {
			selections = []SheinStudioGroupedSelection{studioBatchTaskFallbackSelection(session, batch, item, design)}
		}

		rejectedTasks = append(rejectedTasks, buildStudioBatchMissingSelectionRejections(design, item, missingSelectionIDs)...)
		if len(selections) == 0 && len(missingSelectionIDs) > 0 {
			continue
		}

		designCandidates, rejected := buildStudioBatchTaskCandidatesForDesign(ctx, session, batch, item, design, selections)
		if rejected != nil {
			rejectedTasks = append(rejectedTasks, *rejected)
		}
		candidates = append(candidates, designCandidates...)
	}

	return candidates, rejectedTasks, nil
}

func buildStudioBatchTaskCandidatesForDesign(
	ctx context.Context,
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	item StudioBatchItemRecord,
	design StudioMaterializedDesignRecord,
	selections []SheinStudioGroupedSelection,
) ([]studioBatchTaskCandidate, *SheinStudioRejectedTask) {
	if len(selections) == 0 {
		return nil, buildStudioBatchTaskCandidateRejection(
			design,
			item,
			"",
			"selection_not_in_batch",
			fmt.Sprintf("design %s has no resolved selections for batch task creation", strings.TrimSpace(design.ID)),
		)
	}

	groupMode := studioBatchTaskCandidateGroupMode(batch, item)
	if groupMode == "per_product" && len(selections) != 1 {
		return nil, buildStudioBatchTaskCandidateRejection(
			design,
			item,
			strings.Join(studioBatchTaskCandidateSelectionIDs(selections), ","),
			"selection_cardinality_mismatch",
			fmt.Sprintf("per-product item %s resolved %d selections; exactly one is required", strings.TrimSpace(item.ID), len(selections)),
		)
	}

	candidates := make([]studioBatchTaskCandidate, 0, len(selections))
	var expectedFingerprint string

	for index, grouped := range selections {
		snapshot := grouped.Selection
		selectionID := firstNonEmpty(
			strings.TrimSpace(grouped.SelectionID),
			selectionIDForStudioSelection(snapshot),
			strings.TrimSpace(item.TargetGroupKey),
			strings.TrimSpace(design.TargetGroupKey),
			strings.TrimSpace(design.ID),
		)
		fingerprint := buildStudioBatchCompatibilityFingerprint(snapshot)
		candidate := studioBatchTaskCandidate{
			Design:                   design,
			Item:                     item,
			Selection:                grouped,
			SelectionSnapshot:        snapshot,
			SelectionID:              selectionID,
			CompatibilityFingerprint: fingerprint,
			StyleID:                  buildStudioBatchTaskScopedStyleID(batch.ID, item.ID, design.ID, selectionID),
			Title: firstNonEmpty(
				strings.TrimSpace(grouped.Selection.VariantLabel),
				strings.TrimSpace(grouped.Selection.ProductName),
				strings.TrimSpace(item.TargetGroupLabel),
				strings.TrimSpace(design.TargetGroupLabel),
				strings.TrimSpace(design.ID),
			),
		}
		if groupMode != "per_product" && len(selections) > 1 {
			if !studioBatchCompatibilityFingerprintComplete(snapshot) {
				return nil, buildStudioBatchTaskCandidateRejection(
					design,
					item,
					selectionID,
					"compatibility_incomplete",
					fmt.Sprintf("design %s has an incomplete compatibility fingerprint for batch task creation", strings.TrimSpace(design.ID)),
				)
			}
			if index == 0 {
				expectedFingerprint = fingerprint
			} else if fingerprint != expectedFingerprint {
				return nil, buildStudioBatchTaskCandidateRejection(
					design,
					item,
					selectionID,
					"compatibility_mismatch",
					fmt.Sprintf("design %s has incompatible grouped selections for batch task creation", strings.TrimSpace(design.ID)),
				)
			}
		}
		candidate.SheinStoreID = studioBatchTaskStoreID(session, batch, grouped.SheinStoreID)
		candidate.CandidateKey = buildStudioBatchTaskCandidateKey(ctx, batch, candidate)
		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

func buildStudioBatchTaskCandidateKey(ctx context.Context, batch *StudioBatchRecord, candidate studioBatchTaskCandidate) string {
	tenantID := ""
	if batch != nil {
		tenantID = strings.TrimSpace(batch.TenantID)
	}
	if tenantID == "" {
		tenantID = strings.TrimSpace(TenantIDFromContext(ctx))
	}
	batchID := ""
	if batch != nil {
		batchID = strings.TrimSpace(batch.ID)
	}
	storeID := candidate.SheinStoreID
	if storeID <= 0 {
		storeID = studioBatchTaskStoreID(nil, batch, candidate.Selection.SheinStoreID)
	}
	normalized := strings.Join([]string{
		tenantID,
		batchID,
		strings.TrimSpace(candidate.Item.ID),
		strings.TrimSpace(candidate.Design.ID),
		strings.TrimSpace(candidate.SelectionID),
		strconv.FormatInt(storeID, 10),
	}, "|")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func orderStudioBatchTaskDesignsByRequest(
	designs []StudioMaterializedDesignRecord,
	designIDs []string,
) []StudioMaterializedDesignRecord {
	if len(designs) == 0 || len(designIDs) == 0 {
		return designs
	}
	byID := make(map[string]StudioMaterializedDesignRecord, len(designs))
	for _, design := range designs {
		byID[strings.TrimSpace(design.ID)] = design
	}
	ordered := make([]StudioMaterializedDesignRecord, 0, len(designs))
	seen := make(map[string]struct{}, len(designs))
	for _, rawID := range designIDs {
		designID := strings.TrimSpace(rawID)
		design, ok := byID[designID]
		if !ok {
			continue
		}
		seen[designID] = struct{}{}
		ordered = append(ordered, design)
	}
	for _, design := range designs {
		designID := strings.TrimSpace(design.ID)
		if _, ok := seen[designID]; ok {
			continue
		}
		ordered = append(ordered, design)
	}
	return ordered
}

func buildStudioBatchMissingSelectionRejections(
	design StudioMaterializedDesignRecord,
	item StudioBatchItemRecord,
	missingSelectionIDs []string,
) []SheinStudioRejectedTask {
	rejections := make([]SheinStudioRejectedTask, 0, len(missingSelectionIDs))
	for _, selectionID := range missingSelectionIDs {
		rejections = append(rejections, *buildStudioBatchTaskCandidateRejection(
			design,
			item,
			selectionID,
			"selection_not_in_batch",
			fmt.Sprintf("design %s references selection %s that is not in the batch snapshot", strings.TrimSpace(design.ID), strings.TrimSpace(selectionID)),
		))
	}
	return rejections
}

func buildStudioBatchTaskCandidateRejection(
	design StudioMaterializedDesignRecord,
	item StudioBatchItemRecord,
	selectionID string,
	reasonCode string,
	message string,
) *SheinStudioRejectedTask {
	return &SheinStudioRejectedTask{
		DesignID:    strings.TrimSpace(design.ID),
		ItemID:      strings.TrimSpace(item.ID),
		SelectionID: strings.TrimSpace(selectionID),
		ReasonCode:  strings.TrimSpace(reasonCode),
		Message:     strings.TrimSpace(message),
	}
}

func resolveStudioBatchTaskCandidateSelections(
	batch *StudioBatchRecord,
	item StudioBatchItemRecord,
) ([]SheinStudioGroupedSelection, []string, bool) {
	selectionIDs := studioBatchTaskItemSelectionIDs(item)
	if len(selectionIDs) == 0 {
		return nil, nil, false
	}
	selectionMap := studioBatchSelectionSnapshotMap(batch)
	selections := make([]SheinStudioGroupedSelection, 0, len(selectionIDs))
	missing := make([]string, 0)
	for _, selectionID := range selectionIDs {
		grouped, ok := selectionMap[selectionID]
		if !ok {
			missing = append(missing, selectionID)
			continue
		}
		selections = append(selections, grouped)
	}
	return selections, missing, true
}

func studioBatchTaskItemSelectionIDs(item StudioBatchItemRecord) []string {
	result := make([]string, 0, len(item.SelectionIDs))
	for _, raw := range item.SelectionIDs {
		selectionID := strings.TrimSpace(raw)
		if selectionID == "" {
			continue
		}
		result = append(result, selectionID)
	}
	return result
}

func studioBatchTaskCandidateSelectionIDs(selections []SheinStudioGroupedSelection) []string {
	result := make([]string, 0, len(selections))
	for _, grouped := range selections {
		selectionID := strings.TrimSpace(grouped.SelectionID)
		if selectionID == "" {
			selectionID = selectionIDForStudioSelection(grouped.Selection)
		}
		if selectionID == "" {
			continue
		}
		result = append(result, selectionID)
	}
	return result
}

func studioBatchTaskCandidateGroupMode(batch *StudioBatchRecord, item StudioBatchItemRecord) string {
	if mode := strings.TrimSpace(item.GroupMode); mode != "" {
		return mode
	}
	if batch == nil {
		return ""
	}
	return strings.TrimSpace(batch.GroupedImageMode)
}

func studioBatchCompatibilityFingerprintComplete(selection SheinStudioSelection) bool {
	return selection.ParentProductID > 0 &&
		selection.PrototypeGroupID > 0 &&
		strings.TrimSpace(selection.LayerID) != "" &&
		strings.TrimSpace(selection.DesignType) != "" &&
		selection.PrintableWidth > 0 &&
		selection.PrintableHeight > 0 &&
		strings.TrimSpace(selection.TemplateImageURL) != "" &&
		strings.TrimSpace(selection.MaskImageURL) != ""
}

func studioBatchTaskFallbackSelection(
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	item StudioBatchItemRecord,
	design StudioMaterializedDesignRecord,
) SheinStudioGroupedSelection {
	selection := SheinStudioSelection(batch.Selection)
	if session != nil {
		selection = SheinStudioSelection(session.Selection)
	}
	grouped := SheinStudioGroupedSelection{
		SelectionID: firstNonEmpty(
			strings.TrimSpace(item.TargetGroupKey),
			strings.TrimSpace(design.TargetGroupKey),
			strings.TrimSpace(design.ID),
		),
		Selection: selection,
		Eligible:  true,
	}
	if session != nil && strings.TrimSpace(session.SheinStoreID) != "" {
		grouped.SheinStoreID = strings.TrimSpace(session.SheinStoreID)
	} else if batch != nil && batch.SheinStoreID > 0 {
		grouped.SheinStoreID = strconv.FormatInt(batch.SheinStoreID, 10)
	}
	return grouped
}
