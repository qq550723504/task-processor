package generation

import (
	"context"
	"testing"

	"task-processor/internal/asset"
	"task-processor/internal/productimage"
)

type readableSourceRecordingSceneRenderer struct {
	source   productimage.ReadableAssetSource
	metadata map[string]string
}

func (r *readableSourceRecordingSceneRenderer) Render(_ context.Context, input *productimage.ImageAsset, _ *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	source, err := productimage.ResolveReadableAssetSource(input)
	if err != nil {
		return nil, err
	}
	r.source = source
	r.metadata = cloneMetadataMap(input.Metadata)
	return []productimage.ImageAsset{{
		URL:  "https://cdn.example.test/rendered-scene.jpg",
		Type: productimage.AssetTypeGalleryImage,
	}}, nil
}

func TestProductImageDeferredRendererResolvesLegacyAmazonSourceBeforeClearingMetadata(t *testing.T) {
	t.Parallel()

	uploadDestination := "https://upload.example.test/write-only-destination"
	provenanceURL := "https://detail.1688.com/offer/1/main.jpg"
	renderer := &readableSourceRecordingSceneRenderer{}
	deferred := NewProductImageDeferredRenderer(renderer)

	_, err := deferred.Render(context.Background(), DeferredRenderRequest{
		TaskID: "task-legacy-amazon-retry",
		Task: Task{
			AssetKind: asset.KindSceneImage,
			Purpose:   "scene",
		},
		BaseAsset: asset.AssetRecord{
			ID:   "legacy-amazon-asset",
			Kind: asset.KindMainImage,
			URL:  uploadDestination,
			Metadata: map[string]string{
				"published_provider":       "amazon",
				"uploaded_url":             uploadDestination,
				"uploaded_destination_url": uploadDestination,
				"source_url":               provenanceURL,
			},
		},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if renderer.source.URL != provenanceURL {
		t.Fatalf("renderer readable URL = %q, want provenance URL %q", renderer.source.URL, provenanceURL)
	}
	if renderer.metadata["published_provider"] != "" || renderer.metadata["uploaded_url"] != "" {
		t.Fatalf("renderer metadata retained publication markers: %+v", renderer.metadata)
	}
}
