package memberinvite

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestGormAuditRepositoryStoresIncompleteInvitationWithoutSecrets(t *testing.T) {
	db := openAuditTestDB(t)
	if err := AutoMigrateAuditRepository(db); err != nil {
		t.Fatalf("AutoMigrateAuditRepository() error = %v", err)
	}

	repo := NewGormAuditRepository(db)
	err := repo.Record(context.Background(), AuditRecord{
		ActorUserID: "admin-1",
		TenantID:    "org-1",
		Email:       " Jane@Example.COM ",
		Role:        "listingkit_viewer",
		UserID:      "user-1",
		Outcome:     OutcomeIncomplete,
		ErrorCode:   "zitadel_member_invitation_incomplete",
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	got := readAuditRecord(t, db)
	if got.ActorUserID != "admin-1" || got.TenantID != "org-1" || got.UserID != "user-1" || got.Outcome != OutcomeIncomplete {
		t.Fatalf("audit = %#v", got)
	}
	if got.Email != "j***@example.com" {
		t.Fatalf("email = %q, want masked email", got.Email)
	}
	if got.AuthorizationID != "" {
		t.Fatalf("authorization ID = %q, want empty for incomplete invitation", got.AuthorizationID)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("created at is zero")
	}
}

func TestGormAuditRepositoryStoresOnlyAuditFields(t *testing.T) {
	db := openAuditTestDB(t)
	if err := AutoMigrateAuditRepository(db); err != nil {
		t.Fatalf("AutoMigrateAuditRepository() error = %v", err)
	}

	if err := NewGormAuditRepository(db).Record(context.Background(), AuditRecord{
		ActorUserID:     "admin-1",
		TenantID:        "org-1",
		Email:           "jane@example.com",
		Role:            "listingkit_viewer",
		UserID:          "user-1",
		AuthorizationID: "authorization-1",
		Outcome:         OutcomeSucceeded,
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	columns, err := db.Migrator().ColumnTypes(&memberInvitationAuditRow{})
	if err != nil {
		t.Fatalf("audit columns: %v", err)
	}
	for _, column := range columns {
		switch column.Name() {
		case "id", "actor_user_id", "tenant_id", "email", "phone", "delivery_mode", "contact", "role", "user_id", "authorization_id", "outcome", "error_code", "created_at":
		default:
			t.Fatalf("unexpected audit column %q", column.Name())
		}
	}
}

func TestGormAuditRepositoryStoresMaskedDeliveryModeAndContacts(t *testing.T) {
	db := openAuditTestDB(t)
	if err := AutoMigrateAuditRepository(db); err != nil {
		t.Fatalf("AutoMigrateAuditRepository() error = %v", err)
	}

	record := AuditRecord{ActorUserID: "admin-1", TenantID: "org-1", Email: "jane@example.com", Role: "listingkit_viewer", Outcome: OutcomeSucceeded}
	record.Phone = "+8613712345678"
	if err := NewGormAuditRepository(db).Record(context.Background(), record); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	got := readAuditRecord(t, db)
	if got.Email == "jane@example.com" {
		t.Fatalf("email was stored unmasked: %#v", got)
	}
	if value := got.Phone; value == "+8613712345678" || value == "" {
		t.Fatalf("phone audit value = %q", value)
	}
	if mode := got.DeliveryMode; mode != "email_phone" {
		t.Fatalf("delivery mode = %q", mode)
	}
}

func TestAutoMigrateAuditRepositoryPreservesAndRedactsLegacyRecords(t *testing.T) {
	db := openAuditTestDB(t)
	if err := db.AutoMigrate(&legacyMemberInvitationAuditRow{}); err != nil {
		t.Fatalf("migrate legacy audit table: %v", err)
	}
	if err := db.Create(&legacyMemberInvitationAuditRow{
		ActorUserID: "admin-1", TenantID: "org-1", Email: "jane@example.com", Role: "listingkit_viewer", Outcome: OutcomeSucceeded,
	}).Error; err != nil {
		t.Fatalf("create legacy audit row: %v", err)
	}

	if err := AutoMigrateAuditRepository(db); err != nil {
		t.Fatalf("AutoMigrateAuditRepository() error = %v", err)
	}
	got := readAuditRecord(t, db)
	if got.Email != "j***@example.com" || got.DeliveryMode != "email" || got.Contact != "j***@example.com" {
		t.Fatalf("migrated audit row = %#v", got)
	}
}

type legacyMemberInvitationAuditRow struct {
	ID              uint64    `gorm:"primaryKey"`
	ActorUserID     string    `gorm:"type:varchar(128);not null;index"`
	TenantID        string    `gorm:"type:varchar(128);not null;index"`
	Email           string    `gorm:"type:varchar(320);not null;index"`
	Role            string    `gorm:"type:varchar(64);not null"`
	UserID          string    `gorm:"type:varchar(128)"`
	AuthorizationID string    `gorm:"type:varchar(128)"`
	Outcome         Outcome   `gorm:"type:varchar(32);not null;index"`
	ErrorCode       string    `gorm:"type:varchar(128)"`
	CreatedAt       time.Time `gorm:"not null;autoCreateTime"`
}

func (legacyMemberInvitationAuditRow) TableName() string {
	return "listingkit_member_invitation_audits"
}

func openAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return db
}

func readAuditRecord(t *testing.T, db *gorm.DB) memberInvitationAuditRow {
	t.Helper()

	var row memberInvitationAuditRow
	if err := db.Order("id DESC").First(&row).Error; err != nil {
		t.Fatalf("read audit record: %v", err)
	}
	return row
}
