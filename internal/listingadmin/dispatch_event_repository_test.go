package listingadmin

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestDispatchEventSummaryAggregatesActionsReasonsAndStoreBlockers(t *testing.T) {
	t.Parallel()

	db := newDispatchEventRepositoryTestDB(t)
	now := time.Date(2026, 6, 24, 15, 0, 0, 0, time.UTC)
	seedDispatchEventRows(t, db,
		dispatchEventTestRow(1, 10, 100, "shein", "skipped", "no_capacity", now.Add(-10*time.Minute), func(row *listingDispatchEvent) {
			row.Capacity = 8
			row.Queued = 8
			row.DailyLimit = 500
			row.OwnerNode = "node-a"
		}),
		dispatchEventTestRow(2, 10, 100, "shein", "skipped", "no_capacity", now.Add(-9*time.Minute), func(row *listingDispatchEvent) {
			row.Capacity = 8
			row.Queued = 7
			row.DailyLimit = 500
			row.OwnerNode = "node-a"
		}),
		dispatchEventTestRow(3, 10, 101, "shein", "skipped", "store_paused", now.Add(-8*time.Minute), func(row *listingDispatchEvent) {
			row.Capacity = 2
			row.OwnerNode = "node-b"
		}),
		dispatchEventTestRow(4, 10, 102, "shein", "dispatched", "", now.Add(-7*time.Minute), func(row *listingDispatchEvent) {
			row.Capacity = 2
			row.Queued = 1
			row.DailyLimit = 800
			row.OwnerNode = "node-c"
		}),
		dispatchEventTestRow(5, 10, 103, "shein", "failed", "publish_error", now.Add(-6*time.Minute), nil),
		dispatchEventTestRow(6, 10, 100, "amazon", "skipped", "no_capacity", now.Add(-5*time.Minute), nil),
		dispatchEventTestRow(7, 11, 100, "shein", "skipped", "no_capacity", now.Add(-4*time.Minute), nil),
		dispatchEventTestRow(8, 10, 100, "shein", "skipped", "outside_window", now.Add(-2*time.Hour), nil),
	)

	repo := NewGormDispatchEventRepository(db)
	summary, err := repo.GetDispatchEventSummary(context.Background(), DispatchEventQuery{
		TenantID: 10,
		Platform: "shein",
		From:     now.Add(-time.Hour),
		To:       now,
	})
	if err != nil {
		t.Fatalf("GetDispatchEventSummary() error = %v", err)
	}
	if summary.Total != 5 || summary.Dispatched != 1 || summary.Skipped != 3 || summary.Failed != 1 {
		t.Fatalf("summary counts = total:%d dispatched:%d skipped:%d failed:%d", summary.Total, summary.Dispatched, summary.Skipped, summary.Failed)
	}
	if summary.Window.From != now.Add(-time.Hour) || summary.Window.To != now {
		t.Fatalf("window = %+v, want query window", summary.Window)
	}
	if len(summary.ReasonCounts) != 3 {
		t.Fatalf("reason counts len = %d, want 3", len(summary.ReasonCounts))
	}
	if summary.ReasonCounts[0].ReasonCode != "no_capacity" || summary.ReasonCounts[0].Action != "skipped" || summary.ReasonCounts[0].Count != 2 {
		t.Fatalf("top reason = %+v, want skipped no_capacity count 2", summary.ReasonCounts[0])
	}
	if len(summary.StoreBlockers) == 0 {
		t.Fatalf("store blockers empty, want top blocker")
	}
	topBlocker := summary.StoreBlockers[0]
	if topBlocker.TenantID != 10 || topBlocker.StoreID != 100 || topBlocker.ReasonCode != "no_capacity" || topBlocker.Count != 2 {
		t.Fatalf("top blocker identity = %+v, want tenant 10 store 100 no_capacity count 2", topBlocker)
	}
	if topBlocker.DailyLimit != 500 || topBlocker.MaxQueued != 8 || topBlocker.OwnerNode != "node-a" {
		t.Fatalf("top blocker capacity fields = %+v, want daily limit 500 max queued 8 owner node-a", topBlocker)
	}
}

func TestListDispatchEventsFiltersPagesAndCountsMatchingRows(t *testing.T) {
	t.Parallel()

	db := newDispatchEventRepositoryTestDB(t)
	now := time.Date(2026, 6, 24, 15, 0, 0, 0, time.UTC)
	seedDispatchEventRows(t, db,
		dispatchEventTestRow(1, 10, 100, "shein", "skipped", "no_capacity", now.Add(-10*time.Minute), nil),
		dispatchEventTestRow(2, 10, 100, "shein", "skipped", "no_capacity", now.Add(-9*time.Minute), nil),
		dispatchEventTestRow(3, 10, 100, "shein", "dispatched", "", now.Add(-8*time.Minute), nil),
		dispatchEventTestRow(4, 10, 101, "shein", "skipped", "no_capacity", now.Add(-7*time.Minute), nil),
		dispatchEventTestRow(5, 11, 100, "shein", "skipped", "no_capacity", now.Add(-6*time.Minute), nil),
		dispatchEventTestRow(6, 10, 100, "amazon", "skipped", "no_capacity", now.Add(-5*time.Minute), nil),
		dispatchEventTestRow(7, 10, 100, "shein", "skipped", "store_paused", now.Add(-4*time.Minute), nil),
		dispatchEventTestRow(8, 10, 100, "shein", "skipped", "no_capacity", now.Add(-2*time.Hour), nil),
	)

	repo := NewGormDispatchEventRepository(db)
	page, err := repo.ListDispatchEvents(context.Background(), DispatchEventQuery{
		TenantID:   10,
		Platform:   " shein ",
		StoreID:    dispatchEventInt64Ptr(100),
		Action:     " skipped ",
		ReasonCode: " no_capacity ",
		From:       now.Add(-time.Hour),
		To:         now,
		Limit:      1,
		Offset:     1,
	})
	if err != nil {
		t.Fatalf("ListDispatchEvents() error = %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("page total = %d, want 2", page.Total)
	}
	if page.Limit != 1 || page.Offset != 1 {
		t.Fatalf("page limit/offset = %d/%d, want 1/1", page.Limit, page.Offset)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(page.Items))
	}
	if page.Items[0].TaskID != 1 || page.Items[0].ReasonCode != "no_capacity" || page.Items[0].Action != "skipped" {
		t.Fatalf("item = %+v, want second newest matching skipped no_capacity task 1", page.Items[0])
	}
}

func newDispatchEventRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlite db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&listingDispatchEvent{}); err != nil {
		t.Fatalf("migrate dispatch event: %v", err)
	}
	return db
}

func seedDispatchEventRows(t *testing.T, db *gorm.DB, rows ...listingDispatchEvent) {
	t.Helper()
	for _, row := range rows {
		if row.CreatedAt == nil {
			createdAt := time.Date(2026, 6, 24, 14, 0, 0, 0, time.UTC)
			row.CreatedAt = &createdAt
		}
		if err := db.Table("listing_dispatch_event").Create(&row).Error; err != nil {
			t.Fatalf("seed dispatch event task %d: %v", row.TaskID, err)
		}
	}
}

func dispatchEventTestRow(taskID, tenantID, storeID int64, platform, action, reasonCode string, createdAt time.Time, mutate func(*listingDispatchEvent)) listingDispatchEvent {
	row := listingDispatchEvent{
		TaskID:     taskID,
		TenantID:   tenantID,
		StoreID:    storeID,
		Platform:   platform,
		Action:     action,
		ReasonCode: reasonCode,
		Stage:      "dispatch",
		CreatedAt:  &createdAt,
	}
	if mutate != nil {
		mutate(&row)
	}
	return row
}

func dispatchEventInt64Ptr(value int64) *int64 {
	return &value
}
