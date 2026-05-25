package listingadmin

import (
	"context"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
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
