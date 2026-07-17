package listingkit

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func (s *service) UploadImages(ctx context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error) {
	uploadStore := resolveStudioUploadStore(s)
	if uploadStore == nil {
		return nil, fmt.Errorf("image upload store is not configured")
	}
	if req == nil || len(req.Files) == 0 {
		return nil, fmt.Errorf("invalid request: files are required")
	}

	response := &UploadImagesResponse{
		ImageURLs: make([]string, 0, len(req.Files)),
	}
	uploadedImageRepo := resolveUploadedImageRepository(s)
	for _, file := range req.Files {
		validated, err := validateUploadedImage(file)
		if err != nil {
			return nil, fmt.Errorf("invalid image upload: %w", err)
		}
		file.ContentType = validated.ContentType
		stored, err := uploadStore.Save(ctx, &file)
		if err != nil {
			return nil, fmt.Errorf("save uploaded image: %w", err)
		}
		if uploadedImageRepo != nil {
			_ = uploadedImageRepo.SaveUploadedImage(ctx, &UploadedImageRecord{
				Key:          stored.Key,
				Filename:     stored.Filename,
				PublicURL:    stored.PublicURL,
				ContentType:  stored.ContentType,
				Size:         stored.Size,
				OriginalName: stored.OriginalName,
			})
		}
		publicURL := strings.TrimSpace(stored.PublicURL)
		if publicURL == "" {
			publicURL = buildUploadedImagePath(stored.Key)
		}
		response.ImageURLs = append(response.ImageURLs, publicURL)
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
