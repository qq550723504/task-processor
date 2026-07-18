package listingkit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"

	"task-processor/internal/tenantbridge"
)

func (s *service) UploadImages(ctx context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error) {
	uploadStore := resolveStudioUploadStore(s)
	if uploadStore == nil {
		return nil, fmt.Errorf("image upload store is not configured")
	}
	if req == nil || len(req.Files) == 0 {
		return nil, fmt.Errorf("invalid request: files are required")
	}
	keyedUploadStore, ok := uploadStore.(KeyedImageUploadStore)
	if !ok {
		return uploadImagesWithLegacyStore(ctx, req, uploadStore)
	}
	uploadedImageRepo := resolveUploadedImageRepository(s)
	if uploadedImageRepo == nil {
		return nil, fmt.Errorf("uploaded image repository is not configured")
	}
	legacyTenantID, err := tenantbridge.ResolveLegacyTenantID(ctx, TenantIDFromContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("resolve legacy tenant id: %w", err)
	}
	if legacyTenantID <= 0 {
		return nil, fmt.Errorf("resolve legacy tenant id: no legacy tenant id")
	}

	response := &UploadImagesResponse{
		ImageURLs: make([]string, 0, len(req.Files)),
	}
	for _, file := range req.Files {
		validated, err := validateUploadedImage(file)
		if err != nil {
			return nil, fmt.Errorf("invalid image upload: %w", err)
		}
		file.ContentType = validated.ContentType
		uploadID := uuid.NewString()
		storageKey := fmt.Sprintf("listingkit/tenants/%d/uploads/%s%s", legacyTenantID, uploadID, validated.Extension)
		stored, err := keyedUploadStore.SaveWithKey(ctx, storageKey, &file)
		if err != nil {
			return nil, fmt.Errorf("save uploaded image: %w", err)
		}
		if err := uploadedImageRepo.SaveUploadedImage(ctx, &UploadedImageRecord{
			Key:          stored.Key,
			UploadID:     uploadID,
			StorageKey:   stored.Key,
			Filename:     stored.Filename,
			ContentType:  stored.ContentType,
			Size:         stored.Size,
			OriginalName: stored.OriginalName,
		}); err != nil {
			cleanupErr := uploadStore.Delete(ctx, stored.Key)
			if cleanupErr != nil && !errors.Is(cleanupErr, ErrUploadedImageNotFound) {
				return nil, errors.Join(fmt.Errorf("save uploaded image metadata: %w", err), fmt.Errorf("delete uploaded image after metadata failure: %w", cleanupErr))
			}
			return nil, fmt.Errorf("save uploaded image metadata: %w", err)
		}
		response.ImageURLs = append(response.ImageURLs, buildUploadedImagePath(uploadID))
	}

	return response, nil
}

func uploadImagesWithLegacyStore(ctx context.Context, req *UploadImagesRequest, uploadStore ImageUploadStore) (*UploadImagesResponse, error) {
	response := &UploadImagesResponse{ImageURLs: make([]string, 0, len(req.Files))}
	for _, file := range req.Files {
		stored, err := uploadStore.Save(ctx, &file)
		if err != nil {
			return nil, fmt.Errorf("save uploaded image: %w", err)
		}
		imageURL := strings.TrimSpace(stored.PublicURL)
		if imageURL == "" {
			imageURL = buildUploadedImagePath(stored.Key)
		}
		response.ImageURLs = append(response.ImageURLs, imageURL)
	}
	return response, nil
}

func (s *service) GetUploadedImage(ctx context.Context, uploadID string) (*UploadedImageFile, error) {
	uploadStore := resolveStudioUploadStore(s)
	if uploadStore == nil {
		return nil, ErrUploadedImageNotFound
	}
	uploadedImageRepo := resolveUploadedImageRepository(s)
	if uploadedImageRepo == nil {
		return nil, ErrUploadedImageNotFound
	}
	record, err := uploadedImageRepo.GetUploadedImage(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	storageKey := record.StorageKey
	if storageKey == "" {
		storageKey = record.Key
	}

	stored, err := uploadStore.Open(ctx, storageKey)
	if err != nil {
		return nil, err
	}

	data := stored.Data
	if len(data) == 0 {
		data, err = os.ReadFile(stored.Path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, ErrUploadedImageNotFound
			}
			return nil, fmt.Errorf("read uploaded image: %w", err)
		}
	}

	validated, err := validateUploadedImage(ImageUploadInput{Data: data})
	if err != nil {
		return nil, ErrUploadedImageNotFound
	}
	filename := record.Filename
	if filename == "" {
		filename = stored.Filename
	}
	return &UploadedImageFile{
		Filename:    filename,
		ContentType: validated.ContentType,
		Data:        data,
	}, nil
}

func (s *service) DeleteUploadedImage(ctx context.Context, uploadID string) (*DeletedUploadedImage, error) {
	uploadStore := resolveStudioUploadStore(s)
	if uploadStore == nil {
		return nil, ErrUploadedImageNotFound
	}
	uploadedImageRepo := resolveUploadedImageRepository(s)
	if uploadedImageRepo == nil {
		return nil, ErrUploadedImageNotFound
	}
	claim, err := uploadedImageRepo.ClaimUploadedImageDeletion(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if claim.AlreadyDeleted {
		return &DeletedUploadedImage{Key: uploadID, Size: claim.Record.Size, AlreadyDeleted: true}, nil
	}
	storageKey := claim.Record.StorageKey
	if storageKey == "" {
		storageKey = claim.Record.Key
	}
	if err := uploadStore.Delete(ctx, storageKey); err != nil && !errors.Is(err, ErrUploadedImageNotFound) {
		_ = uploadedImageRepo.ReleaseUploadedImageDeletion(ctx, uploadID)
		return nil, err
	}
	if err := uploadedImageRepo.CompleteUploadedImageDeletion(ctx, uploadID); err != nil {
		return nil, err
	}
	return &DeletedUploadedImage{Key: uploadID, Size: claim.Record.Size}, nil
}
