package common

import (
	"testing"

	"task-processor/internal/asset"
)

func TestBuildImagesFromBundleWithSelectionUsesSelectedAssets(t *testing.T) {
	bundle := &asset.Bundle{
		Assets: []asset.Asset{
			{ID: "source-1", Kind: asset.KindSourceImage, URL: "source-1.jpg"},
			{ID: "source-2", Kind: asset.KindSourceImage, URL: "source-2.jpg"},
			{ID: "white", Kind: asset.KindSourceImage, URL: "white.jpg"},
			{ID: "gallery", Kind: asset.KindSceneImage, URL: "gallery.jpg"},
		},
		Selection: &asset.Selection{
			MainAssetID:     "source-2",
			WhiteBgAssetID:  "white",
			GalleryAssetIDs: []string{"gallery"},
			SourceAssetIDs:  []string{"source-1", "source-2"},
		},
	}

	images := BuildImagesFromBundleWithSelection(bundle)
	if images == nil {
		t.Fatal("BuildImagesFromBundleWithSelection() = nil")
	}
	if images.MainImage != "source-2.jpg" {
		t.Fatalf("main image = %q, want selected asset", images.MainImage)
	}
	if images.WhiteBgImage != "white.jpg" {
		t.Fatalf("white background = %q, want selected asset", images.WhiteBgImage)
	}
	if len(images.Gallery) != 1 || images.Gallery[0] != "gallery.jpg" {
		t.Fatalf("gallery = %#v, want selected asset", images.Gallery)
	}
}

func TestBuildImagesFromBundleWithSelectionFallsBackToFirstSource(t *testing.T) {
	bundle := &asset.Bundle{
		Assets: []asset.Asset{
			{ID: "source-1", Kind: asset.KindSourceImage, URL: "source-1.jpg"},
			{ID: "source-2", Kind: asset.KindSourceImage, URL: "source-2.jpg"},
		},
	}

	images := BuildImagesFromBundleWithSelection(bundle)
	if images == nil || images.MainImage != "source-1.jpg" {
		t.Fatalf("images = %#v, want first source as main image", images)
	}
	if len(images.Gallery) != 1 || images.Gallery[0] != "source-2.jpg" {
		t.Fatalf("gallery = %#v, want remaining source image", images.Gallery)
	}
}
