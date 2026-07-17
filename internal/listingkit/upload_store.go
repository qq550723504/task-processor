package listingkit

import "context"

type ImageUploadStore interface {
	Save(ctx context.Context, input *ImageUploadInput) (*StoredUploadedImage, error)
	Open(ctx context.Context, key string) (*StoredUploadedImage, error)
	Delete(ctx context.Context, key string) error
}

// KeyedImageUploadStore accepts a server-authorized object key. ListingKit
// upload APIs use this interface so callers never choose a storage location.
type KeyedImageUploadStore interface {
	ImageUploadStore
	SaveWithKey(ctx context.Context, key string, input *ImageUploadInput) (*StoredUploadedImage, error)
}
