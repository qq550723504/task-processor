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

func TestGormRepositoryReplaceDesignsAssignsSessionIDForBatchGallery(t *testing.T) {
	t.Cleanup(listingkit.SetOwnerScopeRequiredForTesting(false))

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SheinStudioSession{}, &listingkit.SheinStudioDesign{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewGormRepository(db)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	session := &listingkit.SheinStudioSession{
		ID:           "batch-1",
		TenantID:     "tenant-a",
		UserID:       "user-a",
		SelectionKey: "selection-a",
		Status:       listingkit.SheinStudioSessionStatusReviewing,
		SavedAsBatch: true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := repo.ReplaceDesigns(ctx, session.ID, []string{"design-1"}, []listingkit.SheinStudioDesign{
		{
			ID:       "design-1",
			ImageURL: "https://oss.example.com/design-1.png",
			Prompt:   "retro cherries",
		},
	}); err != nil {
		t.Fatalf("replace designs: %v", err)
	}

	designs, err := repo.ListSessionDesigns(ctx, session.ID)
	if err != nil {
		t.Fatalf("list session designs: %v", err)
	}
	if len(designs) != 1 {
		t.Fatalf("design count = %d, want 1", len(designs))
	}
	if designs[0].SessionID != session.ID {
		t.Fatalf("design session id = %q, want %q", designs[0].SessionID, session.ID)
	}

	items, err := repo.ListGalleryItems(ctx, 10)
	if err != nil {
		t.Fatalf("list gallery items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("gallery item count = %d, want 1", len(items))
	}
	if items[0].SessionID != session.ID || items[0].DesignID != "design-1" {
		t.Fatalf("gallery item = %#v, want linked session/design", items[0])
	}
}

func TestGormRepositoryReplaceDesignsDedupesDuplicateIDs(t *testing.T) {
	t.Cleanup(listingkit.SetOwnerScopeRequiredForTesting(false))

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SheinStudioSession{}, &listingkit.SheinStudioDesign{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewGormRepository(db)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	session := &listingkit.SheinStudioSession{
		ID:           "batch-dedup",
		TenantID:     "tenant-a",
		UserID:       "user-a",
		SelectionKey: "selection-a",
		Status:       listingkit.SheinStudioSessionStatusReviewing,
		SavedAsBatch: true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	err = repo.ReplaceDesigns(ctx, session.ID, []string{"design-1"}, []listingkit.SheinStudioDesign{
		{
			ID:        "design-1",
			SessionID: session.ID,
			ImageURL:  "https://oss.example.com/design-1-a.png",
			Prompt:    "original prompt",
		},
		{
			ID:        "design-1",
			SessionID: session.ID,
			ImageURL:  "https://oss.example.com/design-1-b.png",
			Prompt:    "updated prompt",
		},
	})
	if err != nil {
		t.Fatalf("replace designs: %v", err)
	}

	designs, err := repo.ListSessionDesigns(ctx, session.ID)
	if err != nil {
		t.Fatalf("list session designs: %v", err)
	}
	if len(designs) != 1 {
		t.Fatalf("design count = %d, want 1", len(designs))
	}
	if designs[0].ImageURL != "https://oss.example.com/design-1-b.png" {
		t.Fatalf("image url = %q, want latest duplicate value", designs[0].ImageURL)
	}
	if designs[0].Prompt != "updated prompt" {
		t.Fatalf("prompt = %q, want latest duplicate value", designs[0].Prompt)
	}
}

func TestGormRepositoryUpsertDesignsPreservesExistingSessionDesigns(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SheinStudioSession{}, &listingkit.SheinStudioDesign{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewGormRepository(db)
	baseCtx := listingkit.WithTenantID(context.Background(), "tenant-upsert-designs")
	ctx := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{
		TenantID: "tenant-upsert-designs",
		UserID:   "user-upsert-designs",
	})

	session := &listingkit.SheinStudioSession{
		ID:           "session-upsert-designs",
		UserID:       "user-upsert-designs",
		SelectionKey: "selection-upsert-designs",
		Status:       listingkit.SheinStudioSessionStatusReviewing,
		Selection:    listingkit.SheinStudioSelectionSnapshot{VariantID: 3003},
	}
	if err := repo.CreateSession(baseCtx, session); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := repo.ReplaceDesigns(baseCtx, session.ID, []string{"design-1"}, []listingkit.SheinStudioDesign{
		{
			ID:       "design-1",
			ImageURL: "https://example.com/design-1.png",
			Prompt:   "first",
		},
	}); err != nil {
		t.Fatalf("ReplaceDesigns() seed error = %v", err)
	}

	if err := repo.UpsertDesigns(ctx, session.ID, []string{"design-1", "design-2"}, []listingkit.SheinStudioDesign{
		{
			ID:       "design-2",
			ImageURL: "https://example.com/design-2.png",
			Prompt:   "second",
		},
	}); err != nil {
		t.Fatalf("UpsertDesigns() error = %v", err)
	}

	designs, err := repo.ListSessionDesigns(ctx, session.ID)
	if err != nil {
		t.Fatalf("ListSessionDesigns() error = %v", err)
	}
	if len(designs) != 2 {
		t.Fatalf("design count = %d, want 2", len(designs))
	}
}
