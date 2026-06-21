package shein

import (
	"strings"
	"testing"
)

func TestEnforceVariantImageCoverageBlocksSharedSingleImageAcrossGroups(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		RequestDraft: &RequestDraft{
			SKCList: []SKCRequestDraft{
				{SkcName: "black", ImageInfo: &ImageDraft{MainImage: "shared.jpg"}},
				{SkcName: "white", ImageInfo: &ImageDraft{MainImage: "shared.jpg"}},
			},
		},
	}

	warning, blocked := EnforceVariantImageCoverage(pkg, VariantImageCoverageInput{
		AvailableVariantImageGroups: 1,
		SDSError:                    "SDS failed for white",
	})

	if !blocked {
		t.Fatal("blocked = false, want true")
	}
	if !strings.Contains(warning, "SDS failed for white") {
		t.Fatalf("warning = %q, want SDS error detail", warning)
	}
}

func TestEnforceVariantImageCoverageAllowsDistinctImages(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		RequestDraft: &RequestDraft{
			SKCList: []SKCRequestDraft{
				{SkcName: "black", ImageInfo: &ImageDraft{MainImage: "black.jpg"}},
				{SkcName: "white", ImageInfo: &ImageDraft{MainImage: "white.jpg"}},
			},
		},
	}

	if warning, blocked := EnforceVariantImageCoverage(pkg, VariantImageCoverageInput{}); blocked {
		t.Fatalf("blocked = true, warning = %q; want allowed", warning)
	}
}

func TestVariantImageCoverageMetadataRoundTrip(t *testing.T) {
	t.Parallel()

	pkg := &Package{}
	SetVariantImageCoverageMetadata(pkg, "needs images", true)
	message, blocked := VariantImageCoverageStatus(pkg)
	if !blocked || message != "needs images" {
		t.Fatalf("VariantImageCoverageStatus() = %q, %v; want blocked message", message, blocked)
	}

	SetVariantImageCoverageMetadata(pkg, "", false)
	if pkg.Metadata != nil {
		t.Fatalf("metadata = %#v, want cleared", pkg.Metadata)
	}
}
