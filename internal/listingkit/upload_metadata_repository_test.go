package listingkit

import (
	"context"
	"testing"

	"task-processor/internal/listingkit/tenantctx"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

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
