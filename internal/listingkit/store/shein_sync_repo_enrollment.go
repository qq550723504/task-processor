package store

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/listingkit"
)

func (r *GormSheinSyncRepository) SaveCandidates(ctx context.Context, records []*listingkit.SheinActivityCandidateRecord) error {
	for _, record := range records {
		if record == nil {
			continue
		}

		row := *record
		now := time.Now().UTC()
		row.UpdatedAt = now
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}

		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"},
				{Name: "store_id"},
				{Name: "activity_type"},
				{Name: "activity_key"},
				{Name: "skc_name"},
				{Name: "candidate_version"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"synced_product_id":      row.SyncedProductID,
				"effective_cost_price":   row.EffectiveCostPrice,
				"price_snapshot":         row.PriceSnapshot,
				"inventory_snapshot":     row.InventorySnapshot,
				"calculated_profit_rate": row.CalculatedProfitRate,
				"eligibility_status":     row.EligibilityStatus,
				"eligibility_reason":     row.EligibilityReason,
				"review_status":          row.ReviewStatus,
				"auto_mode_eligible":     row.AutoModeEligible,
				"selected_for_run":       row.SelectedForRun,
				"updated_at":             row.UpdatedAt,
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *GormSheinSyncRepository) ListCandidates(ctx context.Context, query *listingkit.SheinActivityCandidateQuery) ([]listingkit.SheinActivityCandidateRecord, int64, error) {
	page, pageSize := sheinActivityCandidateQueryPage(query)
	db := r.db.WithContext(ctx).Model(&listingkit.SheinActivityCandidateRecord{})
	db = applySheinActivityCandidateFilters(db, query)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []listingkit.SheinActivityCandidateRecord
	if err := db.
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	if err := r.attachLatestFailedEnrollmentErrors(ctx, rows); err != nil {
		return nil, 0, err
	}
	if err := r.attachCandidateMainImages(ctx, rows); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *GormSheinSyncRepository) attachCandidateMainImages(ctx context.Context, rows []listingkit.SheinActivityCandidateRecord) error {
	productIDs := make([]int64, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if row.SyncedProductID <= 0 {
			continue
		}
		if _, ok := seen[row.SyncedProductID]; ok {
			continue
		}
		seen[row.SyncedProductID] = struct{}{}
		productIDs = append(productIDs, row.SyncedProductID)
	}
	if len(productIDs) == 0 {
		return nil
	}

	var products []listingkit.SheinSyncedProductRecord
	if err := r.db.WithContext(ctx).
		Select("id", "main_image_url").
		Where("id IN ?", productIDs).
		Find(&products).Error; err != nil {
		return err
	}
	imageByProductID := make(map[int64]string, len(products))
	for _, product := range products {
		imageByProductID[product.ID] = product.MainImageURL
	}
	for i := range rows {
		rows[i].MainImageURL = imageByProductID[rows[i].SyncedProductID]
	}
	return nil
}

func (r *GormSheinSyncRepository) attachLatestFailedEnrollmentErrors(ctx context.Context, rows []listingkit.SheinActivityCandidateRecord) error {
	candidateIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.ID <= 0 || row.ReviewStatus != listingkit.SheinCandidateReviewStatusFailed {
			continue
		}
		candidateIDs = append(candidateIDs, row.ID)
	}
	if len(candidateIDs) == 0 {
		return nil
	}

	var items []listingkit.SheinActivityEnrollmentItemRecord
	if err := r.db.WithContext(ctx).
		Where("candidate_id IN ? AND status = ? AND error_message <> ''", candidateIDs, listingkit.SheinEnrollmentItemStatusFailed).
		Order("candidate_id ASC, updated_at DESC, id DESC").
		Find(&items).Error; err != nil {
		return err
	}
	errorsByCandidate := make(map[int64]string, len(items))
	for _, item := range items {
		if _, exists := errorsByCandidate[item.CandidateID]; exists {
			continue
		}
		errorsByCandidate[item.CandidateID] = item.ErrorMessage
	}
	for i := range rows {
		rows[i].LastEnrollmentError = errorsByCandidate[rows[i].ID]
	}
	return nil
}

func (r *GormSheinSyncRepository) CreateEnrollmentRun(ctx context.Context, run *listingkit.SheinActivityEnrollmentRunRecord) error {
	if run == nil {
		return nil
	}

	row := *run
	now := time.Now().UTC()
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	*run = row
	return nil
}

func (r *GormSheinSyncRepository) UpdateEnrollmentRun(ctx context.Context, run *listingkit.SheinActivityEnrollmentRunRecord) error {
	if run == nil {
		return nil
	}

	row := *run
	row.UpdatedAt = time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&listingkit.SheinActivityEnrollmentRunRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"tenant_id":       row.TenantID,
			"store_id":        row.StoreID,
			"activity_type":   row.ActivityType,
			"activity_key":    row.ActivityKey,
			"trigger_mode":    row.TriggerMode,
			"status":          row.Status,
			"candidate_count": row.CandidateCount,
			"submitted_count": row.SubmittedCount,
			"succeeded_count": row.SucceededCount,
			"failed_count":    row.FailedCount,
			"started_at":      row.StartedAt,
			"finished_at":     row.FinishedAt,
			"error_summary":   row.ErrorSummary,
			"updated_at":      row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	run.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *GormSheinSyncRepository) ListEnrollmentRuns(ctx context.Context, query *listingkit.SheinEnrollmentRunQuery) ([]listingkit.SheinActivityEnrollmentRunRecord, int64, error) {
	page, pageSize := sheinEnrollmentRunQueryPage(query)
	db := r.db.WithContext(ctx).Model(&listingkit.SheinActivityEnrollmentRunRecord{})
	db = applySheinEnrollmentRunFilters(db, query)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []listingkit.SheinActivityEnrollmentRunRecord
	if err := db.
		Order("started_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *GormSheinSyncRepository) ListEnrollmentItems(ctx context.Context, query *listingkit.SheinEnrollmentItemQuery) ([]listingkit.SheinActivityEnrollmentItemRecord, int64, error) {
	page, pageSize := sheinEnrollmentItemQueryPage(query)
	db := r.db.WithContext(ctx).
		Model(&listingkit.SheinActivityEnrollmentItemRecord{}).
		Joins("JOIN listingkit_shein_activity_enrollment_runs AS runs ON runs.id = listingkit_shein_activity_enrollment_items.run_id")
	db = applySheinEnrollmentItemFilters(db, query)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []listingkit.SheinActivityEnrollmentItemRecord
	selectColumns := "listingkit_shein_activity_enrollment_items.*"
	if query == nil || !query.IncludePayload {
		selectColumns = "listingkit_shein_activity_enrollment_items.id, listingkit_shein_activity_enrollment_items.run_id, listingkit_shein_activity_enrollment_items.candidate_id, listingkit_shein_activity_enrollment_items.store_id, listingkit_shein_activity_enrollment_items.activity_key, listingkit_shein_activity_enrollment_items.candidate_version, listingkit_shein_activity_enrollment_items.synced_product_id, listingkit_shein_activity_enrollment_items.skc_name, listingkit_shein_activity_enrollment_items.status, listingkit_shein_activity_enrollment_items.error_message, listingkit_shein_activity_enrollment_items.created_at, listingkit_shein_activity_enrollment_items.updated_at"
	}
	if err := db.
		Select(selectColumns).
		Order("listingkit_shein_activity_enrollment_items.id ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *GormSheinSyncRepository) SaveEnrollmentItems(ctx context.Context, items []*listingkit.SheinActivityEnrollmentItemRecord) error {
	for _, item := range items {
		if item == nil {
			continue
		}

		row := *item
		now := time.Now().UTC()
		row.UpdatedAt = now
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}

		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "run_id"},
				{Name: "candidate_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"store_id":          row.StoreID,
				"activity_key":      row.ActivityKey,
				"candidate_version": row.CandidateVersion,
				"synced_product_id": row.SyncedProductID,
				"skc_name":          row.SKCName,
				"status":            row.Status,
				"request_payload":   row.RequestPayload,
				"response_payload":  row.ResponsePayload,
				"error_message":     row.ErrorMessage,
				"updated_at":        row.UpdatedAt,
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
