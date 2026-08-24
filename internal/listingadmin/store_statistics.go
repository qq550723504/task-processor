package listingadmin

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/model"
)

type StoreStatistics struct {
	ID                 int64   `json:"id"`
	StoreID            string  `json:"storeId,omitempty"`
	TenantID           int64   `json:"tenantId"`
	Name               string  `json:"name"`
	Platform           string  `json:"platform,omitempty"`
	DailyLimit         int     `json:"dailyLimit"`
	DailyLimitType     string  `json:"dailyLimitType,omitempty"`
	CompletedCount     int     `json:"completedCount"`
	RemainingCount     int     `json:"remainingCount"`
	HoldCount          int     `json:"holdCount"`
	QueuedCount        int     `json:"queuedCount"`
	RemainingQuota     int     `json:"remainingQuota"`
	ProgressPercentage float64 `json:"progressPercentage"`
	Status             int16   `json:"status"`
}

type StoreStatisticsQuery struct {
	TenantID    int64
	OwnerUserID string
	Date        string
	Page        int
	PageSize    int
}

type StoreStatisticsSummary struct {
	CompletedCount int `json:"completed_count"`
	DailyLimit     int `json:"daily_limit"`
	RemainingCount int `json:"remaining_count"`
	QueuedCount    int `json:"queued_count"`
	HoldCount      int `json:"hold_count"`
}

type StoreStatisticsPage struct {
	Items    []StoreStatistics      `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Summary  StoreStatisticsSummary `json:"summary"`
}

type StoreStatisticsRepository interface {
	ListStoreStatistics(ctx context.Context, query StoreStatisticsQuery) (*StoreStatisticsPage, error)
}

type GormStoreStatisticsRepository struct {
	db *gorm.DB
}

func NewGormStoreStatisticsRepository(db *gorm.DB) *GormStoreStatisticsRepository {
	return &GormStoreStatisticsRepository{db: db}
}

func AutoMigrateStoreStatisticsRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	if err := ensureOwnerAuditColumns(db, (listingStore{}).TableName()); err != nil {
		return err
	}
	return ensureOwnerAuditColumns(db, (listingProductImportTask{}).TableName())
}

func (r *GormStoreStatisticsRepository) ListStoreStatistics(ctx context.Context, query StoreStatisticsQuery) (*StoreStatisticsPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("store statistics repository database is not configured")
	}
	date, err := resolveStatisticsDate(query.Date)
	if err != nil {
		return nil, err
	}
	query.Date = date

	page, pageSize := normalizePage(query.Page, query.PageSize)
	query.Page = page
	query.PageSize = pageSize

	total, err := r.countEligibleStores(ctx, query)
	if err != nil {
		return nil, err
	}
	summary, err := r.summarizeTasks(ctx, query)
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return &StoreStatisticsPage{
			Items:    []StoreStatistics{},
			Total:    0,
			Page:     query.Page,
			PageSize: query.PageSize,
			Summary:  summary,
		}, nil
	}

	stores, err := r.listEligibleStores(ctx, query)
	if err != nil {
		return nil, err
	}

	items := make([]StoreStatistics, 0, len(stores))
	for _, store := range stores {
		counts, err := r.countTasks(ctx, store.TenantID, store.ID, query.Date)
		if err != nil {
			return nil, err
		}
		items = append(items, buildStoreStatistics(store, counts))
	}
	return &StoreStatisticsPage{
		Items:    items,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
		Summary:  summary,
	}, nil
}

func (r *GormStoreStatisticsRepository) eligibleStoresQuery(ctx context.Context, query StoreStatisticsQuery) *gorm.DB {
	db := r.db.WithContext(ctx).Table("listing_store").
		Where("deleted = 0 AND status = 0").
		Where("enable_auto_listing = ? AND enable_auto_login = ?", true, true)
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	return applyOwnerScopeForUser(db, ctx, query.OwnerUserID, "owner_user_id")
}

func (r *GormStoreStatisticsRepository) countEligibleStores(ctx context.Context, query StoreStatisticsQuery) (int64, error) {
	var total int64
	if err := r.eligibleStoresQuery(ctx, query).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *GormStoreStatisticsRepository) listEligibleStores(ctx context.Context, query StoreStatisticsQuery) ([]listingStore, error) {
	var stores []listingStore
	if err := r.eligibleStoresQuery(ctx, query).
		Order("id asc").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&stores).Error; err != nil {
		return nil, err
	}
	return stores, nil
}

type storeTaskStatisticsCounts struct {
	Completed int
	Pending   int
	Queued    int
	Hold      int
}

var storeStatisticsCountedStatuses = []int16{
	model.TaskStatusPending.Int16(),
	model.TaskStatusProcessing.Int16(),
	model.TaskStatusCrawled.Int16(),
	model.TaskStatusPendingRetry.Int16(),
	model.TaskStatusQueued.Int16(),
	model.TaskStatusPublished.Int16(),
	model.TaskStatusRepublishing.Int16(),
	model.TaskStatusDraft.Int16(),
	model.TaskStatusPaused.Int16(),
	model.TaskStatusResumed.Int16(),
	model.TaskStatusResuming.Int16(),
}

func (r *GormStoreStatisticsRepository) countTasks(ctx context.Context, tenantID, storeID int64, date string) (storeTaskStatisticsCounts, error) {
	var rows []struct {
		Status int16
		Count  int64
	}
	db := r.db.WithContext(ctx).Table("listing_product_import_task").
		Select("status, count(*) as count").
		Where("deleted = 0 AND tenant_id = ? AND store_id = ?", tenantID, storeID).
		Where("status IN ?", storeStatisticsCountedStatuses)
	if date != "" {
		start, end, ok := statisticsDateRange(date)
		if ok {
			db = db.Where("create_time >= ? AND create_time < ?", start, end)
		}
	}
	err := db.Group("status").Scan(&rows).Error
	if err != nil {
		return storeTaskStatisticsCounts{}, err
	}
	var counts storeTaskStatisticsCounts
	for _, row := range rows {
		accumulateStoreTaskStatistics(&counts, row.Status, row.Count)
	}
	return counts, nil
}

func (r *GormStoreStatisticsRepository) summarizeTasks(ctx context.Context, query StoreStatisticsQuery) (StoreStatisticsSummary, error) {
	eligibleStores := r.eligibleStoresQuery(ctx, query).Select("id, tenant_id")
	var limitRow struct {
		DailyLimit int
	}
	if err := r.eligibleStoresQuery(ctx, query).
		Select("coalesce(sum(case when daily_limit > 0 then daily_limit else 0 end), 0) as daily_limit").
		Scan(&limitRow).Error; err != nil {
		return StoreStatisticsSummary{}, err
	}
	db := r.db.WithContext(ctx).
		Table("listing_product_import_task as task").
		Select("task.status as status, count(*) as count").
		Joins("join (?) as eligible_stores on eligible_stores.id = task.store_id and eligible_stores.tenant_id = task.tenant_id", eligibleStores).
		Where("task.deleted = 0").
		Where("task.status IN ?", storeStatisticsCountedStatuses)
	if query.Date != "" {
		start, end, ok := statisticsDateRange(query.Date)
		if ok {
			db = db.Where("task.create_time >= ? AND task.create_time < ?", start, end)
		}
	}
	var rows []struct {
		Status int16
		Count  int64
	}
	if err := db.Group("task.status").Scan(&rows).Error; err != nil {
		return StoreStatisticsSummary{}, err
	}
	counts := storeTaskStatisticsCounts{}
	for _, row := range rows {
		accumulateStoreTaskStatistics(&counts, row.Status, row.Count)
	}
	return StoreStatisticsSummary{
		CompletedCount: counts.Completed,
		DailyLimit:     limitRow.DailyLimit,
		RemainingCount: counts.Pending,
		QueuedCount:    counts.Queued,
		HoldCount:      counts.Hold,
	}, nil
}

func accumulateStoreTaskStatistics(counts *storeTaskStatisticsCounts, status int16, count int64) {
	if counts == nil {
		return
	}
	switch model.TaskStatus(status) {
	case model.TaskStatusPending,
		model.TaskStatusProcessing,
		model.TaskStatusCrawled,
		model.TaskStatusPendingRetry,
		model.TaskStatusRepublishing,
		model.TaskStatusResumed,
		model.TaskStatusResuming:
		counts.Pending += int(count)
	case model.TaskStatusQueued:
		counts.Queued += int(count)
	case model.TaskStatusPublished, model.TaskStatusDraft:
		counts.Completed += int(count)
	case model.TaskStatusPaused:
		counts.Hold += int(count)
	}
}

func buildStoreStatistics(store listingStore, counts storeTaskStatisticsCounts) StoreStatistics {
	dailyLimit := 0
	if store.DailyLimit != nil {
		dailyLimit = *store.DailyLimit
	}
	remainingQuota := dailyLimit - counts.Completed
	if remainingQuota < 0 {
		remainingQuota = 0
	}
	progress := 0.0
	if dailyLimit > 0 {
		progress = math.Min(100, float64(counts.Completed)*100/float64(dailyLimit))
		progress = math.Round(progress*100) / 100
	}
	return StoreStatistics{
		ID:                 store.ID,
		StoreID:            store.StoreID,
		TenantID:           store.TenantID,
		Name:               store.Name,
		Platform:           store.Platform,
		DailyLimit:         dailyLimit,
		DailyLimitType:     store.DailyLimitType,
		CompletedCount:     counts.Completed,
		RemainingCount:     counts.Pending,
		HoldCount:          counts.Hold,
		QueuedCount:        counts.Queued,
		RemainingQuota:     remainingQuota,
		ProgressPercentage: progress,
		Status:             store.Status,
	}
}

func resolveStatisticsDate(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().Format("2006-01-02"), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return "", fmt.Errorf("date must use YYYY-MM-DD format")
	}
	return parsed.Format("2006-01-02"), nil
}

func statisticsDateRange(value string) (time.Time, time.Time, bool) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return parsed, parsed.AddDate(0, 0, 1), true
}
