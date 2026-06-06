package listingkit

import (
	"reflect"
	"testing"
)

func TestCloneGenerationQueueQuery(t *testing.T) {
	t.Parallel()

	if cloned := cloneGenerationQueueQuery(nil); cloned != nil {
		t.Fatalf("cloneGenerationQueueQuery(nil) = %+v, want nil", cloned)
	}

	original := &GenerationQueueQuery{
		Platform:                      "shein",
		Slot:                          "main",
		FromPlatform:                  "shein",
		FromSlot:                      "detail",
		FromCapability:                "detail_preview",
		FromSectionKey:                "section-1",
		AssetID:                       "asset-preview-1",
		AssetRevision:                 "asset-rev-1",
		PreviewRevision:               "preview-rev-1",
		TaskRevision:                  "task-rev-1",
		DeltaToken:                    "delta-1",
		IfMatch:                       "match-1",
		ResponseMode:                  "full",
		State:                         "ready",
		ExecutionMode:                 "renderer_backed",
		ExecutionQuality:              "hq",
		QualityGrade:                  "ideal",
		QualityGradeLabel:             "Ideal",
		PreviewCapability:             "detail_preview",
		RenderPreviewAvailable:        true,
		RenderPreviewAvailablePresent: true,
		Retryable:                     true,
		RetryablePresent:              true,
		Page:                          2,
		PageSize:                      25,
		SortBy:                        "updated_at",
		SortOrder:                     "desc",
	}

	cloned := cloneGenerationQueueQuery(original)
	if cloned == nil {
		t.Fatal("cloneGenerationQueueQuery() = nil, want clone")
	}
	if cloned == original {
		t.Fatal("cloneGenerationQueueQuery() returned original pointer")
	}
	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf("cloneGenerationQueueQuery() = %+v, want field-for-field clone of %+v", cloned, original)
	}

	cloned.Platform = "amazon"
	cloned.FromSectionKey = "section-2"
	cloned.RenderPreviewAvailable = false
	cloned.RenderPreviewAvailablePresent = false
	cloned.Retryable = false
	cloned.RetryablePresent = false
	cloned.Page = 5

	if original.Platform != "shein" ||
		original.FromSectionKey != "section-1" ||
		!original.RenderPreviewAvailable ||
		!original.RenderPreviewAvailablePresent ||
		!original.Retryable ||
		!original.RetryablePresent ||
		original.Page != 2 {
		t.Fatalf("original mutated after clone update = %+v", original)
	}
}

func TestCloneGenerationRetryGenerationTasksRequest(t *testing.T) {
	t.Parallel()

	if cloned := cloneRetryGenerationTasksRequest(nil); cloned != nil {
		t.Fatalf("cloneRetryGenerationTasksRequest(nil) = %+v, want nil", cloned)
	}

	original := &RetryGenerationTasksRequest{
		TaskIDs:               []string{"task-1", "task-2"},
		Slots:                 []string{"main", "detail"},
		ExecutionQuality:      "hq",
		ExecutionQualityLabel: "High Quality",
		QualityGrade:          "ideal",
		QualityGradeLabel:     "Ideal",
		FallbackOnly:          true,
		RendererOnly:          true,
	}

	cloned := cloneRetryGenerationTasksRequest(original)
	if cloned == nil {
		t.Fatal("cloneRetryGenerationTasksRequest() = nil, want clone")
	}
	if cloned == original {
		t.Fatal("cloneRetryGenerationTasksRequest() returned original pointer")
	}
	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf("cloneRetryGenerationTasksRequest() = %+v, want field-for-field clone of %+v", cloned, original)
	}
	if &cloned.TaskIDs[0] == &original.TaskIDs[0] || &cloned.Slots[0] == &original.Slots[0] {
		t.Fatalf("cloneRetryGenerationTasksRequest() = %+v, want defensive clone of slices", cloned)
	}

	cloned.TaskIDs[0] = "task-99"
	cloned.Slots[0] = "alt"
	cloned.ExecutionQuality = "draft"
	cloned.FallbackOnly = false

	if original.TaskIDs[0] != "task-1" ||
		original.Slots[0] != "main" ||
		original.ExecutionQuality != "hq" ||
		!original.FallbackOnly {
		t.Fatalf("original mutated after clone update = %+v", original)
	}
}
