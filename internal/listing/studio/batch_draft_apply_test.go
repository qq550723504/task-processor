package studio

import (
	"slices"
	"testing"
)

type batchDraftApplyTestTask struct {
	ID string
}

func TestApplyBatchDraftFieldsNormalizesCreateInputAndCopiesMutableFields(t *testing.T) {
	input := BatchDraftInput[string, string, string, batchDraftApplyTestTask, string]{
		Selection: SelectionKeyInput{
			ProductID:          124110,
			ParentProductID:    124100,
			VariantID:          124200,
			PrototypeGroupID:   18203,
			LayerID:            " layer-2 ",
			PrintableWidth:     1200,
			PrintableHeight:    900,
			SelectedVariantIDs: []int64{124200, 124201},
		},
		SelectionDesignType:        " ",
		Prompt:                     "retro cherries",
		PromptMode:                 " guided ",
		ProductImagePrompts:        []string{"front", "back"},
		SelectedSDSImages:          []string{"sds-1"},
		GroupedSelections:          []string{"group-1"},
		TransparentBackground:      true,
		TransparentBackgroundMode:  " REMOVAL ",
		HotStyleReferenceImageURLs: []string{" ", " https://cdn.example.com/ref.png ", "https://cdn.example.com/other.png"},
		HotStyleReferenceBrief:     " brief ",
		HotStyleReferencePrompt:    " prompt ",
		ApprovedDesignIDs:          []string{"design-1"},
		CreatedTasks:               []batchDraftApplyTestTask{{ID: " task-1 "}, {ID: ""}},
		GenerationJobs:             []string{"must-be-dropped-on-create"},
		DesignCount:                1,
	}

	got := ApplyBatchDraftFields(input, true, func(task batchDraftApplyTestTask) string { return task.ID })

	if want := "124110:124100:124200:18203:layer-2:1200:900:124200,124201"; got.SelectionKey != want {
		t.Fatalf("SelectionKey = %q, want %q", got.SelectionKey, want)
	}
	if got.SelectionDesignType != DefaultBatchDesignType {
		t.Fatalf("SelectionDesignType = %q, want %q", got.SelectionDesignType, DefaultBatchDesignType)
	}
	if got.Status != DraftStatusTasksCreated {
		t.Fatalf("Status = %q, want %q", got.Status, DraftStatusTasksCreated)
	}
	if got.TransparentBackgroundMode != TransparencyModeRemoval || !got.TransparentBackground {
		t.Fatalf("transparency = (%q, %t), want (%q, true)", got.TransparentBackgroundMode, got.TransparentBackground, TransparencyModeRemoval)
	}
	if want := []string{"https://cdn.example.com/ref.png"}; !slices.Equal(got.HotStyleReferenceImageURLs, want) {
		t.Fatalf("HotStyleReferenceImageURLs = %#v, want %#v", got.HotStyleReferenceImageURLs, want)
	}
	if got.HotStyleReferenceBrief != "brief" || got.HotStyleReferencePrompt != "prompt" {
		t.Fatalf("hot-style text = (%q, %q), want trimmed values", got.HotStyleReferenceBrief, got.HotStyleReferencePrompt)
	}
	if len(got.GenerationJobs) != 0 {
		t.Fatalf("GenerationJobs = %#v, want dropped on create", got.GenerationJobs)
	}
	if want := []string{" task-1 "}; !slices.Equal(got.CreatedTaskIDs, want) {
		t.Fatalf("CreatedTaskIDs = %#v, want %#v", got.CreatedTaskIDs, want)
	}

	input.ProductImagePrompts[0] = "mutated"
	input.Selection.SelectedVariantIDs[0] = 999
	if got.ProductImagePrompts[0] != "front" {
		t.Fatalf("ProductImagePrompts = %#v, want copied values", got.ProductImagePrompts)
	}
	if got.Selection.SelectedVariantIDs[0] != 124200 {
		t.Fatalf("SelectedVariantIDs = %#v, want copied values", got.Selection.SelectedVariantIDs)
	}
}

func TestApplyBatchDraftFieldsPreservesGenerationJobsOnUpdate(t *testing.T) {
	got := ApplyBatchDraftFields(BatchDraftInput[string, string, string, batchDraftApplyTestTask, string]{
		Selection:      SelectionKeyInput{VariantID: 42},
		GenerationJobs: []string{"job-1"},
		CreatedTasks:   []batchDraftApplyTestTask{{ID: "task-1"}},
		DesignCount:    1,
	}, false, func(task batchDraftApplyTestTask) string { return task.ID })

	if want := []string{"job-1"}; !slices.Equal(got.GenerationJobs, want) {
		t.Fatalf("GenerationJobs = %#v, want %#v", got.GenerationJobs, want)
	}
	if got.Status != DraftStatusGenerating {
		t.Fatalf("Status = %q, want %q", got.Status, DraftStatusGenerating)
	}
}
