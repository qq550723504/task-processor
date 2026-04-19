package listingkit

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func (s *service) UploadImages(ctx context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error) {
	if s.uploadStore == nil {
		return nil, fmt.Errorf("image upload store is not configured")
	}
	if req == nil || len(req.Files) == 0 {
		return nil, fmt.Errorf("invalid request: files are required")
	}

	response := &UploadImagesResponse{
		ImageURLs: make([]string, 0, len(req.Files)),
	}
	for _, file := range req.Files {
		stored, err := s.uploadStore.Save(ctx, &file)
		if err != nil {
			return nil, fmt.Errorf("save uploaded image: %w", err)
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
	if s.uploadStore == nil {
		return nil, ErrUploadedImageNotFound
	}

	stored, err := s.uploadStore.Open(ctx, key)
	if err != nil {
		return nil, err
	}

	data := stored.Data
	if len(data) == 0 {
		var err error
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

func buildUploadedImagePath(key string) string {
	trimmedKey := strings.TrimLeft(key, "/")
	return "/api/v1/listing-kits/uploads/files/" + trimmedKey
}
