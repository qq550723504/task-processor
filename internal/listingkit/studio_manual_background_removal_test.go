package listingkit

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"

	"task-processor/internal/shared/tenantctx"
)

func TestApplyManualStudioBatchDesignBackgroundRemovalPersistsFirstManualResult(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	if err := repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:                        "batch-1",
		Status:                    StudioBatchStatusReviewReady,
		TransparentBackground:     true,
		TransparentBackgroundMode: StudioTransparencyModeNone,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}, []StudioBatchItemRecord{{
		ID:        "item-1",
		BatchID:   "batch-1",
		Status:    StudioBatchItemStatusReviewReady,
		CreatedAt: now,
		UpdatedAt: now,
	}}, nil, []StudioMaterializedDesignRecord{{
		ID:                        "design-1",
		BatchID:                   "batch-1",
		ItemID:                    "item-1",
		ImageURL:                  "https://cdn.example.test/generated.png",
		TransparentBackgroundMode: StudioTransparencyModeNone,
		BackgroundRemovalStatus:   StudioBackgroundRemovalStatusNotRequested,
		ReviewStatus:              StudioMaterializedDesignReviewStatusApproved,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{repo: repo})

	detail, err := svc.ApplyManualStudioBatchDesignBackgroundRemoval(
		ctx,
		"batch-1",
		"design-1",
		"https://cdn.example.test/manual.png",
	)
	if err != nil {
		t.Fatalf("ApplyManualStudioBatchDesignBackgroundRemoval() error = %v", err)
	}

	design := detail.Items[0].Designs[0]
	if design.OriginalImageURL != "https://cdn.example.test/generated.png" ||
		design.ImageURL != "https://cdn.example.test/manual.png" ||
		design.TransparentBackgroundMode != StudioTransparencyModeRemoval ||
		design.BackgroundRemovalStatus != StudioBackgroundRemovalStatusSucceeded ||
		design.ReviewStatus != StudioMaterializedDesignReviewStatusApproved {
		t.Fatalf("manual result design = %+v, want original snapshot, manual image, removal mode, succeeded status, and preserved review state", design)
	}
}

func TestStudioBatchManualBackgroundRemovalUploadsPNGAndAppliesReturnedURL(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := tenantctx.WithTenantID(context.Background(), "227")
	now := time.Now().UTC()
	if err := repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:        "batch-1",
		Status:    StudioBatchStatusReviewReady,
		CreatedAt: now,
		UpdatedAt: now,
	}, []StudioBatchItemRecord{{
		ID:        "item-1",
		BatchID:   "batch-1",
		Status:    StudioBatchItemStatusReviewReady,
		CreatedAt: now,
		UpdatedAt: now,
	}}, nil, []StudioMaterializedDesignRecord{{
		ID:                      "design-1",
		BatchID:                 "batch-1",
		ItemID:                  "item-1",
		ImageURL:                "https://cdn.example.test/generated.png",
		ReviewStatus:            StudioMaterializedDesignReviewStatusApproved,
		CreatedAt:               now,
		UpdatedAt:               now,
		BackgroundRemovalStatus: StudioBackgroundRemovalStatusNotRequested,
	}}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	store := &stubMetadataImageUploadStore{saveResult: &StoredUploadedImage{PublicURL: "https://cdn.example.test/manual-upload.png"}}
	svc := seedSupportDeps(&service{
		studioDeps: studioDependencies{uploadStore: store},
		studio: studioCollaborators{
			batchGroup: taskStudioBatchCollaborators{
				batch: newTaskStudioBatchService(taskStudioBatchServiceConfig{repo: repo}),
			},
		},
	}, supportDependencySeed{uploadedImageRepository: NewMemUploadedImageRepository()})

	detail, err := svc.ApplyManualStudioBatchDesignBackgroundRemoval(ctx, "batch-1", "design-1", &ImageUploadInput{
		Filename: "manual.png",
		Data:     studioTestOpaquePNG(t),
	})
	if err != nil {
		t.Fatalf("ApplyManualStudioBatchDesignBackgroundRemoval() error = %v", err)
	}
	if store.saveCalls != 1 {
		t.Fatalf("upload save calls = %d, want 1", store.saveCalls)
	}
	if store.deleteCalls != 0 {
		t.Fatalf("upload delete calls = %d, want 0", store.deleteCalls)
	}
	design := detail.Items[0].Designs[0]
	if design.ImageURL == "https://cdn.example.test/generated.png" || design.ImageURL == "" {
		t.Fatalf("design image URL = %q, want uploaded manual URL", design.ImageURL)
	}
}

func TestStudioBatchManualBackgroundRemovalDeletesNewUploadWhenApplyFails(t *testing.T) {
	t.Parallel()

	ctx := tenantctx.WithTenantID(context.Background(), "227")
	store := &stubMetadataImageUploadStore{saveResult: &StoredUploadedImage{PublicURL: "https://cdn.example.test/manual-upload.png"}}
	svc := seedSupportDeps(&service{
		studioDeps: studioDependencies{uploadStore: store},
		studio: studioCollaborators{
			batchGroup: taskStudioBatchCollaborators{
				batch: newTaskStudioBatchService(taskStudioBatchServiceConfig{}),
			},
		},
	}, supportDependencySeed{uploadedImageRepository: NewMemUploadedImageRepository()})

	_, err := svc.ApplyManualStudioBatchDesignBackgroundRemoval(ctx, "batch-1", "design-1", &ImageUploadInput{
		Filename: "manual.png",
		Data:     studioTestOpaquePNG(t),
	})
	if err == nil {
		t.Fatal("ApplyManualStudioBatchDesignBackgroundRemoval() error = nil, want batch apply failure")
	}
	if store.saveCalls != 1 {
		t.Fatalf("upload save calls = %d, want 1", store.saveCalls)
	}
	if store.deletedKey == "" || store.deletedKey != store.savedKey {
		t.Fatalf("deleted key = %q, saved key = %q, want cleanup of created upload", store.deletedKey, store.savedKey)
	}
}

func TestStudioBatchManualBackgroundRemovalDeletesLegacyUploadByStoredKeyWhenApplyFails(t *testing.T) {
	t.Parallel()

	ctx := tenantctx.WithTenantID(context.Background(), "227")
	store := &stubLegacyManualUploadStore{
		saveResult: &StoredUploadedImage{
			Key:       "legacy/manual-upload.png",
			PublicURL: "https://cdn.example.test/manual-upload.png",
			Filename:  "manual-upload.png",
		},
	}
	svc := &service{
		studioDeps: studioDependencies{uploadStore: store},
		studio: studioCollaborators{
			batchGroup: taskStudioBatchCollaborators{
				batch: newTaskStudioBatchService(taskStudioBatchServiceConfig{}),
			},
		},
	}

	_, err := svc.ApplyManualStudioBatchDesignBackgroundRemoval(ctx, "batch-1", "design-1", &ImageUploadInput{
		Filename: "manual.png",
		Data:     studioTestOpaquePNG(t),
	})
	if err == nil {
		t.Fatal("ApplyManualStudioBatchDesignBackgroundRemoval() error = nil, want batch apply failure")
	}
	if store.saveCalls != 1 {
		t.Fatalf("legacy upload save calls = %d, want 1", store.saveCalls)
	}
	if store.deletedKey != "legacy/manual-upload.png" {
		t.Fatalf("legacy deleted key = %q, want stored key cleanup", store.deletedKey)
	}
}

func TestStudioBatchManualBackgroundRemovalRejectsJPEGBytesBeforeUpload(t *testing.T) {
	t.Parallel()

	ctx := tenantctx.WithTenantID(context.Background(), "227")
	store := &stubMetadataImageUploadStore{}
	svc := seedSupportDeps(&service{
		studioDeps: studioDependencies{uploadStore: store},
		studio: studioCollaborators{
			batchGroup: taskStudioBatchCollaborators{
				batch: newTaskStudioBatchService(taskStudioBatchServiceConfig{}),
			},
		},
	}, supportDependencySeed{uploadedImageRepository: NewMemUploadedImageRepository()})

	_, err := svc.ApplyManualStudioBatchDesignBackgroundRemoval(ctx, "batch-1", "design-1", &ImageUploadInput{
		Filename: "manual.jpg",
		Data:     studioTestJPEG(t),
	})
	if err == nil {
		t.Fatal("ApplyManualStudioBatchDesignBackgroundRemoval() error = nil, want png validation error")
	}
	if store.saveCalls != 0 {
		t.Fatalf("upload save calls = %d, want 0", store.saveCalls)
	}
	if store.deleteCalls != 0 {
		t.Fatalf("upload delete calls = %d, want 0", store.deleteCalls)
	}
}

func studioTestJPEG(t *testing.T) []byte {
	t.Helper()
	var output bytesBuffer
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	if err := jpeg.Encode(&output, img, nil); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}
	return output.bytes
}

type stubLegacyManualUploadStore struct {
	saveResult *StoredUploadedImage
	savedInput *ImageUploadInput
	saveCalls  int
	deletedKey string
	deleteCalls int
}

func (s *stubLegacyManualUploadStore) Save(_ context.Context, input *ImageUploadInput) (*StoredUploadedImage, error) {
	s.saveCalls++
	if input != nil {
		copyInput := *input
		copyInput.Data = append([]byte(nil), input.Data...)
		s.savedInput = &copyInput
	}
	if s.saveResult != nil {
		result := *s.saveResult
		return &result, nil
	}
	return &StoredUploadedImage{}, nil
}

func (s *stubLegacyManualUploadStore) Open(context.Context, string) (*StoredUploadedImage, error) {
	return nil, ErrUploadedImageNotFound
}

func (s *stubLegacyManualUploadStore) Delete(_ context.Context, key string) error {
	s.deletedKey = key
	s.deleteCalls++
	return nil
}
