package listingkit

import (
	"context"
	"errors"
	"fmt"
)

type fallbackImageUploadStore struct {
	primary  ImageUploadStore
	fallback ImageUploadStore
}

func NewFallbackImageUploadStore(primary, fallback ImageUploadStore) (ImageUploadStore, error) {
	if primary == nil {
		return nil, fmt.Errorf("primary upload store cannot be nil")
	}
	if fallback == nil {
		return primary, nil
	}
	return &fallbackImageUploadStore{
		primary:  primary,
		fallback: fallback,
	}, nil
}

func (s *fallbackImageUploadStore) Save(ctx context.Context, input *ImageUploadInput) (*StoredUploadedImage, error) {
	return s.primary.Save(ctx, input)
}

func (s *fallbackImageUploadStore) SaveWithKey(ctx context.Context, key string, input *ImageUploadInput) (*StoredUploadedImage, error) {
	keyed, ok := s.primary.(KeyedImageUploadStore)
	if !ok {
		return nil, fmt.Errorf("primary upload store does not support keyed writes")
	}
	return keyed.SaveWithKey(ctx, key, input)
}

func (s *fallbackImageUploadStore) Open(ctx context.Context, key string) (*StoredUploadedImage, error) {
	file, err := s.primary.Open(ctx, key)
	if err == nil || s.fallback == nil || !errors.Is(err, ErrUploadedImageNotFound) {
		return file, err
	}
	return s.fallback.Open(ctx, key)
}

func (s *fallbackImageUploadStore) Delete(ctx context.Context, key string) error {
	err := s.primary.Delete(ctx, key)
	if err == nil || s.fallback == nil || !errors.Is(err, ErrUploadedImageNotFound) {
		return err
	}
	return s.fallback.Delete(ctx, key)
}
