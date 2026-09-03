package sourcing

import (
	"testing"
)

func TestToSnapshotPersistsImageAssetCandidates(t *testing.T) {
	snapshot, err := ToSnapshot(SourceEnvelope{
		Identity:         SourceIdentity{SourceType: SourceTypeCrawler, SourcePlatform: "1688", SourceID: "777"},
		ProductCandidate: ProductCandidate{Title: "Lunch Bag"},
		AssetCandidates: []AssetCandidate{
			{SourceID: "main", URL: "https://cdn.example.test/main.jpg", MediaType: "image", Role: "primary", Checksum: "sha-main", Width: 1600, Height: 1200},
			{SourceID: "video", URL: "https://cdn.example.test/demo.mp4", MediaType: "video", Role: "video"},
		},
	})
	if err != nil {
		t.Fatalf("ToSnapshot() error = %v", err)
	}
	if len(snapshot.Images) != 1 {
		t.Fatalf("snapshot.Images = %#v, want one image candidate", snapshot.Images)
	}
	if got := snapshot.Images[0]; got.URL != "https://cdn.example.test/main.jpg" || got.Role != "primary" || got.Width != 1600 || got.Height != 1200 {
		t.Fatalf("snapshot.Images[0] = %#v, want persisted image identity", got)
	}
}

func TestToSnapshotPreservesTypedVariantCommerceFacts(t *testing.T) {
	snapshot, err := ToSnapshot(SourceEnvelope{
		Identity: SourceIdentity{SourceType: SourceTypeCrawler, SourcePlatform: "1688", SourceID: "777"},
		ProductCandidate: ProductCandidate{
			Title: "Lunch Bag",
			Variants: []ProductVariantCandidate{{
				SourceID: "SKU-1", SKU: "SKU-1", Currency: "CNY", Price: 19.9, Stock: 20,
			}},
		},
	})
	if err != nil {
		t.Fatalf("ToSnapshot() error = %v", err)
	}
	if len(snapshot.Variants) != 1 || snapshot.Variants[0].Price == nil || snapshot.Variants[0].Price.Currency != "CNY" || snapshot.Variants[0].Price.Amount != 19.9 || snapshot.Variants[0].Stock != 20 {
		t.Fatalf("snapshot variant = %+v, want typed price and stock", snapshot.Variants)
	}
}
