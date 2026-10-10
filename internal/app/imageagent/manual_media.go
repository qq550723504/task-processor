package imageagentapp

import (
	"context"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
)

type ImageSetManualMedia struct{ Media collection.SourceMedia }

func (r ImageSetManualMedia) ReadManualImage(ctx context.Context, ref asset.ManualImageReference) (asset.SourceImage, error) {
	if r.Media == nil || !ref.Valid() {
		return asset.SourceImage{}, asset.ErrApprovedAssetsNotReady
	}
	image, err := r.Media.Read(ctx, collection.MediaIdentity{Hash: ref.Hash, Bytes: ref.Bytes})
	if err != nil {
		return asset.SourceImage{}, err
	}
	if image.Hash != ref.Hash || image.Bytes != ref.Bytes {
		return asset.SourceImage{}, asset.ErrApprovalConflict
	}
	id := "source-media-" + ref.Hash
	return asset.SourceImage{ID: id, URL: image.URL, Width: image.Width, Height: image.Height, ReferenceHash: asset.ReferenceHash(id, image.URL)}, nil
}
