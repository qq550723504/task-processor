package memberinvite

import (
	"context"
	"testing"

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
	if got.Email != "jane@example.com" {
		t.Fatalf("email = %q, want normalized email", got.Email)
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
		case "id", "actor_user_id", "tenant_id", "email", "role", "user_id", "authorization_id", "outcome", "error_code", "created_at":
		default:
			t.Fatalf("unexpected audit column %q", column.Name())
		}
	}
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
