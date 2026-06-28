package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

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

func sheinEnrollmentItemQueryPage(query *listingkit.SheinEnrollmentItemQuery) (int, int) {
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
	if query.ExecutableOnly {
		db = db.Where("eligibility_status = ?", listingkit.SheinCandidateEligibilityStatusEligible)
		db = db.Where("review_status IN ?", []listingkit.SheinCandidateReviewStatus{
			listingkit.SheinCandidateReviewStatusPendingReview,
			listingkit.SheinCandidateReviewStatusApproved,
			listingkit.SheinCandidateReviewStatusAutoQueued,
		})
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

func applySheinEnrollmentItemFilters(db *gorm.DB, query *listingkit.SheinEnrollmentItemQuery) *gorm.DB {
	if query == nil {
		return db
	}
	if query.RunID > 0 {
		db = db.Where("listingkit_shein_activity_enrollment_items.run_id = ?", query.RunID)
	}
	if query.Status != nil {
		db = db.Where("listingkit_shein_activity_enrollment_items.status = ?", *query.Status)
	}
	if query.TenantID > 0 {
		db = db.Where("runs.tenant_id = ?", query.TenantID)
	}
	if query.StoreID > 0 {
		db = db.Where("runs.store_id = ?", query.StoreID)
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
