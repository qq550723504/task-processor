package listingadmin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
	"task-processor/internal/model"
)

func TestStoreStatisticsRepositoryRejectsInvalidDate(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingStore{}, &listingProductImportTask{}); err != nil {
		t.Fatalf("migrate statistics tables: %v", err)
	}

	repo := NewGormStoreStatisticsRepository(db)
	_, err = repo.ListStoreStatistics(context.Background(), StoreStatisticsQuery{
		TenantID: 101,
		Date:     "2026/05/15",
	})
	if err == nil {
		t.Fatal("expected invalid date to return error")
	}
	if !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Fatalf("err = %v, want YYYY-MM-DD hint", err)
	}
}

func TestStoreStatisticsRepositoryReturnsPaginatedPageWithFullSummary(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingStore{}, &listingProductImportTask{}); err != nil {
		t.Fatalf("migrate statistics tables: %v", err)
	}

	trueValue := true
	falseValue := false
	limitA := 5
	limitB := 7
	seedStoreStatisticsStore(t, db, listingStore{
		ID:                1,
		TenantID:          101,
		Name:              "Tenant 101 Store A",
		Username:          "store-a",
		Password:          "secret",
		Platform:          "SHEIN",
		ShopType:          "semi",
		DailyLimit:        &limitA,
		DailyLimitType:    "fixed",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})
	seedStoreStatisticsStore(t, db, listingStore{
		ID:                2,
		TenantID:          202,
		Name:              "Tenant 202 Store B",
		Username:          "store-b",
		Password:          "secret",
		Platform:          "TEMU",
		ShopType:          "semi",
		DailyLimit:        &limitB,
		DailyLimitType:    "fixed",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})
	seedStoreStatisticsStore(t, db, listingStore{
		ID:                3,
		TenantID:          303,
		Name:              "Disabled Store",
		Username:          "store-disabled",
		Password:          "secret",
		Platform:          "AMAZON",
		ShopType:          "semi",
		EnableAutoListing: &falseValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})

	date := "2026-05-15"
	seedStoreStatisticsTask(t, db, listingProductImportTask{TenantID: 101, StoreID: 1, ProductID: "A-completed", Status: model.TaskStatusPublished.Int16(), CreateTime: timePtrStoreStatistics(time.Date(2026, 5, 15, 8, 0, 0, 0, time.UTC))})
	seedStoreStatisticsTask(t, db, listingProductImportTask{TenantID: 101, StoreID: 1, ProductID: "A-pending", Status: 0, CreateTime: timePtrStoreStatistics(time.Date(2026, 5, 15, 8, 30, 0, 0, time.UTC))})
	seedStoreStatisticsTask(t, db, listingProductImportTask{TenantID: 101, StoreID: 1, ProductID: "A-queued", Status: 5, CreateTime: timePtrStoreStatistics(time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC))})
	seedStoreStatisticsTask(t, db, listingProductImportTask{TenantID: 202, StoreID: 2, ProductID: "B-completed", Status: model.TaskStatusPublished.Int16(), CreateTime: timePtrStoreStatistics(time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC))})
	seedStoreStatisticsTask(t, db, listingProductImportTask{TenantID: 202, StoreID: 2, ProductID: "B-hold", Status: 10, CreateTime: timePtrStoreStatistics(time.Date(2026, 5, 15, 10, 30, 0, 0, time.UTC))})
	seedStoreStatisticsTask(t, db, listingProductImportTask{TenantID: 303, StoreID: 3, ProductID: "disabled-completed", Status: model.TaskStatusPublished.Int16(), CreateTime: timePtrStoreStatistics(time.Date(2026, 5, 15, 11, 0, 0, 0, time.UTC))})

	repo := NewGormStoreStatisticsRepository(db)
	page, err := repo.ListStoreStatistics(context.Background(), StoreStatisticsQuery{
		Date:     date,
		Page:     2,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("ListStoreStatistics: %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("total = %d, want 2 eligible stores", page.Total)
	}
	if page.Page != 2 || page.PageSize != 1 {
		t.Fatalf("page metadata = %d/%d, want 2/1", page.Page, page.PageSize)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(page.Items))
	}
	if got := page.Items[0]; got.ID != 2 || got.Name != "Tenant 202 Store B" {
		t.Fatalf("page item = %+v, want second eligible store ordered by id asc", got)
	}
	if page.Summary.CompletedCount != 2 || page.Summary.DailyLimit != 12 || page.Summary.RemainingCount != 1 || page.Summary.QueuedCount != 1 || page.Summary.HoldCount != 1 {
		t.Fatalf("summary = %+v, want counts across both eligible stores only", page.Summary)
	}
}

func TestStoreStatisticsRepositoryCountsCanonicalPublishedAndDraftStatuses(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingStore{}, &listingProductImportTask{}); err != nil {
		t.Fatalf("migrate statistics tables: %v", err)
	}

	trueValue := true
	seedStoreStatisticsStore(t, db, listingStore{
		ID:                867,
		TenantID:          246,
		Name:              "ND02",
		Username:          "nd02",
		Password:          "secret",
		Platform:          "SHEIN",
		ShopType:          "semi",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})

	date := "2026-08-18"
	for _, task := range []listingProductImportTask{
		{TenantID: 246, StoreID: 867, ProductID: "published", Status: model.TaskStatusPublished.Int16()},
		{TenantID: 246, StoreID: 867, ProductID: "draft", Status: model.TaskStatusDraft.Int16()},
		{TenantID: 246, StoreID: 867, ProductID: "pending", Status: model.TaskStatusPending.Int16()},
		{TenantID: 246, StoreID: 867, ProductID: "queued", Status: model.TaskStatusQueued.Int16()},
		{TenantID: 246, StoreID: 867, ProductID: "paused", Status: model.TaskStatusPaused.Int16()},
		{TenantID: 246, StoreID: 867, ProductID: "terminated", Status: model.TaskStatusTerminated.Int16()},
	} {
		task.CreateTime = timePtrStoreStatistics(time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC))
		seedStoreStatisticsTask(t, db, task)
	}

	page, err := NewGormStoreStatisticsRepository(db).ListStoreStatistics(context.Background(), StoreStatisticsQuery{
		TenantID: 246,
		Date:     date,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListStoreStatistics: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %+v, want one store", page.Items)
	}
	got := page.Items[0]
	if got.CompletedCount != 2 || got.RemainingCount != 1 || got.QueuedCount != 1 || got.HoldCount != 1 {
		t.Fatalf("statistics = %+v, want published/draft=2 pending=1 queued=1 hold=1", got)
	}
}

func TestStoreStatisticsSummaryJSONUsesFrontendContractFields(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(StoreStatisticsSummary{
		CompletedCount: 3,
		DailyLimit:     11,
		RemainingCount: 4,
		QueuedCount:    5,
		HoldCount:      6,
	})
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}

	got := string(payload)
	for _, want := range []string{
		`"completed_count":3`,
		`"daily_limit":11`,
		`"remaining_count":4`,
		`"queued_count":5`,
		`"hold_count":6`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary json = %s, want %s", got, want)
		}
	}
	for _, unwanted := range []string{
		`"completedCount":3`,
		`"dailyLimit":11`,
		`"remainingCount":4`,
		`"queuedCount":5`,
		`"holdCount":6`,
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("summary json = %s, should not contain %s", got, unwanted)
		}
	}
}

func TestStoreStatisticsRepositoryReturnsEmptyPageMetadataWhenNoEligibleStoresMatch(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingStore{}, &listingProductImportTask{}); err != nil {
		t.Fatalf("migrate statistics tables: %v", err)
	}

	falseValue := false
	seedStoreStatisticsStore(t, db, listingStore{
		ID:                9,
		TenantID:          101,
		Name:              "Manual Store",
		Username:          "manual-store",
		Password:          "secret",
		Platform:          "SHEIN",
		ShopType:          "semi",
		EnableAutoListing: &falseValue,
		EnableAutoLogin:   &falseValue,
		Status:            0,
	})

	repo := NewGormStoreStatisticsRepository(db)
	page, err := repo.ListStoreStatistics(context.Background(), StoreStatisticsQuery{
		TenantID: 101,
		Date:     "2026-05-15",
		Page:     2,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("ListStoreStatistics: %v", err)
	}
	if page == nil {
		t.Fatal("page = nil, want non-nil empty page")
	}
	if page.Total != 0 || page.Page != 2 || page.PageSize != 1 {
		t.Fatalf("page metadata = %+v, want total=0 page=2 pageSize=1", page)
	}
	if len(page.Items) != 0 {
		t.Fatalf("items = %+v, want empty items slice", page.Items)
	}
	if page.Items == nil {
		t.Fatal("items = nil, want empty slice")
	}
	if page.Summary != (StoreStatisticsSummary{}) {
		t.Fatalf("summary = %+v, want zero-value summary", page.Summary)
	}
}

func seedStoreStatisticsStore(t *testing.T, db *gorm.DB, store listingStore) {
	t.Helper()
	if err := db.Table("listing_store").Create(&store).Error; err != nil {
		t.Fatalf("seed listing store: %v", err)
	}
}

func seedStoreStatisticsTask(t *testing.T, db *gorm.DB, task listingProductImportTask) {
	t.Helper()
	if task.CategoryID == nil {
		categoryID := int64(1)
		task.CategoryID = &categoryID
	}
	if err := db.Table("listing_product_import_task").Create(&task).Error; err != nil {
		t.Fatalf("seed import task: %v", err)
	}
}

func timePtrStoreStatistics(value time.Time) *time.Time {
	return &value
}
