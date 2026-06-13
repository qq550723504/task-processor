package listingkit

import (
	"context"
	"testing"

	"task-processor/internal/listingkit/tenantctx"
)

func TestUploadImagesRecordsUploadedImageMetadata(t *testing.T) {
	t.Parallel()

	metadataRepo := NewMemUploadedImageRepository()
	svc := &service{
		mirrors: serviceDependencyMirrors{
			uploadStore: &stubMetadataImageUploadStore{
				saveResult: &StoredUploadedImage{
					Key:         "20260515/a.jpg",
					Filename:    "a.jpg",
					PublicURL:   "https://cdn.example.com/20260515/a.jpg",
					ContentType: "image/jpeg",
					Size:        3,
				},
			},
			uploadedImageRepo: metadataRepo,
		},
	}
	ctx := tenantctx.WithTenantID(context.Background(), "tenant-a")

	if _, err := svc.UploadImages(ctx, &UploadImagesRequest{Files: []ImageUploadInput{{Filename: "a.jpg", Data: []byte{1, 2, 3}}}}); err != nil {
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
	svc := &service{mirrors: serviceDependencyMirrors{uploadStore: store, uploadedImageRepo: metadataRepo}}

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
}

func (s *stubMetadataImageUploadStore) Save(context.Context, *ImageUploadInput) (*StoredUploadedImage, error) {
	return s.saveResult, nil
}

func (s *stubMetadataImageUploadStore) Open(_ context.Context, key string) (*StoredUploadedImage, error) {
	return &StoredUploadedImage{Key: key, Size: 3}, nil
}

func (s *stubMetadataImageUploadStore) Delete(_ context.Context, key string) error {
	s.deletedKey = key
	return nil
}
