package listingkit

import (
	"testing"

	"task-processor/internal/asset"
	"task-processor/internal/productimage"
)

func TestImageProcessRequestsRequireExplicitSupportedTargets(t *testing.T) {
	task := &Task{Request: &GenerateRequest{
		ProductURL: "https://example.test/product",
		Options:    &GenerateOptions{ProcessImages: true},
	}}

	requests, err := toImageProcessRequests(task)
	if err == nil {
		t.Fatalf("toImageProcessRequests() = %#v, want explicit-target error", requests)
	}
	if len(requests) != 0 {
		t.Fatalf("toImageProcessRequests() produced %#v without a target", requests)
	}
}

func TestListingKitResultKeepsMultiTargetAssetsOutOfLegacyScalars(t *testing.T) {
	result := &ListingKitResult{}
	sheinImages := &productimage.ImageProcessResult{}
	temuImages := &productimage.ImageProcessResult{}
	sheinBundle := &asset.Bundle{}
	temuBundle := &asset.Bundle{}

	result.recordTargetImageAssets("shein", sheinImages, sheinBundle, asset.InventorySummaryFromBundle(sheinBundle))
	result.recordTargetImageAssets("temu", temuImages, temuBundle, asset.InventorySummaryFromBundle(temuBundle))

	if result.ImageAssetsForTarget("shein") != sheinImages || result.ImageAssetsForTarget("temu") != temuImages {
		t.Fatalf("target-keyed images = %#v", result.ImageAssetsByTarget)
	}
	if result.AssetBundleForTarget("shein") != sheinBundle || result.AssetBundleForTarget("temu") != temuBundle {
		t.Fatalf("target-keyed bundles = %#v", result.AssetBundlesByTarget)
	}
	if result.ImageAssets != nil || result.AssetBundle != nil || result.AssetInventorySummary != nil {
		t.Fatalf("legacy scalar projection = %#v/%#v/%#v, want unset for unselected multi-target result", result.ImageAssets, result.AssetBundle, result.AssetInventorySummary)
	}

	result.applyCompatibilityAssetProjection("temu")
	if result.ImageAssets != temuImages || result.AssetBundle != temuBundle {
		t.Fatalf("compatibility projection = %#v/%#v, want temu values", result.ImageAssets, result.AssetBundle)
	}
}

func TestImageProcessRequestsKeepEachNormalizedTarget(t *testing.T) {
	task := &Task{Request: &GenerateRequest{
		ProductURL: "https://example.test/product",
		Platforms:  []string{" TEMU ", "shein", "temu"},
		Options:    &GenerateOptions{ProcessImages: true},
	}}

	requests, err := toImageProcessRequests(task)
	if err != nil {
		t.Fatalf("toImageProcessRequests() error = %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("toImageProcessRequests() count = %d, want 2", len(requests))
	}
	if requests[0].TargetPlatform != "temu" || requests[1].TargetPlatform != "shein" {
		t.Fatalf("toImageProcessRequests() targets = %q, %q; want temu, shein", requests[0].TargetPlatform, requests[1].TargetPlatform)
	}
	for _, request := range requests {
		if request.Marketplace != "" {
			t.Fatalf("new ListingKit request should use TargetPlatform, got legacy marketplace %q", request.Marketplace)
		}
	}
}

func TestProcessableImageRequestWithoutSupportedTargetFailsValidation(t *testing.T) {
	for _, platforms := range [][]string{nil, {}, {"unsupported"}} {
		req := &GenerateRequest{
			ImageURLs: []string{"https://example.test/image.jpg"},
			Platforms: platforms,
			Options:   &GenerateOptions{ProcessImages: true},
		}
		normalizeGenerateRequest(req)
		if err := validateRequest(req); err == nil {
			t.Fatalf("validateRequest() error = nil for platforms %#v", platforms)
		}
	}
}
