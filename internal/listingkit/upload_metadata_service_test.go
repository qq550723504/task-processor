package listingkit

import (
	"context"
	"strings"
	"testing"

	"task-processor/internal/shared/tenantctx"
)

func TestUploadImagesRecordsUploadedImageMetadata(t *testing.T) {
	t.Parallel()

	metadataRepo := NewMemUploadedImageRepository()
	svc := seedSupportDeps(&service{
		studioDeps: studioDependencies{
			uploadStore: &stubMetadataImageUploadStore{
				saveResult: &StoredUploadedImage{
					Key:         "20260515/a.jpg",
					Filename:    "a.jpg",
					PublicURL:   "https://cdn.example.com/20260515/a.jpg",
					ContentType: "image/jpeg",
					Size:        3,
				},
			},
		},
	}, supportDependencySeed{
		uploadedImageRepository: metadataRepo,
	})
	ctx := tenantctx.WithTenantID(context.Background(), "tenant-a")

	if _, err := svc.UploadImages(ctx, &UploadImagesRequest{Files: []ImageUploadInput{{Filename: "a.webp", Data: validWebPData(t)}}}); err != nil {
		t.Fatalf("UploadImages() error = %v", err)
	}

	record, err := metadataRepo.GetUploadedImage(ctx, "20260515/a.jpg")
	if err != nil {
		t.Fatalf("GetUploadedImage() error = %v", err)
	}
	if record.TenantID != "tenant-a" || record.Key != "20260515/a.jpg" || record.Size != 3 {
		t.Fatalf("record = %#v", record)
	}
	if record.PublicURL != "https://cdn.example.com/20260515/a.jpg" {
		t.Fatalf("public url = %q", record.PublicURL)
	}
}

func TestUploadImagesRejectsInvalidImageBeforeStorage(t *testing.T) {
	t.Parallel()
	store := &stubMetadataImageUploadStore{saveResult: &StoredUploadedImage{Key: "listingkit/tenants/1/uploads/id.jpg"}}
	svc := seedSupportDeps(&service{studioDeps: studioDependencies{uploadStore: store}}, supportDependencySeed{})

	_, err := svc.UploadImages(context.Background(), &UploadImagesRequest{Files: []ImageUploadInput{{Filename: "not-an-image.jpg", Data: []byte("not an image")}}})
	if err == nil || !strings.Contains(err.Error(), "invalid image") {
		t.Fatalf("UploadImages() error = %v, want invalid image", err)
	}
	if store.saveCalls != 0 {
		t.Fatalf("store save calls = %d, want 0", store.saveCalls)
	}
}

func TestDeleteUploadedImageUsesMetadataAndMarksRecordDeleted(t *testing.T) {
	t.Parallel()

	metadataRepo := NewMemUploadedImageRepository()
	ctx := tenantctx.WithTenantID(context.Background(), "tenant-a")
	if err := metadataRepo.SaveUploadedImage(ctx, &UploadedImageRecord{
		TenantID:     "tenant-a",
		Key:          "20260515/a.jpg",
		Filename:     "a.jpg",
		ContentType:  "image/jpeg",
		Size:         3,
		OriginalName: "a.jpg",
	}); err != nil {
		t.Fatalf("SaveUploadedImage() error = %v", err)
	}
	store := &stubMetadataImageUploadStore{}
	svc := seedSupportDeps(&service{
		studioDeps: studioDependencies{uploadStore: store},
	}, supportDependencySeed{
		uploadedImageRepository: metadataRepo,
	})

	deleted, err := svc.DeleteUploadedImage(ctx, "20260515/a.jpg")
	if err != nil {
		t.Fatalf("DeleteUploadedImage() error = %v", err)
	}
	if deleted.Size != 3 {
		t.Fatalf("deleted size = %d, want 3", deleted.Size)
	}
	if store.deletedKey != "20260515/a.jpg" {
		t.Fatalf("deleted key = %q", store.deletedKey)
	}
	if _, err := metadataRepo.GetUploadedImage(ctx, "20260515/a.jpg"); err != ErrUploadedImageNotFound {
		t.Fatalf("GetUploadedImage() after delete error = %v, want ErrUploadedImageNotFound", err)
	}
}

type stubMetadataImageUploadStore struct {
	saveResult *StoredUploadedImage
	deletedKey string
	saveCalls  int
}

func (s *stubMetadataImageUploadStore) Save(context.Context, *ImageUploadInput) (*StoredUploadedImage, error) {
	s.saveCalls++
	return s.saveResult, nil
}

func (s *stubMetadataImageUploadStore) Open(_ context.Context, key string) (*StoredUploadedImage, error) {
	return &StoredUploadedImage{Key: key, Size: 3}, nil
}

func (s *stubMetadataImageUploadStore) Delete(_ context.Context, key string) error {
	s.deletedKey = key
	return nil
}
