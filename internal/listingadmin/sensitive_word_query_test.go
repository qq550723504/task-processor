package listingadmin

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestFindSensitiveWordRowsUsesRequestOwnerScope(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingSensitiveWord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingSensitiveWord{
		{TenantID: 101, OwnerUserID: "user-a", Word: "keep", Language: "en", Level: 1, Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-b", Word: "other-owner", Language: "en", Level: 1, Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Word: "deleted", Language: "en", Level: 1, Status: 1, Deleted: 1},
	} {
		if err := db.Table("listing_sensitive_word").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, page, pageSize, err := findSensitiveWordRows(ctx, db.Table("listing_sensitive_word"), SensitiveWordQuery{
		TenantID: 101,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("findSensitiveWordRows: %v", err)
	}
	if total != 1 || page != 1 || pageSize != 20 {
		t.Fatalf("result meta = total:%d page:%d pageSize:%d, want 1/1/20", total, page, pageSize)
	}
	if len(rows) != 1 || rows[0].Word != "keep" {
		t.Fatalf("rows = %+v, want only active owner-scoped row", rows)
	}
}

func TestFindSensitiveWordRowsAppliesResourceFilters(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingSensitiveWord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	level := 2
	status := int16(1)
	for _, row := range []listingSensitiveWord{
		{TenantID: 101, OwnerUserID: "user-a", Word: "restricted brand", Language: "en", Tags: "policy", Level: 2, Remark: "manual", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Word: "restricted brand", Language: "zh", Tags: "policy", Level: 2, Remark: "manual", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Word: "safe", Language: "en", Tags: "other", Level: 1, Remark: "auto", Status: 0, Deleted: 0},
	} {
		if err := db.Table("listing_sensitive_word").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, _, _, err := findSensitiveWordRows(ctx, db.Table("listing_sensitive_word"), SensitiveWordQuery{
		TenantID: 101,
		Word:     "restricted",
		Language: "en",
		Tags:     "policy",
		Level:    &level,
		Status:   &status,
		Remark:   "manual",
	})
	if err != nil {
		t.Fatalf("findSensitiveWordRows: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Language != "en" {
		t.Fatalf("rows = %+v total=%d, want only fully matched word", rows, total)
	}
}
