package listingkit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

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
	ctx := tenantctx.WithTenantID(context.Background(), "227")

	response, err := svc.UploadImages(ctx, &UploadImagesRequest{Files: []ImageUploadInput{{Filename: "a.webp", Data: validWebPData(t)}}})
	if err != nil {
		t.Fatalf("UploadImages() error = %v", err)
	}
	uploadID := strings.TrimPrefix(response.ImageURLs[0], "/api/v1/listing-kits/uploads/files/")

	record, err := metadataRepo.GetUploadedImage(ctx, uploadID)
	if err != nil {
		t.Fatalf("GetUploadedImage() error = %v", err)
	}
	if record.TenantID != "227" || record.UploadID != uploadID || !strings.HasPrefix(record.StorageKey, "listingkit/tenants/227/uploads/") || record.Size != int64(len(validWebPData(t))) {
		t.Fatalf("record = %#v", record)
	}
	if record.PublicURL != "" {
		t.Fatalf("public url = %q, want empty", record.PublicURL)
	}
}

func TestUploadImagesRejectsInvalidImageBeforeStorage(t *testing.T) {
	t.Parallel()
	store := &stubMetadataImageUploadStore{saveResult: &StoredUploadedImage{Key: "listingkit/tenants/1/uploads/id.jpg"}}
	svc := seedSupportDeps(&service{studioDeps: studioDependencies{uploadStore: store}}, supportDependencySeed{uploadedImageRepository: NewMemUploadedImageRepository()})

	ctx := tenantctx.WithTenantID(context.Background(), "227")
	_, err := svc.UploadImages(ctx, &UploadImagesRequest{Files: []ImageUploadInput{{Filename: "not-an-image.jpg", Data: []byte("not an image")}}})
	if err == nil || !strings.Contains(err.Error(), "invalid image") {
		t.Fatalf("UploadImages() error = %v, want invalid image", err)
	}
	if store.saveCalls != 0 {
		t.Fatalf("store save calls = %d, want 0", store.saveCalls)
	}
}

func TestUploadImagesUsesOpaqueIDAndTenantScopedStorageKey(t *testing.T) {
	t.Parallel()
	metadataRepo := NewMemUploadedImageRepository()
	store := &stubMetadataImageUploadStore{saveResult: &StoredUploadedImage{Key: "legacy/upload.webp", Filename: "upload.webp"}}
	svc := seedSupportDeps(&service{studioDeps: studioDependencies{uploadStore: store}}, supportDependencySeed{uploadedImageRepository: metadataRepo})
	ctx := tenantctx.WithTenantID(context.Background(), "227")

	response, err := svc.UploadImages(ctx, &UploadImagesRequest{Files: []ImageUploadInput{{Filename: "shirt.jpg", Data: validWebPData(t)}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ImageURLs) != 1 {
		t.Fatalf("image URLs = %#v", response.ImageURLs)
	}
	uploadID := strings.TrimPrefix(response.ImageURLs[0], "/api/v1/listing-kits/uploads/files/")
	if _, err := uuid.Parse(uploadID); err != nil {
		t.Fatalf("response upload ID = %q: %v", uploadID, err)
	}
	wantKey := "listingkit/tenants/227/uploads/" + uploadID + ".webp"
	if store.savedKey != wantKey {
		t.Fatalf("storage key = %q, want %q", store.savedKey, wantKey)
	}
	record, err := metadataRepo.GetUploadedImage(ctx, uploadID)
	if err != nil {
		t.Fatal(err)
	}
	if record.StorageKey != wantKey || record.PublicURL != "" {
		t.Fatalf("record = %#v", record)
	}
}

func TestUploadImagesDeletesObjectWhenMetadataSaveFails(t *testing.T) {
	t.Parallel()
	store := &stubMetadataImageUploadStore{}
	repo := &failingUploadedImageRepository{MemUploadedImageRepository: NewMemUploadedImageRepository(), saveErr: errors.New("db down")}
	svc := seedSupportDeps(&service{studioDeps: studioDependencies{uploadStore: store}}, supportDependencySeed{uploadedImageRepository: repo})

	_, err := svc.UploadImages(tenantctx.WithTenantID(context.Background(), "227"), &UploadImagesRequest{Files: []ImageUploadInput{{Filename: "shirt.webp", Data: validWebPData(t)}}})
	if err == nil || !strings.Contains(err.Error(), "save uploaded image metadata") {
		t.Fatalf("UploadImages() error = %v, want metadata failure", err)
	}
	if store.deletedKey != store.savedKey || store.deletedKey == "" {
		t.Fatalf("deleted key = %q, saved key = %q", store.deletedKey, store.savedKey)
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

func TestGetUploadedImageDoesNotOpenForeignObject(t *testing.T) {
	t.Parallel()
	metadataRepo := NewMemUploadedImageRepository()
	ownerCtx := tenantctx.WithTenantID(context.Background(), "227")
	uploadID := uuid.NewString()
	if err := metadataRepo.SaveUploadedImage(ownerCtx, &UploadedImageRecord{UploadID: uploadID, StorageKey: "listingkit/tenants/227/uploads/" + uploadID + ".webp"}); err != nil {
		t.Fatal(err)
	}
	store := &stubMetadataImageUploadStore{}
	svc := seedSupportDeps(&service{studioDeps: studioDependencies{uploadStore: store}}, supportDependencySeed{uploadedImageRepository: metadataRepo})

	_, err := svc.GetUploadedImage(tenantctx.WithTenantID(context.Background(), "202"), uploadID)
	if !errors.Is(err, ErrUploadedImageNotFound) {
		t.Fatalf("GetUploadedImage() error = %v, want %v", err, ErrUploadedImageNotFound)
	}
	if store.openCalls != 0 {
		t.Fatalf("store open calls = %d, want 0", store.openCalls)
	}
}

func TestDeleteUploadedImageIsIdempotent(t *testing.T) {
	t.Parallel()
	metadataRepo := NewMemUploadedImageRepository()
	ctx := tenantctx.WithTenantID(context.Background(), "227")
	uploadID := uuid.NewString()
	storageKey := "listingkit/tenants/227/uploads/" + uploadID + ".webp"
	if err := metadataRepo.SaveUploadedImage(ctx, &UploadedImageRecord{UploadID: uploadID, StorageKey: storageKey, Size: 3}); err != nil {
		t.Fatal(err)
	}
	store := &stubMetadataImageUploadStore{}
	svc := seedSupportDeps(&service{studioDeps: studioDependencies{uploadStore: store}}, supportDependencySeed{uploadedImageRepository: metadataRepo})

	first, err := svc.DeleteUploadedImage(ctx, uploadID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.DeleteUploadedImage(ctx, uploadID)
	if err != nil {
		t.Fatal(err)
	}
	if first.AlreadyDeleted || !second.AlreadyDeleted || store.deleteCalls != 1 {
		t.Fatalf("first=%#v second=%#v deleteCalls=%d", first, second, store.deleteCalls)
	}
}

type stubMetadataImageUploadStore struct {
	saveResult  *StoredUploadedImage
	deletedKey  string
	deleteCalls int
	saveCalls   int
	savedKey    string
	openCalls   int
}

type failingUploadedImageRepository struct {
	*MemUploadedImageRepository
	saveErr error
}

func (r *failingUploadedImageRepository) SaveUploadedImage(context.Context, *UploadedImageRecord) error {
	return r.saveErr
}

func (s *stubMetadataImageUploadStore) Save(context.Context, *ImageUploadInput) (*StoredUploadedImage, error) {
	s.saveCalls++
	return s.saveResult, nil
}

func (s *stubMetadataImageUploadStore) SaveWithKey(_ context.Context, key string, input *ImageUploadInput) (*StoredUploadedImage, error) {
	s.saveCalls++
	s.savedKey = key
	return &StoredUploadedImage{
		Key:          key,
		Filename:     input.Filename,
		ContentType:  input.ContentType,
		Size:         int64(len(input.Data)),
		OriginalName: input.Filename,
	}, nil
}

func (s *stubMetadataImageUploadStore) Open(_ context.Context, key string) (*StoredUploadedImage, error) {
	s.openCalls++
	return &StoredUploadedImage{Key: key, Size: 3}, nil
}

func (s *stubMetadataImageUploadStore) Delete(_ context.Context, key string) error {
	s.deletedKey = key
	s.deleteCalls++
	return nil
}
