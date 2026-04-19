package listingkit

import "context"

type ImageUploadStore interface {
	Save(ctx context.Context, input *ImageUploadInput) (*StoredUploadedImage, error)
	Open(ctx context.Context, key string) (*StoredUploadedImage, error)
}
