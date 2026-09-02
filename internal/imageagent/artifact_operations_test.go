package imageagent

import (
	"strings"
	"testing"
)

func TestNormalizeStagingManifestValidatesOperationIdentifiersWithoutBusinessVocabulary(t *testing.T) {
	for _, operation := range []string{"extract_subject", "render_white_background", "render_scene", "future_capability_v2"} {
		t.Run(operation, func(t *testing.T) {
			if _, err := NormalizeStagingManifest(StagingManifest{Assets: []StagedAssetRef{testOperationAsset(operation)}}); err != nil {
				t.Fatalf("NormalizeStagingManifest(%q) error = %v", operation, err)
			}
		})
	}

	for _, operation := range []string{"provider=https://transient.example", "authorization=secret", "C:/worker/generated.png", "UpperCase", "has space", "trailing_"} {
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
