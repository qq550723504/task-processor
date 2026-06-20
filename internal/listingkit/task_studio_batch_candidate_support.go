package listingkit

import (
	"context"
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
}

type studioBatchTaskCandidate struct {
	Design                   StudioMaterializedDesignRecord
	Item                     StudioBatchItemRecord
	Selection                SheinStudioGroupedSelection
	SelectionSnapshot        SheinStudioSelection
	SelectionID              string
	CompatibilityFingerprint string
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
	candidates, rejectedTasks, err := s.buildStudioBatchTaskCandidates(ctx, session, batchDetail.Batch, batchDetail, designs)
	if err != nil {
		return nil, err
	}
	return &StudioBatchTaskState{
		Session:       session,
		Batch:         batchDetail.Batch,
		DesignIDs:     stateDesignIDs,
		Candidates:    candidates,
		RejectedTasks: rejectedTasks,
	}, nil
}

func (s *taskStudioBatchService) buildStudioBatchTaskCandidates(
	ctx context.Context,
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	detail *StudioBatchDetailGraph,
	designs []StudioMaterializedDesignRecord,
) ([]studioBatchTaskCandidate, []SheinStudioRejectedTask, error) {
	_ = ctx
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

		selections := resolveStudioBatchItemSelections(batch, item)
		if len(selections) == 0 {
			selections = studioBatchAllGroupedSelections(batch)
		}
		if len(selections) == 0 {
			selections = []SheinStudioGroupedSelection{studioBatchTaskFallbackSelection(session, batch, item, design)}
		}

		candidate, rejected, ok := buildStudioBatchTaskCandidate(batch, item, design, selections)
		if rejected != nil {
			rejectedTasks = append(rejectedTasks, *rejected)
		}
		if !ok {
			continue
		}
		candidates = append(candidates, candidate)
	}

	return candidates, rejectedTasks, nil
}

func buildStudioBatchTaskCandidate(
	batch *StudioBatchRecord,
	item StudioBatchItemRecord,
	design StudioMaterializedDesignRecord,
	selections []SheinStudioGroupedSelection,
) (studioBatchTaskCandidate, *SheinStudioRejectedTask, bool) {
	var (
		chosen             studioBatchTaskCandidate
		chosenFingerprint  string
		compatibilityError *SheinStudioRejectedTask
	)

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
		if index == 0 {
			chosen = candidate
			chosenFingerprint = fingerprint
			continue
		}
		if fingerprint != chosenFingerprint {
			compatibilityError = &SheinStudioRejectedTask{
				DesignID:    design.ID,
				ItemID:      item.ID,
				SelectionID: selectionID,
				ReasonCode:  "compatibility_mismatch",
				Message:     fmt.Sprintf("design %s has incompatible grouped selections for batch task creation", strings.TrimSpace(design.ID)),
			}
			break
		}
	}

	if compatibilityError != nil {
		return studioBatchTaskCandidate{}, compatibilityError, false
	}
	return chosen, nil, true
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
