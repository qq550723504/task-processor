package listingkit

import (
	"testing"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

func TestBuildAmazonPreviewPayloadUsesVisualPreviewBase(t *testing.T) {
	t.Parallel()

	imageBundle := &common.PublishImageBundle{
		Platform: "amazon",
		Main: &common.BundleSlot{
			AssetID: "asset-main",
			URL:     "https://cdn.example.com/amazon-main.jpg",
		},
	}
	assetBundle := &asset.Bundle{
		Assets: []asset.Asset{{
			ID:   "asset-main",
			Kind: asset.KindMainImage,
			URL:  "https://cdn.example.com/amazon-main.jpg",
			Metadata: map[string]string{
				"prompt_key": "productimage.scene.home",
			},
		}},
	}
	renderPreviews := &PlatformAssetRenderPreviews{Platform: "amazon"}
	payload := buildAmazonPreviewPayload(&AmazonPackage{
		ImageBundle: imageBundle,
		Draft: &amazonlisting.AmazonListingDraft{
			Title:       "Desk Lamp",
			Brand:       "Northwind",
			ProductType: "lighting",
		},
	}, assetBundle, renderPreviews)

	if payload == nil {
		t.Fatal("expected amazon payload")
	}
	if payload.ImageBundle != imageBundle || payload.RenderPreviews != renderPreviews {
		t.Fatalf("amazon payload visual base = %+v", payload)
	}
	if len(payload.ScenePresets) != 1 || payload.ScenePresets[0].AssetID != "asset-main" {
		t.Fatalf("amazon scene presets = %+v", payload.ScenePresets)
	}
}

func TestBuildTemuPreviewPayloadCopiesReviewStateFromBase(t *testing.T) {
	t.Parallel()

	renderPreviews := &PlatformAssetRenderPreviews{Platform: "temu"}
	payload := buildTemuPreviewPayload(&TemuPackage{
		GoodsName:   "Travel Mug",
		ReviewNotes: []string{"check color wording"},
		ImageBundle: &common.PublishImageBundle{Platform: "temu"},
	}, &asset.Bundle{}, renderPreviews)

	if payload == nil {
		t.Fatal("expected temu payload")
	}
	if !payload.NeedsReview || len(payload.ReviewNotes) != 1 || payload.ReviewNotes[0] != "check color wording" {
		t.Fatalf("temu review state = %+v", payload)
	}
	if payload.RenderPreviews != renderPreviews {
		t.Fatalf("temu render previews = %+v", payload.RenderPreviews)
	}
}

func TestBuildWalmartPreviewPayloadCopiesReviewStateFromBase(t *testing.T) {
	t.Parallel()

	renderPreviews := &PlatformAssetRenderPreviews{Platform: "walmart"}
	payload := buildWalmartPreviewPayload(&WalmartPackage{
		ProductName: "Storage Basket",
		ReviewNotes: []string{"confirm material"},
		ImageBundle: &common.PublishImageBundle{Platform: "walmart"},
	}, &asset.Bundle{}, renderPreviews)

	if payload == nil {
		t.Fatal("expected walmart payload")
	}
	if !payload.NeedsReview || len(payload.ReviewNotes) != 1 || payload.ReviewNotes[0] != "confirm material" {
		t.Fatalf("walmart review state = %+v", payload)
	}
	if payload.RenderPreviews != renderPreviews {
		t.Fatalf("walmart render previews = %+v", payload.RenderPreviews)
	}
}

func TestBuildPreviewPayloadFromResultUsesPlatformRenderPreviewSelection(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		AssetBundle: &asset.Bundle{},
		Amazon: &AmazonPackage{
			ImageBundle: &common.PublishImageBundle{Platform: "amazon"},
			Draft: &amazonlisting.AmazonListingDraft{
				Title: "Desk Lamp",
			},
		},
		Temu: &TemuPackage{
			GoodsName:   "Travel Mug",
			ReviewNotes: []string{"check color wording"},
			ImageBundle: &common.PublishImageBundle{Platform: "temu"},
		},
	}
	platformPreviews := []PlatformAssetRenderPreviews{
		{Platform: "amazon"},
		{Platform: "temu"},
	}

	amazonPayload := buildAmazonPreviewPayloadFromResult(result, platformPreviews)
	if amazonPayload == nil || amazonPayload.RenderPreviews == nil || amazonPayload.RenderPreviews.Platform != "amazon" {
		t.Fatalf("amazon payload = %+v", amazonPayload)
	}

	temuPayload := buildTemuPreviewPayloadFromResult(result, platformPreviews)
	if temuPayload == nil || temuPayload.RenderPreviews == nil || temuPayload.RenderPreviews.Platform != "temu" {
		t.Fatalf("temu payload = %+v", temuPayload)
	}
}
