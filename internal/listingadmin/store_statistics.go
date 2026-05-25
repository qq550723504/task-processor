package listingadmin

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
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
}

type StoreStatisticsRepository interface {
	ListStoreStatistics(ctx context.Context, query StoreStatisticsQuery) ([]StoreStatistics, error)
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

func (r *GormStoreStatisticsRepository) ListStoreStatistics(ctx context.Context, query StoreStatisticsQuery) ([]StoreStatistics, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("store statistics repository database is not configured")
	}
	query.Date = normalizeStatisticsDate(query.Date)

	stores, err := r.listEligibleStores(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(stores) == 0 {
		return []StoreStatistics{}, nil
	}

	items := make([]StoreStatistics, 0, len(stores))
	for _, store := range stores {
		counts, err := r.countTasks(ctx, store.TenantID, store.ID, query.Date)
		if err != nil {
			return nil, err
		}
		items = append(items, buildStoreStatistics(store, counts))
	}
	return items, nil
}

func (r *GormStoreStatisticsRepository) listEligibleStores(ctx context.Context, query StoreStatisticsQuery) ([]listingStore, error) {
	db := r.db.WithContext(ctx).Table("listing_store").
		Where("deleted = 0 AND status = 0").
		Where("enable_auto_listing = ? AND enable_auto_login = ?", true, true)
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if ownerScopeEnabled() && strings.TrimSpace(query.OwnerUserID) != "" {
		db = db.Where("owner_user_id = ?", strings.TrimSpace(query.OwnerUserID))
	}
	var stores []listingStore
	if err := db.Order("id asc").Find(&stores).Error; err != nil {
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

func (r *GormStoreStatisticsRepository) countTasks(ctx context.Context, tenantID, storeID int64, date string) (storeTaskStatisticsCounts, error) {
	var rows []struct {
		Status int16
		Count  int64
	}
	db := r.db.WithContext(ctx).Table("listing_product_import_task").
		Select("status, count(*) as count").
		Where("deleted = 0 AND tenant_id = ? AND store_id = ?", tenantID, storeID).
		Where("status IN ?", []int16{0, 1, 2, 5, 10})
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
		switch row.Status {
		case 0, 1:
			counts.Pending += int(row.Count)
		case 2:
			counts.Completed += int(row.Count)
		case 5:
			counts.Queued += int(row.Count)
		case 10:
			counts.Hold += int(row.Count)
		}
	}
	return counts, nil
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

func normalizeStatisticsDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().Format("2006-01-02")
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return value
	}
	return parsed.Format("2006-01-02")
}

func statisticsDateRange(value string) (time.Time, time.Time, bool) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return parsed, parsed.AddDate(0, 0, 1), true
}
