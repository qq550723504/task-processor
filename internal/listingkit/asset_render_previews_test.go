package listingkit

import (
	"testing"

	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

func TestBuildAssetRenderPreviewsIncludesRenderSummary(t *testing.T) {
	t.Parallel()

	bundle := &asset.Bundle{
		Assets: []asset.Asset{
			{
				ID:   "asset-selling-point",
				Kind: asset.KindSellingPointImage,
				Role: "selling_point",
				Metadata: map[string]string{
					"render_profile":          "shein_selling_point",
					"template_label":          "SHEIN Selling Point",
					"draw_preview_format":     "svg",
					"layout_draw_preview_svg": "<svg/>",
					"render_output_version":   "v2",
					"draw_output_version":     "v1",
					"draw_preview_version":    "v1",
					"layout_render_output": `{
						"layout_engine":"selling_point_output_v2",
						"visual_mode":"selling_point",
						"layers":[
							{"layer_type":"background","region":"full_canvas","style_token":"bg-soft"},
							{"layer_type":"badge","region":"title_band","style_token":"badge-dark"},
							{"layer_type":"text","region":"body_copy","style_token":"copy-primary"}
						]
					}`,
				},
			},
		},
	}

	previews := buildAssetRenderPreviews(bundle)
	if len(previews) != 1 {
		t.Fatalf("previews = %+v", previews)
	}
	preview := previews[0]
	if preview.VisualMode != "selling_point" {
		t.Fatalf("visual_mode = %q, want selling_point", preview.VisualMode)
	}
	if preview.LayoutEngine != "selling_point_output_v2" {
		t.Fatalf("layout_engine = %q, want selling_point_output_v2", preview.LayoutEngine)
	}
	if preview.RenderOutputVersion != "v2" || preview.DrawOutputVersion != "v1" || preview.DrawPreviewVersion != "v1" {
		t.Fatalf("preview versions = %+v", preview)
	}
	if len(preview.LayerTypes) != 3 || preview.LayerTypes[0] != "background" || preview.LayerTypes[2] != "text" {
		t.Fatalf("layer_types = %+v", preview.LayerTypes)
	}
	if len(preview.Regions) != 3 || preview.Regions[1] != "title_band" {
		t.Fatalf("regions = %+v", preview.Regions)
	}
	if len(preview.StyleTokens) != 3 || preview.StyleTokens[2] != "copy-primary" {
		t.Fatalf("style_tokens = %+v", preview.StyleTokens)
	}
}

func TestBuildPlatformAssetRenderPreviewsIncludesRasterAssetFallback(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{
				{
					ID:   "gallery-rendered-1",
					Kind: asset.KindSceneImage,
					URL:  "http://127.0.0.1:9100/listingkit-assets/gallery-rendered-1.png",
				},
			},
		},
		Shein: &SheinPackage{
			ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Gallery: []common.BundleSlot{{
					Key:           "gallery",
					Purpose:       "gallery",
					AssetID:       "gallery-rendered-1",
					Kind:          string(asset.KindSceneImage),
					TemplateLabel: "SHEIN Lifestyle Gallery",
				}},
			},
		},
	}

	previews := buildPlatformAssetRenderPreviews(result)
	if len(previews) != 1 {
		t.Fatalf("previews = %+v, want 1 platform group", previews)
	}
	if len(previews[0].Gallery) != 1 {
		t.Fatalf("gallery previews = %+v, want 1 gallery preview", previews[0].Gallery)
	}
	if previews[0].Gallery[0].AssetURL != "http://127.0.0.1:9100/listingkit-assets/gallery-rendered-1.png" {
		t.Fatalf("gallery preview = %+v, want raster asset url fallback", previews[0].Gallery[0])
	}
}

func TestBuildPlatformAssetRenderPreviewsMapsBundleSlotURLToPublishedAssetURL(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{
				{
					ID:   "gallery-1",
					Kind: asset.KindSceneImage,
					URL:  "http://127.0.0.1:9100/listingkit-assets/gallery-1.png",
					Metadata: map[string]string{
						"published_path": "tmp\\productimage\\gallery-1.png",
						"local_path":     "tmp\\productimage\\gallery-1.png",
					},
				},
			},
		},
		Shein: &SheinPackage{
			ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Gallery: []common.BundleSlot{{
					Key:           "gallery",
					Purpose:       "gallery",
					AssetID:       "rendered-scene-image-source-1",
					URL:           "tmp\\productimage\\gallery-1.png",
					Kind:          string(asset.KindSceneImage),
					TemplateLabel: "SHEIN Lifestyle Gallery",
				}},
			},
		},
	}

	previews := buildPlatformAssetRenderPreviews(result)
	if len(previews) != 1 {
		t.Fatalf("previews = %+v, want 1 platform group", previews)
	}
	if len(previews[0].Gallery) != 1 {
		t.Fatalf("gallery previews = %+v, want 1 gallery preview", previews[0].Gallery)
	}
	if previews[0].Gallery[0].AssetURL != "http://127.0.0.1:9100/listingkit-assets/gallery-1.png" {
		t.Fatalf("gallery preview = %+v, want published url resolved from slot url", previews[0].Gallery[0])
	}
}

func TestDecorateListingKitResultReviewRefreshesStalePlatformRenderPreviews(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{
				{
					ID:   "gallery-1",
					Kind: asset.KindSceneImage,
					URL:  "http://127.0.0.1:9100/listingkit-assets/gallery-1.png",
					Metadata: map[string]string{
						"published_path": "tmp\\productimage\\gallery-1.png",
						"local_path":     "tmp\\productimage\\gallery-1.png",
					},
				},
			},
		},
		PlatformAssetRenderPreviews: []PlatformAssetRenderPreviews{{
			Platform: "shein",
			Summary:  &PlatformAssetRenderPreviewSummary{TotalPreviews: 1},
			Auxiliary: []AssetRenderPreviewSlot{{
				Slot:    "auxiliary",
				AssetID: "stale-preview",
			}},
		}},
		Shein: &SheinPackage{
			ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Gallery: []common.BundleSlot{{
					Key:           "gallery",
					Purpose:       "gallery",
					AssetID:       "rendered-scene-image-source-1",
					URL:           "tmp\\productimage\\gallery-1.png",
					Kind:          string(asset.KindSceneImage),
					TemplateLabel: "SHEIN Lifestyle Gallery",
				}},
			},
		},
	}

	decorateListingKitResultReview(result, nil)

	if len(result.PlatformAssetRenderPreviews) != 1 || len(result.PlatformAssetRenderPreviews[0].Gallery) != 1 {
		t.Fatalf("platform previews = %+v, want refreshed gallery preview", result.PlatformAssetRenderPreviews)
	}
	if result.PlatformAssetRenderPreviews[0].Gallery[0].AssetURL != "http://127.0.0.1:9100/listingkit-assets/gallery-1.png" {
		t.Fatalf("gallery preview = %+v, want refreshed published url", result.PlatformAssetRenderPreviews[0].Gallery[0])
	}
}

func TestBuildPlatformAssetRenderPreviewsPrefersPublishedURLOverLocalRendererPath(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{
				{
					ID:   "gallery-1",
					Kind: asset.KindGalleryImage,
					URL:  "http://127.0.0.1:9100/listingkit-assets/gallery-1.png",
					Role: "gallery",
					SourceAssetIDs: []string{
						"source-1",
					},
					Metadata: map[string]string{
						"published_url":  "http://127.0.0.1:9100/listingkit-assets/gallery-1.png",
						"published_path": "tmp\\productimage\\gallery-published.png",
						"local_path":     "tmp\\productimage\\gallery-published.png",
					},
				},
				{
					ID:   "rendered-scene-image-source-1",
					Kind: asset.KindSceneImage,
					URL:  "tmp\\productimage\\gallery-renderer-output.png",
					Role: "gallery",
					SourceAssetIDs: []string{
						"source-1",
					},
					Metadata: map[string]string{
						"local_path": "tmp\\productimage\\gallery-renderer-output.png",
					},
				},
			},
		},
		Shein: &SheinPackage{
			ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Gallery: []common.BundleSlot{{
					Key:           "gallery",
					Purpose:       "gallery",
					AssetID:       "rendered-scene-image-source-1",
					URL:           "tmp\\productimage\\gallery-renderer-output.png",
					Kind:          string(asset.KindSceneImage),
					TemplateLabel: "SHEIN Lifestyle Gallery",
					SourceAssetIDs: []string{
						"source-1",
					},
				}},
			},
		},
	}

	previews := buildPlatformAssetRenderPreviews(result)
	if len(previews) != 1 || len(previews[0].Gallery) != 1 {
		t.Fatalf("previews = %+v, want gallery preview", previews)
	}
	if previews[0].Gallery[0].AssetURL != "http://127.0.0.1:9100/listingkit-assets/gallery-1.png" {
		t.Fatalf("gallery preview = %+v, want published url preferred over local path", previews[0].Gallery[0])
	}
}
