package workspace

import (
	"testing"

	common "task-processor/internal/publishing/common"
)

func TestBuildRevisionInputFromEditorContext(t *testing.T) {
	ctx := &EditorContext{
		Basics: &EditorBasicsContext{
			SpuName:     "SPU-1",
			Description: "Ready draft",
			Images:      &common.ImageSet{MainImage: "main.jpg"},
			ReviewNotes: []string{"review"},
		},
		Category: &EditorCategoryContext{
			SuggestedPatch: &CategoryResolutionPatch{
				MatchedPath: []string{"Home", "Kitchen"},
			},
		},
	}

	input := BuildRevisionInputFromEditorContext(ctx)

	if input == nil {
		t.Fatal("expected revision input")
	}
	if input.SpuName == nil || *input.SpuName != "SPU-1" {
		t.Fatalf("spu name = %#v", input.SpuName)
	}
	if input.Images == nil || input.Images.MainImage != "main.jpg" {
		t.Fatalf("images = %#v", input.Images)
	}
	if input.CategoryResolution == nil || len(input.CategoryResolution.MatchedPath) != 2 {
		t.Fatalf("category patch = %#v", input.CategoryResolution)
	}
}
