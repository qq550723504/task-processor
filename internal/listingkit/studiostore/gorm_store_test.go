package studiostore

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
)

func TestGormRepositoryOwnerScopeFiltersStudioSessions(t *testing.T) {
	t.Cleanup(listingkit.SetOwnerScopeRequiredForTesting(true))

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SheinStudioSession{}, &listingkit.SheinStudioDesign{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewGormRepository(db)
	baseCtx := listingkit.WithTenantID(context.Background(), "tenant-a")
	userACtx := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})

	sessionA := &listingkit.SheinStudioSession{
		ID:           "session-a",
		TenantID:     "tenant-a",
		UserID:       "user-a",
		SelectionKey: "selection-a",
		Status:       listingkit.SheinStudioSessionStatusSelecting,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	sessionB := &listingkit.SheinStudioSession{
		ID:           "session-b",
		TenantID:     "tenant-a",
		UserID:       "user-b",
		SelectionKey: "selection-b",
		Status:       listingkit.SheinStudioSessionStatusSelecting,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	for _, session := range []*listingkit.SheinStudioSession{sessionA, sessionB} {
		if err := repo.CreateSession(baseCtx, session); err != nil {
			t.Fatalf("create session %s: %v", session.ID, err)
		}
	}

	got, err := repo.GetSession(userACtx, "session-a")
	if err != nil {
		t.Fatalf("get owned session: %v", err)
	}
	if got == nil || got.ID != "session-a" {
		t.Fatalf("owned session = %#v, want session-a", got)
	}

	got, err = repo.GetSession(userACtx, "session-b")
	if err != nil {
		t.Fatalf("get foreign session: %v", err)
	}
	if got != nil {
		t.Fatalf("foreign session = %#v, want nil", got)
	}
}
