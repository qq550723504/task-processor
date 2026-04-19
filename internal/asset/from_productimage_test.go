package asset

import (
	"testing"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func TestBuildBundleBuildsReusableAssetSnapshot(t *testing.T) {
	t.Parallel()

	bundle := BuildBundle(&productenrich.CanonicalProduct{
		Images: []productenrich.CanonicalImage{
			{URL: "https://example.com/source-1.jpg", Role: "primary"},
			{URL: "https://example.com/source-2.jpg", Role: "gallery"},
		},
	}, &productimage.ImageProcessResult{
		MainImage:     &productimage.ImageAsset{URL: "https://cdn.example.com/main.jpg", SourceURL: "https://example.com/source-1.jpg"},
		WhiteBgImage:  &productimage.ImageAsset{URL: "https://cdn.example.com/white.jpg"},
		SubjectCutout: &productimage.ImageAsset{URL: "https://cdn.example.com/cutout.png"},
		GalleryImages: []productimage.ImageAsset{
			{URL: "https://cdn.example.com/gallery-1.jpg"},
		},
		Review: &productimage.ReviewDecision{
			NeedsReview: true,
			Reasons:     []string{"主图带中文广告"},
		},
	})

	if bundle == nil {
		t.Fatal("expected bundle")
	}
	if bundle.Selection == nil || bundle.Selection.MainAssetID == "" {
		t.Fatalf("selection = %+v", bundle.Selection)
	}
	if bundle.Stats == nil || bundle.Stats.TotalAssets != 6 {
		t.Fatalf("stats = %+v", bundle.Stats)
	}
	if bundle.Review == nil || !bundle.Review.NeedsReview {
		t.Fatalf("review = %+v", bundle.Review)
	}
}
