package listingkit

import "testing"

func TestPreviewPlatformsPrefersResultPlatforms(t *testing.T) {
	t.Parallel()

	got := previewPlatforms(&Task{
		Request: &GenerateRequest{Platforms: []string{"amazon"}},
		Result:  &ListingKitResult{Platforms: []string{"shein", "temu"}},
	})
	if len(got) != 2 || got[0] != "shein" || got[1] != "temu" {
		t.Fatalf("previewPlatforms() = %#v, want result platforms", got)
	}
}

func TestBuildPreviewHeaderCopiesSummaryFields(t *testing.T) {
	t.Parallel()

	header := buildPreviewHeader(&ListingKitResult{
		Country:  "US",
		Language: "en_US",
		Summary: &GenerationSummary{
			SourceType:   "text",
			ImageCount:   2,
			VariantCount: 3,
			Warnings:     []string{"warn"},
		},
	}, "")
	if header == nil {
		t.Fatal("expected preview header")
	}
	if header.Country != "US" || header.Language != "en_US" {
		t.Fatalf("header locale fields = %+v", header)
	}
	if header.SourceType != "text" || header.ImageCount != 2 || header.VariantCount != 3 {
		t.Fatalf("header summary fields = %+v", header)
	}
	if len(header.Warnings) != 1 || header.Warnings[0] != "warn" {
		t.Fatalf("header warnings = %#v, want [warn]", header.Warnings)
	}
}

func TestNormalizePreviewPlatform(t *testing.T) {
	t.Parallel()

	if got := normalizePreviewPlatform("  SHEIN "); got != "shein" {
		t.Fatalf("normalizePreviewPlatform() = %q, want %q", got, "shein")
	}
}
