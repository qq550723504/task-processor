package productimage

import (
	"os"
	"testing"
)

func TestResolveReadableAssetURLPrefersUploadedURLToProvenance(t *testing.T) {
	t.Parallel()

	got := ResolveReadableAssetURL(
		"https://images.amazon.example/uploaded-main.jpg",
		"https://detail.1688.com/offer/1/main.jpg",
		map[string]string{"uploaded_url": "https://images.amazon.example/uploaded-main.jpg"},
	)
	if got != "https://images.amazon.example/uploaded-main.jpg" {
		t.Fatalf("ResolveReadableAssetURL() = %q, want uploaded URL", got)
	}
}

func TestResolveReadableAssetURLIgnoresLegacyAmazonUploadDestination(t *testing.T) {
	t.Parallel()

	uploadDestination := "https://upload.example.com/write-only-destination"
	provenance := "https://detail.1688.com/offer/1/main.jpg"
	got := ResolveReadableAssetURL(
		uploadDestination,
		provenance,
		map[string]string{
			"published_provider": "amazon",
			"uploaded_url":       uploadDestination,
			"published_url":      uploadDestination,
		},
	)
	if got != provenance {
		t.Fatalf("ResolveReadableAssetURL() = %q, want provenance URL after ignoring legacy upload destination", got)
	}
}

func TestResolveReadableAssetSourcePrefersPublishedPathToRemoteProvenance(t *testing.T) {
	t.Parallel()

	asset := &ImageAsset{
		URL:       "https://images.amazon.example/uploaded-main.jpg",
		SourceURL: "https://detail.1688.com/offer/1/main.jpg",
		Metadata:  map[string]string{"published_path": t.TempDir() + "/published-main.jpg"},
	}
	if err := os.WriteFile(asset.Metadata["published_path"], []byte("published"), 0o644); err != nil {
		t.Fatalf("write published asset: %v", err)
	}

	got, err := ResolveReadableAssetSource(asset)
	if err != nil {
		t.Fatalf("ResolveReadableAssetSource() error = %v", err)
	}
	if got.LocalPath != asset.Metadata["published_path"] || got.URL != "" {
		t.Fatalf("ResolveReadableAssetSource() = %+v, want published local path", got)
	}
}
