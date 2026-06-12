package listingkit

import (
	"context"

	"task-processor/internal/listingkit/tenantctx"

	"gorm.io/gorm"
)

func prepareStudioBatchGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord, designs []StudioMaterializedDesignRecord) (StudioBatchRecord, []StudioBatchItemRecord, []StudioGenerationAttemptRecord, []StudioMaterializedDesignRecord, error) {
	batchRow := *batch
	applyStudioBatchScopeDefaults(ctx, &batchRow.TenantID, &batchRow.UserID)

	itemRows := make([]StudioBatchItemRecord, 0, len(items))
	itemIDs := make(map[string]struct{}, len(items))
	for _, item := range items {
		row := item
		row.BatchID = batchRow.ID
		row.TenantID = batchRow.TenantID
		row.UserID = batchRow.UserID
		itemRows = append(itemRows, row)
		itemIDs[row.ID] = struct{}{}
	}

	attemptRows := make([]StudioGenerationAttemptRecord, 0, len(attempts))
	for _, attempt := range attempts {
		if _, ok := itemIDs[attempt.ItemID]; !ok {
			return StudioBatchRecord{}, nil, nil, nil, ErrStudioBatchUnknownItemReference
		}
		row := attempt
		row.BatchID = batchRow.ID
		row.TenantID = batchRow.TenantID
		row.UserID = batchRow.UserID
		attemptRows = append(attemptRows, row)
	}

	designRows := make([]StudioMaterializedDesignRecord, 0, len(designs))
	for _, design := range designs {
		if _, ok := itemIDs[design.ItemID]; !ok {
			return StudioBatchRecord{}, nil, nil, nil, ErrStudioBatchUnknownItemReference
		}
		row := design
		row.BatchID = batchRow.ID
		row.TenantID = batchRow.TenantID
		row.UserID = batchRow.UserID
		if row.ReviewStatus == "" {
			row.ReviewStatus = StudioMaterializedDesignReviewStatusApproved
		}
		designRows = append(designRows, row)
	}

	return batchRow, itemRows, attemptRows, designRows, nil
}

func applyStudioBatchScopeDefaults(ctx context.Context, tenantID *string, userID *string) {
	if tenantID != nil && *tenantID == "" {
		*tenantID = tenantctx.TenantIDFromContext(ctx)
	}
	if userID != nil && *userID == "" {
		*userID = RequestUserIDFromContext(ctx)
	}
}

func applyStudioBatchAccessScope(db *gorm.DB, ctx context.Context) *gorm.DB {
	tenantID, ok := tenantctx.TenantScopeFromContext(ctx)
	if ok {
		if tenantID == tenantctx.DefaultTenantID {
			db = db.Where("(tenant_id = ? OR tenant_id = '' OR tenant_id IS NULL)", tenantID)
		} else {
			db = db.Where("tenant_id = ?", tenantID)
		}
	}
	if OwnerScopeEnabled() {
		if userID := RequestUserIDFromContext(ctx); userID != "" {
			db = db.Where("user_id = ?", userID)
		}
	}
	return db
}

func matchesStudioBatchScope(ctx context.Context, tenantID string, userID string) bool {
	if !tenantctx.MatchesTenant(tenantID, tenantctx.TenantIDFromContext(ctx)) {
		return false
	}
	if !OwnerScopeEnabled() {
		return true
	}
	requestUserID := RequestUserIDFromContext(ctx)
	return requestUserID == "" || requestUserID == userID
}
