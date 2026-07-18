package listingkit

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/shared/tenantctx"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestGormUploadedImageRepositoryHidesForeignAndMalformedUploadIDs(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrateUploadedImageRepository(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := NewGormUploadedImageRepository(db)
	ctxA := tenantctx.WithTenantID(context.Background(), "101")
	ctxB := tenantctx.WithTenantID(context.Background(), "202")
	const uploadID = "b4b7d3a5-5d06-4f2c-a13e-2735e9e963d5"
	if err := repo.SaveUploadedImage(ctxA, &UploadedImageRecord{
		UploadID:   uploadID,
		StorageKey: "listingkit/tenants/101/uploads/b4b7d3a5-5d06-4f2c-a13e-2735e9e963d5.png",
	}); err != nil {
		t.Fatalf("save tenant a: %v", err)
	}
	if _, err := repo.GetUploadedImage(ctxB, uploadID); !errors.Is(err, ErrUploadedImageNotFound) {
		t.Fatalf("foreign lookup = %v, want ErrUploadedImageNotFound", err)
	}
	if _, err := repo.GetUploadedImage(ctxA, "../secret"); !errors.Is(err, ErrUploadedImageNotFound) {
		t.Fatalf("malformed lookup = %v, want ErrUploadedImageNotFound", err)
	}
}

func TestGormUploadedImageRepositoryClaimsOnlyOneDeletion(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrateUploadedImageRepository(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := NewGormUploadedImageRepository(db)
	ctx := tenantctx.WithTenantID(context.Background(), "101")
	const uploadID = "7e3fde24-728e-4a6e-a9e4-563aee5df1c9"
	if err := repo.SaveUploadedImage(ctx, &UploadedImageRecord{UploadID: uploadID, StorageKey: "listingkit/tenants/101/uploads/7e3fde24-728e-4a6e-a9e4-563aee5df1c9.png"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	claim, err := repo.ClaimUploadedImageDeletion(ctx, uploadID)
	if err != nil || !claim.Claimed || claim.AlreadyDeleted {
		t.Fatalf("first claim = %#v, %v", claim, err)
	}
	if err := repo.CompleteUploadedImageDeletion(ctx, uploadID); err != nil {
		t.Fatalf("complete deletion: %v", err)
	}
	again, err := repo.ClaimUploadedImageDeletion(ctx, uploadID)
	if err != nil || again.Claimed || !again.AlreadyDeleted {
		t.Fatalf("second claim = %#v, %v", again, err)
	}
}

func TestGormUploadedImageRepositoryScopesByTenantAndSoftDeletes(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrateUploadedImageRepository(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := NewGormUploadedImageRepository(db)
	ctxA := tenantctx.WithTenantID(context.Background(), "tenant-a")
	ctxB := tenantctx.WithTenantID(context.Background(), "tenant-b")

	if err := repo.SaveUploadedImage(ctxA, &UploadedImageRecord{Key: "same.jpg", Size: 3}); err != nil {
		t.Fatalf("save tenant a: %v", err)
	}
	if err := repo.SaveUploadedImage(ctxB, &UploadedImageRecord{Key: "same.jpg", Size: 7}); err != nil {
		t.Fatalf("save tenant b: %v", err)
	}

	recordA, err := repo.GetUploadedImage(ctxA, "same.jpg")
	if err != nil {
		t.Fatalf("get tenant a: %v", err)
	}
	if recordA.Size != 3 {
		t.Fatalf("tenant a size = %d, want 3", recordA.Size)
	}
	deleted, err := repo.MarkUploadedImageDeleted(ctxA, "same.jpg")
	if err != nil {
		t.Fatalf("delete tenant a: %v", err)
	}
	if deleted.DeletedAt == nil {
		t.Fatal("deleted_at = nil")
	}
	if _, err := repo.GetUploadedImage(ctxA, "same.jpg"); err != ErrUploadedImageNotFound {
		t.Fatalf("get deleted tenant a error = %v, want ErrUploadedImageNotFound", err)
	}
	recordB, err := repo.GetUploadedImage(ctxB, "same.jpg")
	if err != nil {
		t.Fatalf("get tenant b: %v", err)
	}
	if recordB.Size != 7 {
		t.Fatalf("tenant b size = %d, want 7", recordB.Size)
	}
}
