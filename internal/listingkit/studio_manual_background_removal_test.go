package listingkit

import (
	"context"
	"errors"
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

func TestApplyManualStudioBatchDesignBackgroundRemovalDoesNotOverwriteAutomaticClaim(t *testing.T) {
	t.Parallel()

	baseRepo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	if err := baseRepo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
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
		BackgroundRemovalStatus: StudioBackgroundRemovalStatusNotRequested,
		CreatedAt:               now,
		UpdatedAt:               now,
	}}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	repo := &automaticClaimBeforeManualWriteRepository{StudioBatchRepository: baseRepo}
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{repo: repo})

	_, err := svc.ApplyManualStudioBatchDesignBackgroundRemoval(
		ctx,
		"batch-1",
		"design-1",
		"https://cdn.example.test/manual.png",
	)
	if !errors.Is(err, ErrStudioBatchActionValidation) {
		t.Fatalf("ApplyManualStudioBatchDesignBackgroundRemoval() error = %v, want ErrStudioBatchActionValidation", err)
	}

	detail, getErr := baseRepo.GetStudioBatchDetail(ctx, "batch-1")
	if getErr != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", getErr)
	}
	design := detail.DesignsByItem["item-1"][0]
	if design.BackgroundRemovalStatus != StudioBackgroundRemovalStatusPending || design.ImageURL != "https://cdn.example.test/generated.png" {
		t.Fatalf("design after race = %+v, want pending automatic claim with original image retained", design)
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

func TestStudioBatchManualBackgroundRemovalRetainsCommittedUploadWhenDetailReadFails(t *testing.T) {
	t.Parallel()

	baseRepo := NewMemStudioBatchRepository()
	ctx := tenantctx.WithTenantID(context.Background(), "227")
	now := time.Now().UTC()
	if err := baseRepo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
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
		BackgroundRemovalStatus: StudioBackgroundRemovalStatusNotRequested,
		CreatedAt:               now,
		UpdatedAt:               now,
	}}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	detailReadErr := errors.New("detail read unavailable")
	repo := &failSecondDetailReadStudioBatchRepository{
		StudioBatchRepository: baseRepo,
		err:                   detailReadErr,
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

	_, err := svc.ApplyManualStudioBatchDesignBackgroundRemoval(ctx, "batch-1", "design-1", &ImageUploadInput{
		Filename: "manual.png",
		Data:     studioTestOpaquePNG(t),
	})
	if !errors.Is(err, detailReadErr) {
		t.Fatalf("ApplyManualStudioBatchDesignBackgroundRemoval() error = %v, want detail read error", err)
	}
	if store.deleteCalls != 0 {
		t.Fatalf("upload delete calls = %d, want 0 after committed manual write", store.deleteCalls)
	}
	detail, getErr := baseRepo.GetStudioBatchDetail(ctx, "batch-1")
	if getErr != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", getErr)
	}
	if got := detail.DesignsByItem["item-1"][0].ImageURL; got == "" || got == "https://cdn.example.test/generated.png" {
		t.Fatalf("committed design image URL = %q, want retained uploaded image URL", got)
	}
}

func TestStudioBatchManualBackgroundRemovalClassifiesTruncatedPNGAsValidation(t *testing.T) {
	t.Parallel()

	ctx := tenantctx.WithTenantID(context.Background(), "227")
	store := &stubMetadataImageUploadStore{}
	svc := seedSupportDeps(&service{
		studioDeps: studioDependencies{uploadStore: store},
	}, supportDependencySeed{uploadedImageRepository: NewMemUploadedImageRepository()})
	pngData := studioTestOpaquePNG(t)

	_, err := svc.ApplyManualStudioBatchDesignBackgroundRemoval(ctx, "batch-1", "design-1", &ImageUploadInput{
		Filename: "truncated.png",
		Data:     pngData[:len(pngData)/2],
	})
	if !errors.Is(err, ErrStudioBatchActionValidation) {
		t.Fatalf("ApplyManualStudioBatchDesignBackgroundRemoval() error = %v, want ErrStudioBatchActionValidation", err)
	}
	if store.saveCalls != 0 {
		t.Fatalf("upload save calls = %d, want 0", store.saveCalls)
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
	saveResult  *StoredUploadedImage
	savedInput  *ImageUploadInput
	saveCalls   int
	deletedKey  string
	deleteCalls int
}

type automaticClaimBeforeManualWriteRepository struct {
	StudioBatchRepository
	claimed bool
}

func (r *automaticClaimBeforeManualWriteRepository) claimAutomatic(ctx context.Context, design *StudioMaterializedDesignRecord) error {
	if r.claimed {
		return nil
	}
	r.claimed = true
	pending := *design
	pending.BackgroundRemovalStatus = StudioBackgroundRemovalStatusPending
	pending.BackgroundRemovalModel = ""
	pending.BackgroundRemovalError = ""
	_, err := r.StudioBatchRepository.(studioBackgroundRemovalRepository).ClaimStudioMaterializedDesignBackgroundRemoval(ctx, &pending)
	return err
}

func (r *automaticClaimBeforeManualWriteRepository) ClaimStudioMaterializedDesignBackgroundRemoval(ctx context.Context, design *StudioMaterializedDesignRecord) (bool, error) {
	return r.StudioBatchRepository.(studioBackgroundRemovalRepository).ClaimStudioMaterializedDesignBackgroundRemoval(ctx, design)
}

func (r *automaticClaimBeforeManualWriteRepository) UpdateStudioMaterializedDesignBackgroundRemoval(ctx context.Context, design *StudioMaterializedDesignRecord) error {
	if err := r.claimAutomatic(ctx, design); err != nil {
		return err
	}
	return r.StudioBatchRepository.(studioBackgroundRemovalRepository).UpdateStudioMaterializedDesignBackgroundRemoval(ctx, design)
}

func (r *automaticClaimBeforeManualWriteRepository) ApplyManualStudioMaterializedDesignBackgroundRemoval(ctx context.Context, design *StudioMaterializedDesignRecord) (bool, error) {
	if err := r.claimAutomatic(ctx, design); err != nil {
		return false, err
	}
	return r.StudioBatchRepository.(manualBackgroundRemovalApplier).ApplyManualStudioMaterializedDesignBackgroundRemoval(ctx, design)
}

type failSecondDetailReadStudioBatchRepository struct {
	StudioBatchRepository
	reads int
	err   error
}

func (r *failSecondDetailReadStudioBatchRepository) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetailGraph, error) {
	r.reads++
	if r.reads == 2 {
		return nil, r.err
	}
	return r.StudioBatchRepository.GetStudioBatchDetail(ctx, batchID)
}

func (r *failSecondDetailReadStudioBatchRepository) ClaimStudioMaterializedDesignBackgroundRemoval(ctx context.Context, design *StudioMaterializedDesignRecord) (bool, error) {
	return r.StudioBatchRepository.(studioBackgroundRemovalRepository).ClaimStudioMaterializedDesignBackgroundRemoval(ctx, design)
}

func (r *failSecondDetailReadStudioBatchRepository) UpdateStudioMaterializedDesignBackgroundRemoval(ctx context.Context, design *StudioMaterializedDesignRecord) error {
	return r.StudioBatchRepository.(studioBackgroundRemovalRepository).UpdateStudioMaterializedDesignBackgroundRemoval(ctx, design)
}

func (r *failSecondDetailReadStudioBatchRepository) ApplyManualStudioMaterializedDesignBackgroundRemoval(ctx context.Context, design *StudioMaterializedDesignRecord) (bool, error) {
	return r.StudioBatchRepository.(manualBackgroundRemovalApplier).ApplyManualStudioMaterializedDesignBackgroundRemoval(ctx, design)
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
