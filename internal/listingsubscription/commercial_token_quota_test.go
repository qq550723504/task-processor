package listingsubscription

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestReadTokenQuotaUsesActiveFiniteEntitlement(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:commercial-token-quota?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&tenantEntitlementRow{}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	if err := db.Create(&tenantEntitlementRow{TenantID: "org-1", ModuleCode: ModuleListingKit, Status: StatusActive, StartsAt: &start, ExpiresAt: &end, LimitsJSON: `{"token":100}`, UpdatedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	quota, err := NewGormRepository(db).ReadTokenQuota(context.Background(), "org-1", start.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if quota.Total != 100 || quota.Metric != "token" || !quota.WindowStart.Equal(start) || !quota.WindowEnd.Equal(end) {
		t.Fatalf("quota=%+v", quota)
	}
}
