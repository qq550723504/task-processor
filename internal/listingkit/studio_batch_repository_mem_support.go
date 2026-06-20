package listingkit

import (
	"context"
	"time"

	"gorm.io/gorm"
)

func (r *MemStudioBatchRepository) ReplaceStudioBatchGenerationGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord) error {
	if batch == nil {
		return nil
	}

	batchRow, itemRows, _, _, err := prepareStudioBatchGraph(ctx, batch, items, nil, nil)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.batches[batchRow.ID]
	if ok && !matchesStudioBatchScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}

	r.batches[batchRow.ID] = batchRow
	itemIDs := make(map[string]struct{}, len(itemRows))
	for _, row := range itemRows {
		itemIDs[row.ID] = struct{}{}
		r.items[row.ID] = row
	}

	for itemID, item := range r.items {
		if item.BatchID == batchRow.ID {
			if _, keep := itemIDs[itemID]; !keep {
				delete(r.items, itemID)
			}
		}
	}
	for attemptID, attempt := range r.attempts {
		if attempt.BatchID == batchRow.ID {
			delete(r.attempts, attemptID)
		}
	}
	for designID, design := range r.designs {
		if design.BatchID == batchRow.ID {
			delete(r.designs, designID)
		}
	}
	return nil
}

func (r *MemStudioBatchRepository) ReplaceStudioItemMaterializedDesigns(ctx context.Context, itemID string, designs []StudioMaterializedDesignRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[itemID]
	if !ok || !matchesStudioBatchScope(ctx, item.TenantID, item.UserID) {
		return gorm.ErrRecordNotFound
	}

	for designID, row := range r.designs {
		if row.ItemID == itemID && matchesStudioBatchScope(ctx, row.TenantID, row.UserID) {
			delete(r.designs, designID)
		}
	}
	for _, design := range designs {
		row := design
		row.BatchID = item.BatchID
		row.ItemID = item.ID
		row.TenantID = item.TenantID
		row.UserID = item.UserID
		if row.TargetGroupKey == "" {
			row.TargetGroupKey = item.TargetGroupKey
		}
		if row.TargetGroupLabel == "" {
			row.TargetGroupLabel = item.TargetGroupLabel
		}
		if row.ReviewStatus == "" {
			row.ReviewStatus = StudioMaterializedDesignReviewStatusApproved
		}
		r.designs[row.ID] = row
	}
	return nil
}

func (r *MemStudioBatchRepository) ReplaceStudioMaterializedDesignReviews(ctx context.Context, batchID string, designIDs []string, updatedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	batch, ok := r.batches[batchID]
	if !ok || !matchesStudioBatchScope(ctx, batch.TenantID, batch.UserID) {
		return gorm.ErrRecordNotFound
	}

	approvedSet := make(map[string]struct{}, len(designIDs))
	for _, designID := range designIDs {
		approvedSet[designID] = struct{}{}
	}
	for _, designID := range designIDs {
		row, ok := r.designs[designID]
		if !ok || row.BatchID != batchID || !matchesStudioBatchScope(ctx, row.TenantID, row.UserID) {
			return gorm.ErrRecordNotFound
		}
	}

	for designID, row := range r.designs {
		if row.BatchID != batchID || !matchesStudioBatchScope(ctx, row.TenantID, row.UserID) {
			continue
		}
		if _, ok := approvedSet[designID]; ok {
			row.ReviewStatus = StudioMaterializedDesignReviewStatusApproved
		} else {
			row.ReviewStatus = StudioMaterializedDesignReviewStatusUnreviewed
		}
		row.UpdatedAt = updatedAt
		r.designs[designID] = row
	}
	return nil
}

func (r *MemStudioBatchRepository) UpdateStudioBatch(ctx context.Context, batch *StudioBatchRecord) error {
	if batch == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.batches[batch.ID]
	if !ok || !matchesStudioBatchScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	row := *batch
	applyStudioBatchDefaultScope(existing.TenantID, existing.UserID, &row.TenantID, &row.UserID)
	r.batches[row.ID] = row
	return nil
}

func (r *MemStudioBatchRepository) UpdateStudioBatchItem(ctx context.Context, item *StudioBatchItemRecord) error {
	if item == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.items[item.ID]
	if !ok || !matchesStudioBatchScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	row := *item
	if err := resolveStudioBatchItemOwnership(existing, &row); err != nil {
		return err
	}
	r.items[row.ID] = row
	return nil
}

func (r *MemStudioBatchRepository) UpdateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error {
	if attempt == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.attempts[attempt.ID]
	if !ok || !matchesStudioBatchScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	row := *attempt
	if err := resolveStudioGenerationAttemptOwnership(existing, &row); err != nil {
		return err
	}
	if row.Provider == "" {
		row.Provider = existing.Provider
	}
	if row.RequestID == "" {
		row.RequestID = existing.RequestID
	}
	if row.SubmitResponsePayload == "" {
		row.SubmitResponsePayload = existing.SubmitResponsePayload
	}
	if row.ResultCheckedAt == nil {
		row.ResultCheckedAt = existing.ResultCheckedAt
	}
	r.attempts[row.ID] = row
	return nil
}

func (r *MemStudioBatchRepository) UpdateStudioMaterializedDesign(ctx context.Context, design *StudioMaterializedDesignRecord) error {
	if design == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.designs[design.ID]
	if !ok || !matchesStudioBatchScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	row := *design
	if err := resolveStudioMaterializedDesignOwnership(existing, &row); err != nil {
		return err
	}
	if row.ReviewStatus == "" {
		row.ReviewStatus = existing.ReviewStatus
	}
	r.designs[row.ID] = row
	return nil
}

func (r *MemStudioBatchRepository) buildDetailGraphLocked(ctx context.Context, batch StudioBatchRecord) *StudioBatchDetailGraph {
	items := make([]StudioBatchItemRecord, 0)
	for _, item := range r.items {
		if item.BatchID != batch.ID || !matchesStudioBatchScope(ctx, item.TenantID, item.UserID) {
			continue
		}
		items = append(items, item)
	}
	attempts := make([]StudioGenerationAttemptRecord, 0)
	for _, attempt := range r.attempts {
		if !matchesStudioBatchScope(ctx, attempt.TenantID, attempt.UserID) {
			continue
		}
		attempts = append(attempts, attempt)
	}

	designs := make([]StudioMaterializedDesignRecord, 0)
	for _, design := range r.designs {
		if !matchesStudioBatchScope(ctx, design.TenantID, design.UserID) {
			continue
		}
		designs = append(designs, design)
	}

	return buildStudioBatchDetailGraph(&batch, items, attempts, designs)
}
