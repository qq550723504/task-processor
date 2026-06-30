package publishing

import (
	"strings"
	"testing"
)

func TestEnforceVariantImageCoverageBlocksSharedSingleImageAcrossGroups(t *testing.T) {
	t.Parallel()

	warning, blocked := EnforceVariantImageCoverage(VariantImageCoverageState{
		RequiredGroupCount:          2,
		DistinctImageCount:          1,
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

	if warning, blocked := EnforceVariantImageCoverage(VariantImageCoverageState{
		RequiredGroupCount: 2,
		DistinctImageCount: 2,
	}); blocked {
		t.Fatalf("blocked = true, warning = %q; want allowed", warning)
	}
}

func TestVariantImageGroupCountCountsDistinctAndUnnamedGroups(t *testing.T) {
	t.Parallel()

	if got := VariantImageGroupCount([]string{"black", " black ", "", "white", ""}); got != 4 {
		t.Fatalf("VariantImageGroupCount() = %d, want 4", got)
	}
}

func TestVariantImageCoverageMetadataRoundTrip(t *testing.T) {
	t.Parallel()

	metadata := SetVariantImageCoverageMetadata(nil, "needs images", true)
	message, blocked := VariantImageCoverageStatus(metadata)
	if !blocked || message != "needs images" {
		t.Fatalf("VariantImageCoverageStatus() = %q, %v; want blocked message", message, blocked)
	}

	metadata = SetVariantImageCoverageMetadata(metadata, "", false)
	if metadata != nil {
		t.Fatalf("metadata = %#v, want cleared", metadata)
	}
}
