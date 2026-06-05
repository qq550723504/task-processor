package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/listingkit"
)

type GormSheinSyncRepository struct {
	db *gorm.DB
}

func NewSheinSyncRepository(db *gorm.DB) listingkit.SheinSyncRepository {
	return &GormSheinSyncRepository{db: db}
}

func AutoMigrateSheinSyncRepository(db *gorm.DB) error {
	return db.AutoMigrate(
		&listingkit.SheinSyncedProductRecord{},
		&listingkit.SheinSyncJobRecord{},
		&listingkit.SheinActivityCandidateRecord{},
		&listingkit.SheinActivityEnrollmentRunRecord{},
		&listingkit.SheinActivityEnrollmentItemRecord{},
	)
}

func (r *GormSheinSyncRepository) UpsertSyncedProducts(ctx context.Context, records []*listingkit.SheinSyncedProductRecord) error {
	for _, record := range records {
		if record == nil {
			continue
		}

		row := *record
		listingkit.ApplyEffectiveCostPrice(&row)
		now := time.Now().UTC()
		row.UpdatedAt = now
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}
		if row.LastSyncAt == nil {
			row.LastSyncAt = &now
		}

		updates := sheinSyncedProductAssignments(row)
		var existing listingkit.SheinSyncedProductRecord
		err := r.db.WithContext(ctx).
			Where("tenant_id = ? AND store_id = ? AND skc_name = ?", row.TenantID, row.StoreID, row.SKCName).
			First(&existing).Error
		switch {
		case err == nil:
			if record.CreatedAt.IsZero() {
				row.CreatedAt = existing.CreatedAt
			}
			updates = sheinSyncedProductAssignments(row)
			if updateErr := r.db.WithContext(ctx).
				Model(&listingkit.SheinSyncedProductRecord{}).
				Where("id = ?", existing.ID).
				Updates(updates).Error; updateErr != nil {
				return updateErr
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			if createErr := r.db.WithContext(ctx).
				Model(&listingkit.SheinSyncedProductRecord{}).
				Create(updates).Error; createErr != nil {
				return createErr
			}
		default:
			return err
		}
	}
	return nil
}

func (r *GormSheinSyncRepository) ListSyncedProducts(ctx context.Context, query *listingkit.SheinSyncedProductQuery) ([]listingkit.SheinSyncedProductRecord, int64, error) {
	page, pageSize := sheinSyncQueryPage(query)
	db := r.db.WithContext(ctx).Model(&listingkit.SheinSyncedProductRecord{})
	db = applySheinSyncedProductFilters(db, query)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []listingkit.SheinSyncedProductRecord
	if err := db.
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *GormSheinSyncRepository) UpdateManualCostPrice(ctx context.Context, productID int64, manualCostPrice *float64) error {
	var row listingkit.SheinSyncedProductRecord
	if err := r.db.WithContext(ctx).Where("id = ?", productID).First(&row).Error; err != nil {
		return err
	}

	row.ManualCostPrice = cloneFloat64Ptr(manualCostPrice)
	listingkit.ApplyEffectiveCostPrice(&row)
	row.UpdatedAt = time.Now().UTC()

	result := r.db.WithContext(ctx).
		Model(&listingkit.SheinSyncedProductRecord{}).
		Where("id = ?", productID).
		Updates(map[string]any{
			"manual_cost_price":    row.ManualCostPrice,
			"effective_cost_price": row.EffectiveCostPrice,
			"cost_price_source":    row.CostPriceSource,
			"updated_at":           row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormSheinSyncRepository) MarkMissingSyncedProductsInactive(ctx context.Context, tenantID, storeID int64, activeSKCNames []string) error {
	db := r.db.WithContext(ctx).
		Model(&listingkit.SheinSyncedProductRecord{}).
		Where("tenant_id = ? AND store_id = ?", tenantID, storeID)
	if len(activeSKCNames) > 0 {
		db = db.Where("skc_name NOT IN ?", activeSKCNames)
	}
	return db.Updates(map[string]any{
		"is_active":  false,
		"updated_at": time.Now().UTC(),
	}).Error
}

func (r *GormSheinSyncRepository) SaveSyncJob(ctx context.Context, job *listingkit.SheinSyncJobRecord) error {
	if job == nil {
		return nil
	}

	row := *job
	now := time.Now().UTC()
	row.UpdatedAt = now
	if row.ID <= 0 {
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
		*job = row
		return nil
	}

	result := r.db.WithContext(ctx).
		Model(&listingkit.SheinSyncJobRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"tenant_id":         row.TenantID,
			"store_id":          row.StoreID,
			"trigger_mode":      row.TriggerMode,
			"status":            row.Status,
			"started_at":        row.StartedAt,
			"finished_at":       row.FinishedAt,
			"fetched_count":     row.FetchedCount,
			"inserted_count":    row.InsertedCount,
			"updated_count":     row.UpdatedCount,
			"deactivated_count": row.DeactivatedCount,
			"skipped_count":     row.SkippedCount,
			"error_summary":     row.ErrorSummary,
			"updated_at":        row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	job.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *GormSheinSyncRepository) ListSyncJobs(ctx context.Context, query *listingkit.SheinSyncJobQuery) ([]listingkit.SheinSyncJobRecord, int64, error) {
	page, pageSize := sheinSyncJobQueryPage(query)
	db := r.db.WithContext(ctx).Model(&listingkit.SheinSyncJobRecord{})
	db = applySheinSyncJobFilters(db, query)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []listingkit.SheinSyncJobRecord
	if err := db.
		Order("started_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

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
	return rows, total, nil
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

func normalizeSheinSyncPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func sheinSyncQueryPage(query *listingkit.SheinSyncedProductQuery) (int, int) {
	if query == nil {
		return normalizeSheinSyncPage(0, 0)
	}
	return normalizeSheinSyncPage(query.Page, query.PageSize)
}

func sheinSyncJobQueryPage(query *listingkit.SheinSyncJobQuery) (int, int) {
	if query == nil {
		return normalizeSheinSyncPage(0, 0)
	}
	return normalizeSheinSyncPage(query.Page, query.PageSize)
}

func sheinActivityCandidateQueryPage(query *listingkit.SheinActivityCandidateQuery) (int, int) {
	if query == nil {
		return normalizeSheinSyncPage(0, 0)
	}
	if len(query.CandidateIDs) > 0 {
		page := query.Page
		if page <= 0 {
			page = 1
		}
		pageSize := query.PageSize
		if pageSize <= 0 {
			pageSize = len(query.CandidateIDs)
		}
		if pageSize < len(query.CandidateIDs) {
			pageSize = len(query.CandidateIDs)
		}
		return page, pageSize
	}
	return normalizeSheinSyncPage(query.Page, query.PageSize)
}

func sheinEnrollmentRunQueryPage(query *listingkit.SheinEnrollmentRunQuery) (int, int) {
	if query == nil {
		return normalizeSheinSyncPage(0, 0)
	}
	return normalizeSheinSyncPage(query.Page, query.PageSize)
}

func applySheinSyncedProductFilters(db *gorm.DB, query *listingkit.SheinSyncedProductQuery) *gorm.DB {
	if query == nil {
		return db
	}
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if query.StoreID > 0 {
		db = db.Where("store_id = ?", query.StoreID)
	}
	if query.SKCName != "" {
		db = db.Where("skc_name = ?", query.SKCName)
	}
	if query.IsActive != nil {
		db = db.Where("is_active = ?", *query.IsActive)
	}
	return db
}

func applySheinSyncJobFilters(db *gorm.DB, query *listingkit.SheinSyncJobQuery) *gorm.DB {
	if query == nil {
		return db
	}
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if query.StoreID > 0 {
		db = db.Where("store_id = ?", query.StoreID)
	}
	if query.TriggerMode != nil {
		db = db.Where("trigger_mode = ?", *query.TriggerMode)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.StartedFrom != nil {
		db = db.Where("started_at >= ?", *query.StartedFrom)
	}
	if query.StartedTo != nil {
		db = db.Where("started_at <= ?", *query.StartedTo)
	}
	return db
}

func applySheinActivityCandidateFilters(db *gorm.DB, query *listingkit.SheinActivityCandidateQuery) *gorm.DB {
	if query == nil {
		return db
	}
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if query.StoreID > 0 {
		db = db.Where("store_id = ?", query.StoreID)
	}
	if query.ActivityType != "" {
		db = db.Where("activity_type = ?", query.ActivityType)
	}
	if query.ActivityKey != "" {
		db = db.Where("activity_key = ?", query.ActivityKey)
	}
	if query.SKCName != "" {
		db = db.Where("skc_name = ?", query.SKCName)
	}
	if query.CandidateVersion != "" {
		db = db.Where("candidate_version = ?", query.CandidateVersion)
	}
	if len(query.CandidateIDs) > 0 {
		db = db.Where("id IN ?", query.CandidateIDs)
	}
	return db
}

func applySheinEnrollmentRunFilters(db *gorm.DB, query *listingkit.SheinEnrollmentRunQuery) *gorm.DB {
	if query == nil {
		return db
	}
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if query.StoreID > 0 {
		db = db.Where("store_id = ?", query.StoreID)
	}
	if query.ActivityType != "" {
		db = db.Where("activity_type = ?", query.ActivityType)
	}
	if query.ActivityKey != "" {
		db = db.Where("activity_key = ?", query.ActivityKey)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}

func cloneFloat64Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}

func cloneTimePtr(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}

func sheinSyncedProductKey(tenantID, storeID int64, skcName string) string {
	return fmt.Sprintf("%d:%d:%s", tenantID, storeID, skcName)
}

func sheinCandidateKey(record listingkit.SheinActivityCandidateRecord) string {
	return fmt.Sprintf("%d:%d:%s:%s:%s:%s", record.TenantID, record.StoreID, record.ActivityType, record.ActivityKey, record.SKCName, record.CandidateVersion)
}

func sheinSyncedProductAssignments(row listingkit.SheinSyncedProductRecord) map[string]any {
	return map[string]any{
		"tenant_id":            row.TenantID,
		"store_id":             row.StoreID,
		"spu_name":             row.SPUName,
		"spu_code":             row.SPUCode,
		"skc_name":             row.SKCName,
		"skc_code":             row.SKCCode,
		"supplier_code":        row.SupplierCode,
		"category_id":          row.CategoryID,
		"brand_name":           row.BrandName,
		"product_name_multi":   row.ProductNameMulti,
		"main_image_url":       row.MainImageURL,
		"sale_name":            row.SaleName,
		"shelf_status":         row.ShelfStatus,
		"publish_time":         row.PublishTime,
		"first_shelf_time":     row.FirstShelfTime,
		"currency":             row.Currency,
		"price_snapshot":       row.PriceSnapshot,
		"inventory_snapshot":   row.InventorySnapshot,
		"site_snapshot":        row.SiteSnapshot,
		"auto_cost_price":      row.AutoCostPrice,
		"manual_cost_price":    row.ManualCostPrice,
		"effective_cost_price": row.EffectiveCostPrice,
		"cost_price_source":    row.CostPriceSource,
		"sync_version":         row.SyncVersion,
		"last_sync_at":         row.LastSyncAt,
		"is_active":            row.IsActive,
		"created_at":           row.CreatedAt,
		"updated_at":           row.UpdatedAt,
	}
}
