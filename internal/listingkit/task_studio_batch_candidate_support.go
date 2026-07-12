package listingkit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	studiobatch "task-processor/internal/listingkit/studiobatch"
	sdstemplate "task-processor/internal/sds/template"

	"gorm.io/gorm"
)

type StudioBatchTaskState struct {
	Session              *SheinStudioSession
	Batch                *StudioBatchRecord
	DesignIDs            []string
	AllApprovedDesignIDs []string
	Candidates           []studioBatchTaskCandidate
	RejectedTasks        []SheinStudioRejectedTask
	FailedTasks          []SheinStudioFailedTask
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
		Session:              session,
		Batch:                batchDetail.Batch,
		DesignIDs:            stateDesignIDs,
		AllApprovedDesignIDs: allApprovedStudioBatchDesignIDs(batchDetail),
		Candidates:           candidates,
		RejectedTasks:        rejectedTasks,
		FailedTasks:          failedTasks,
	}, nil
}

func allApprovedStudioBatchDesignIDs(detail *StudioBatchDetailGraph) []string {
	if detail == nil || len(detail.DesignsByItem) == 0 {
		return nil
	}
	designIDs := make([]string, 0)
	for _, item := range detail.Items {
		for _, design := range detail.DesignsByItem[strings.TrimSpace(item.ID)] {
			if design.ReviewStatus != StudioMaterializedDesignReviewStatusApproved {
				continue
			}
			designID := strings.TrimSpace(design.ID)
			if designID == "" {
				continue
			}
			designIDs = append(designIDs, designID)
		}
	}
	return normalizeStudioBatchDesignIDs(designIDs)
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
	itemSelectionsByID := studioBatchTaskCandidateSelectionsByItem(candidates)
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
				Source:   studioBatchTaskLinkSourceBatchCreated,
				Message:  err.Error(),
			})
			continue
		}
		if !result.Eligible {
			_ = s.persistStudioBatchTaskLink(
				ctx,
				candidate,
				"",
				studioBatchTaskLinkStatusFailed,
				studioBatchTaskLinkSourceRejected,
				result.ReasonCode,
				result.Message,
			)
			rejected = append(rejected, SheinStudioRejectedTask{
				DesignID:    strings.TrimSpace(candidate.Design.ID),
				ItemID:      strings.TrimSpace(candidate.Item.ID),
				SelectionID: strings.TrimSpace(candidate.SelectionID),
				Source:      studioBatchTaskLinkSourceRejected,
				ReasonCode:  strings.TrimSpace(result.ReasonCode),
				Message:     strings.TrimSpace(result.Message),
			})
			continue
		}
		eligible = append(eligible, candidate)
	}
	return eligible, rejected, failed
}

func studioBatchTaskCandidateSelectionsByItem(candidates []studioBatchTaskCandidate) map[string][]SheinStudioGroupedSelection {
	selectionsByItemID := make(map[string][]SheinStudioGroupedSelection)
	seenByItemID := make(map[string]map[string]struct{})
	for _, candidate := range candidates {
		itemID := strings.TrimSpace(candidate.Item.ID)
		if itemID == "" {
			continue
		}
		selectionID := strings.TrimSpace(candidate.SelectionID)
		if selectionID == "" {
			selectionID = selectionIDForStudioSelection(candidate.SelectionSnapshot)
		}
		if seenByItemID[itemID] == nil {
			seenByItemID[itemID] = make(map[string]struct{})
		}
		if _, seen := seenByItemID[itemID][selectionID]; seen {
			continue
		}
		seenByItemID[itemID][selectionID] = struct{}{}
		selectionsByItemID[itemID] = append(selectionsByItemID[itemID], candidate.Selection)
	}
	return selectionsByItemID
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
		var hydrateErr error
		selections, hydrateErr = s.hydrateStudioBatchTaskSelections(ctx, selections)
		if hydrateErr != nil {
			return nil, nil, hydrateErr
		}

		designCandidates, rejected := buildStudioBatchTaskCandidatesForDesign(ctx, session, batch, item, design, selections)
		if rejected != nil {
			rejectedTasks = append(rejectedTasks, *rejected)
		}
		candidates = append(candidates, designCandidates...)
	}

	return candidates, rejectedTasks, nil
}

func (s *taskStudioBatchService) hydrateStudioBatchTaskSelections(
	ctx context.Context,
	selections []SheinStudioGroupedSelection,
) ([]SheinStudioGroupedSelection, error) {
	if s == nil || s.sdsProductDetailProvider == nil || len(selections) == 0 {
		return selections, nil
	}
	hydrated := append([]SheinStudioGroupedSelection(nil), selections...)
	detailsByParentID := make(map[int64]*sdstemplate.ProductDetail)
	for i, grouped := range hydrated {
		selection := grouped.Selection
		if !studioBatchSelectionNeedsProductTableHydration(selection) {
			continue
		}
		if selection.ParentProductID <= 0 {
			continue
		}
		detail, ok := detailsByParentID[selection.ParentProductID]
		if !ok {
			var err error
			detail, err = s.sdsProductDetailProvider.GetProductDetail(ctx, selection.ParentProductID)
			if err != nil {
				return nil, fmt.Errorf("hydrate SDS product detail %d: %w", selection.ParentProductID, err)
			}
			detailsByParentID[selection.ParentProductID] = detail
		}
		grouped.Selection = hydrateStudioBatchSelectionProductTables(selection, detail)
		hydrated[i] = grouped
	}
	return hydrated, nil
}

func studioBatchSelectionNeedsProductTableHydration(selection SheinStudioSelection) bool {
	return strings.TrimSpace(selection.ProductSize) == "" || strings.TrimSpace(selection.PackagingSpecification) == ""
}

func hydrateStudioBatchSelectionProductTables(selection SheinStudioSelection, detail *sdstemplate.ProductDetail) SheinStudioSelection {
	if detail == nil {
		return selection
	}
	productDetails := detail.ProductDetails
	if strings.TrimSpace(selection.ProductSize) == "" {
		selection.ProductSize = strings.TrimSpace(productDetails.ProductSize)
	}
	if strings.TrimSpace(selection.PackagingSpecification) == "" {
		selection.PackagingSpecification = strings.TrimSpace(productDetails.PackagingSpecification)
	}
	return selection
}

func buildStudioBatchTaskCandidatesForDesign(
	ctx context.Context,
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	item StudioBatchItemRecord,
	design StudioMaterializedDesignRecord,
	selections []SheinStudioGroupedSelection,
) ([]studioBatchTaskCandidate, *SheinStudioRejectedTask) {
	result := studiobatch.Evaluate(studioBatchCandidateEvaluationInput(ctx, session, batch, item, design, selections))
	if len(result.Rejections) > 0 {
		rejection := result.Rejections[0]
		return nil, &SheinStudioRejectedTask{
			DesignID:    rejection.DesignID,
			ItemID:      rejection.ItemID,
			SelectionID: rejection.SelectionID,
			ReasonCode:  rejection.ReasonCode,
			Message:     rejection.Message,
		}
	}
	candidates := make([]studioBatchTaskCandidate, 0, len(result.Candidates))
	for index, candidate := range result.Candidates {
		if index >= len(selections) {
			return nil, buildStudioBatchTaskCandidateRejection(design, item, "", "candidate_projection_invalid", "candidate projection did not preserve selection order")
		}
		grouped := selections[index]
		grouped.Selection.DesignType = candidate.SelectionSnapshot.DesignType
		candidates = append(candidates, studioBatchTaskCandidate{
			Design:                   design,
			Item:                     item,
			Selection:                grouped,
			SelectionSnapshot:        grouped.Selection,
			SelectionID:              candidate.SelectionID,
			CompatibilityFingerprint: candidate.CompatibilityFingerprint,
			CandidateKey:             candidate.CandidateKey,
			SheinStoreID:             candidate.StoreID,
			StyleID:                  candidate.StyleID,
			Title:                    candidate.Title,
		})
	}
	return candidates, nil
}

func studioBatchCandidateEvaluationInput(
	ctx context.Context,
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	item StudioBatchItemRecord,
	design StudioMaterializedDesignRecord,
	selections []SheinStudioGroupedSelection,
) studiobatch.EvaluationInput {
	input := studiobatch.EvaluationInput{
		Item:                       studiobatch.Item{ID: item.ID, TargetGroupKey: item.TargetGroupKey, TargetGroupLabel: item.TargetGroupLabel, GroupMode: item.GroupMode},
		Design:                     studiobatch.Design{ID: design.ID, TargetGroupKey: design.TargetGroupKey, TargetGroupLabel: design.TargetGroupLabel},
		ResolvedSelections:         make([]studiobatch.GroupedSelection, 0, len(selections)),
		ExplicitSelectionOwnership: true,
	}
	if batch != nil {
		input.TenantID = strings.TrimSpace(batch.TenantID)
		input.BatchID = batch.ID
		input.BatchGroupMode = batch.GroupedImageMode
		input.BatchStoreID = studioBatchTaskStoreID(session, batch, "")
		input.BatchSelection = studioBatchCandidateSelection(SheinStudioSelection(batch.Selection))
	}
	if input.TenantID == "" {
		input.TenantID = TenantIDFromContext(ctx)
	}
	for _, grouped := range selections {
		input.ResolvedSelections = append(input.ResolvedSelections, studiobatch.GroupedSelection{
			SelectionID: grouped.SelectionID,
			StoreID:     parseStudioBatchTaskStoreID(grouped.SheinStoreID),
			Selection:   studioBatchCandidateSelection(grouped.Selection),
		})
	}
	return input
}

func studioBatchCandidateSelection(selection SheinStudioSelection) studiobatch.Selection {
	selectedVariantIDs := append([]int64(nil), selection.SelectedVariantIDs...)
	if len(selectedVariantIDs) == 0 {
		for _, variant := range selection.Variants {
			if variant.VariantID > 0 {
				selectedVariantIDs = append(selectedVariantIDs, variant.VariantID)
			}
		}
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
		SelectedVariantIDs: selectedVariantIDs,
	}
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
		studioBatchTaskCandidateCompatibilityFingerprint(candidate),
		strconv.FormatInt(storeID, 10),
	}, "|")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func studioBatchTaskCandidateCompatibilityFingerprint(candidate studioBatchTaskCandidate) string {
	if value := strings.TrimSpace(candidate.CompatibilityFingerprint); value != "" {
		return value
	}
	return buildStudioBatchCompatibilityFingerprint(candidate.SelectionSnapshot)
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

func studioBatchTaskCompatibilityFingerprintComplete(selection SheinStudioSelection) bool {
	return selection.ParentProductID > 0 &&
		selection.PrototypeGroupID > 0 &&
		strings.TrimSpace(selection.LayerID) != "" &&
		strings.TrimSpace(selection.DesignType) != "" &&
		selection.PrintableWidth > 0 &&
		selection.PrintableHeight > 0 &&
		strings.TrimSpace(selection.TemplateImageURL) != ""
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
