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

// ImageUploadPublicURLResolver resolves an object key to a URL that external
// providers can fetch. It is optional because some local/private stores only
// support inline image bytes.
type ImageUploadPublicURLResolver interface {
	ResolvePublicURL(ctx context.Context, key string) (string, error)
}
