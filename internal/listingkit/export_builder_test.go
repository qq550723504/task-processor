package listingkit

import (
	"strings"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildListingKitExportForSelectedPlatform(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID: "task-export-1",
		Request: &GenerateRequest{
			Platforms: []string{"amazon", "shein", "temu", "walmart"},
		},
		Result: &ListingKitResult{
			Platforms: []string{"amazon", "shein", "temu", "walmart"},
			Country:   "US",
			Language:  "en_US",
			Summary: &GenerationSummary{
				SourceType:   "1688_url",
				ImageCount:   4,
				VariantCount: 2,
			},
			Shein: &SheinPackage{
				RequestDraft: &SheinRequestDraft{
					SpuName: "Travel Bottle",
				},
				PreviewProduct: &sheinproduct.Product{
					SPUName: "Travel Bottle",
				},
				Inspection: &SheinInspection{
					NeedsReview: true,
					Summary:     []string{"请确认类目"},
				},
				ReviewNotes: []string{"请确认类目"},
			},
		},
	}

	export, err := buildListingKitExport(task, "shein")
	if err != nil {
		t.Fatalf("build export: %v", err)
	}

	if export.SelectedPlatform != "shein" {
		t.Fatalf("selected platform = %q, want shein", export.SelectedPlatform)
	}
	if export.Shein == nil {
		t.Fatal("expected shein export payload")
	}
	if export.Amazon != nil || export.Temu != nil || export.Walmart != nil {
		t.Fatal("expected only shein export payload")
	}
	if export.Shein.RequestDraft == nil || export.Shein.RequestDraft.SpuName != "Travel Bottle" {
		t.Fatalf("unexpected shein request draft: %+v", export.Shein.RequestDraft)
	}
	if !strings.Contains(export.FileName, "shein") {
		t.Fatalf("file name = %q, want platform suffix", export.FileName)
	}
}

func TestBuildListingKitExportReturnsBundleByDefault(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID: "task-export-2",
		Request: &GenerateRequest{
			Platforms: []string{"temu", "walmart"},
		},
		Result: &ListingKitResult{
			Platforms: []string{"temu", "walmart"},
			Temu: &TemuPackage{
				GoodsName: "Bottle",
			},
			Walmart: &WalmartPackage{
				ProductName: "Bottle",
			},
		},
	}

	export, err := buildListingKitExport(task, "")
	if err != nil {
		t.Fatalf("build export bundle: %v", err)
	}

	if export.Temu == nil || export.Walmart == nil {
		t.Fatal("expected bundle export to include available platforms")
	}
	if !strings.Contains(export.FileName, "bundle") {
		t.Fatalf("file name = %q, want bundle suffix", export.FileName)
	}
}
