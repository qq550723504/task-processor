package listingkit

import (
	"context"
	"errors"
	"fmt"
	"os"

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
		return nil, fmt.Errorf("image upload store does not support tenant-scoped keys")
	}
	uploadedImageRepo := resolveUploadedImageRepository(s)
	if uploadedImageRepo == nil {
		return nil, fmt.Errorf("uploaded image repository is not configured")
	}
	legacyTenantID, err := tenantbridge.ResolveLegacyTenantID(ctx, TenantIDFromContext(ctx))
	if err != nil || legacyTenantID <= 0 {
		return nil, fmt.Errorf("resolve legacy tenant id: %w", err)
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

func (s *service) GetUploadedImage(ctx context.Context, key string) (*UploadedImageFile, error) {
	uploadStore := resolveStudioUploadStore(s)
	if uploadStore == nil {
		return nil, ErrUploadedImageNotFound
	}

	stored, err := uploadStore.Open(ctx, key)
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

	return &UploadedImageFile{
		Filename:    stored.Filename,
		ContentType: stored.ContentType,
		Data:        data,
	}, nil
}

func (s *service) DeleteUploadedImage(ctx context.Context, key string) (*DeletedUploadedImage, error) {
	uploadStore := resolveStudioUploadStore(s)
	if uploadStore == nil {
		return nil, ErrUploadedImageNotFound
	}
	var stored *StoredUploadedImage
	uploadedImageRepo := resolveUploadedImageRepository(s)
	if uploadedImageRepo != nil {
		record, err := uploadedImageRepo.GetUploadedImage(ctx, key)
		if err != nil {
			return nil, err
		}
		stored = &StoredUploadedImage{Key: record.Key, Size: record.Size}
	} else {
		var err error
		stored, err = uploadStore.Open(ctx, key)
		if err != nil {
			return nil, err
		}
	}
	if err := uploadStore.Delete(ctx, stored.Key); err != nil {
		return nil, err
	}
	if uploadedImageRepo != nil {
		if _, err := uploadedImageRepo.MarkUploadedImageDeleted(ctx, stored.Key); err != nil {
			return nil, err
		}
	}
	return &DeletedUploadedImage{Key: stored.Key, Size: stored.Size}, nil
}
