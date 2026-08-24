package studio

import (
	"slices"
	"testing"
)

func TestNormalizeBatchDesignTypeDefaultsBlankValue(t *testing.T) {
	if got := NormalizeBatchDesignType("  "); got != DefaultBatchDesignType {
		t.Fatalf("NormalizeBatchDesignType() = %q, want %q", got, DefaultBatchDesignType)
	}
}

func TestNormalizeBatchDesignTypePreservesExplicitValue(t *testing.T) {
	if got := NormalizeBatchDesignType("embroidery"); got != "embroidery" {
		t.Fatalf("NormalizeBatchDesignType() = %q, want embroidery", got)
	}
}

func TestResolveDraftStatusPrioritizesGenerationTasksThenDesigns(t *testing.T) {
	tests := []struct {
		name  string
		input DraftStatusInput
		want  DraftStatus
	}{
		{name: "generation jobs", input: DraftStatusInput{GenerationJobCount: 1}, want: DraftStatusGenerating},
		{name: "created tasks", input: DraftStatusInput{CreatedTaskCount: 1}, want: DraftStatusTasksCreated},
		{name: "designs", input: DraftStatusInput{DesignCount: 1}, want: DraftStatusReviewing},
		{name: "empty", want: DraftStatusSelecting},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveDraftStatus(tt.input); got != tt.want {
				t.Fatalf("ResolveDraftStatus(%+v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeHotStyleReferenceImageURLsKeepsFirstDistinctNonEmptyURL(t *testing.T) {
	got := NormalizeHotStyleReferenceImageURLs([]string{
		"  ",
		" https://cdn.example.com/first.png ",
		"https://cdn.example.com/first.png",
		"https://cdn.example.com/second.png",
	})

	if want := []string{"https://cdn.example.com/first.png"}; !slices.Equal(got, want) {
		t.Fatalf("NormalizeHotStyleReferenceImageURLs() = %#v, want %#v", got, want)
	}
}

func TestBuildSelectionKeyUsesStableBatchSelectionFields(t *testing.T) {
	got := BuildSelectionKey(SelectionKeyInput{
		ProductID:        124110,
		ParentProductID:  124100,
		VariantID:        124200,
		PrototypeGroupID: 18203,
		LayerID:          " layer-2 ",
		PrintableWidth:   1200,
		PrintableHeight:  900,
		SelectedVariantIDs: []int64{
			124200,
			124201,
		},
	})

	if want := "124110:124100:124200:18203:layer-2:1200:900:124200,124201"; got != want {
		t.Fatalf("BuildSelectionKey() = %q, want %q", got, want)
	}
}

func TestShouldDropCreateGenerationJobsOnlyForCreate(t *testing.T) {
	if !ShouldDropCreateGenerationJobs(true, 1) {
		t.Fatal("ShouldDropCreateGenerationJobs(create, 1) = false, want true")
	}
	if ShouldDropCreateGenerationJobs(false, 1) {
		t.Fatal("ShouldDropCreateGenerationJobs(update, 1) = true, want false")
	}
	if ShouldDropCreateGenerationJobs(true, 0) {
		t.Fatal("ShouldDropCreateGenerationJobs(create, 0) = true, want false")
	}
}

func TestResolveBatchNameUsesRequestedName(t *testing.T) {
	got := ResolveBatchName(BatchNameResolutionInput{
		RequestedName: " Summer ",
		ExistingName:  "Old",
		IsCreate:      false,
		ExistingNames: []string{"批次9"},
	})

	if got != "Summer" {
		t.Fatalf("ResolveBatchName() = %q, want Summer", got)
	}
}

func TestResolveBatchNamePreservesExistingNameOnUpdate(t *testing.T) {
	got := ResolveBatchName(BatchNameResolutionInput{
		ExistingName:  " Existing ",
		IsCreate:      false,
		ExistingNames: []string{"批次9"},
	})

	if got != "Existing" {
		t.Fatalf("ResolveBatchName() = %q, want Existing", got)
	}
}

func TestResolveBatchNameGeneratesDefaultName(t *testing.T) {
	got := ResolveBatchName(BatchNameResolutionInput{
		IsCreate:      true,
		ExistingNames: []string{"批次2"},
	})

	if got != "批次3" {
		t.Fatalf("ResolveBatchName() = %q, want 批次3", got)
	}
}
