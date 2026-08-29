package imageagent

import (
	"strings"
	"testing"
)

func TestNormalizeStagingManifestRestrictsOperationsToTrustedVocabulary(t *testing.T) {
	validOperations := []string{
		"select_subject",
		"extract_subject_placeholder",
		"cleanup_placeholder",
		"remove_overlay_text_placeholder",
		"remove_promo_badge_placeholder",
		"remove_logo_overlay_placeholder",
		"render_white_bg_placeholder",
		"extract_subject",
		"cleanup_image",
		"render_white_bg",
		"extract_subject_bbox",
		"extract_subject_segmenter",
		"render_white_bg_model",
		"compose_on_white_canvas",
		"cleanup_overlay_signal",
		"cleanup_quality",
		"remove_overlay_regions",
		"resize",
		"render_scene_model",
		"render_image_model",
		"extract_subject_model",
		"normalize_for_amazon",
		"download_source",
		"optimize_for_amazon",
		"render_scene_canvas",
	}
	for _, operation := range validOperations {
		t.Run(operation, func(t *testing.T) {
			if _, err := NormalizeStagingManifest(StagingManifest{Assets: []StagedAssetRef{testOperationAsset(operation)}}); err != nil {
				t.Fatalf("NormalizeStagingManifest(%q) error = %v", operation, err)
			}
		})
	}

	for _, operation := range []string{"provider=https://transient.example", "authorization=secret", "C:/worker/generated.png", "unknown_operation"} {
		t.Run("reject "+operation, func(t *testing.T) {
			if _, err := NormalizeStagingManifest(StagingManifest{Assets: []StagedAssetRef{testOperationAsset(operation)}}); err == nil {
				t.Fatalf("NormalizeStagingManifest(%q) error = nil, want validation failure", operation)
			}
		})
	}
}

func testOperationAsset(operation string) StagedAssetRef {
	return StagedAssetRef{
		ObjectKey:         "image-agent/staging/tenant-a/run-1/3/slot-1/2/0-" + strings.Repeat("a", 64) + ".png",
		SHA256:            strings.Repeat("a", 64),
		SizeBytes:         1,
		ContentType:       "image/png",
		Width:             1,
		Height:            1,
		SourceAssetID:     "source-1",
		Operations:        []string{operation},
		ProviderReceiptID: "receipt-1",
	}
}
