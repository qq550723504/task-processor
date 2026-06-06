package listingkit

import (
	"reflect"
	"testing"
)

func TestCloneAssetGenerationActionTarget(t *testing.T) {
	t.Parallel()

	if cloned := cloneAssetGenerationActionTarget(nil); cloned != nil {
		t.Fatalf("cloneAssetGenerationActionTarget(nil) = %+v, want nil", cloned)
	}

	original := &AssetGenerationActionTarget{
		ActionKey:       "approve_section_review",
		InteractionMode: "review_only",
		Filters: &AssetGenerationRecommendedFilters{
			QualityGrade:           "ideal",
			QualityGradeLabel:      "Ideal",
			Platforms:              []string{"shein", "temu"},
			RetryableOnly:          true,
			ExecutionQuality:       "hq",
			RenderPreviewAvailable: true,
			PreviewCapability:      "detail_preview",
		},
		NavigationTarget: &GenerationReviewNavigationTarget{
			DispatchKind: "queue",
			QueueQuery: &GenerationQueueQuery{
				Platform:          "shein",
				Slot:              "main",
				PreviewCapability: "detail_preview",
				ResponseMode:      "full",
			},
			SessionQuery: &GenerationQueueQuery{
				Platform: "shein",
				Slot:     "detail",
			},
			PreviewQuery: &GenerationQueueQuery{
				Platform: "shein",
				AssetID:  "asset-preview-1",
			},
			ActionTarget: &AssetGenerationActionTarget{
				ActionKey:       "retry_failed_generations",
				InteractionMode: "retryable",
				QueueQuery: &GenerationQueueQuery{
					Platform: "shein",
					Slot:     "detail",
				},
				RetryRequest: &RetryGenerationTasksRequest{
					TaskIDs:          []string{"child-task-1"},
					Slots:            []string{"detail"},
					ExecutionQuality: "standard",
				},
				ExpectedImpact: &AssetGenerationActionImpact{
					Platforms:     []string{"shein"},
					QualityGrades: []string{"ideal"},
					States:        []string{"ready"},
				},
			},
		},
		QueueQuery: &GenerationQueueQuery{
			Platform:                      "shein",
			Slot:                          "main",
			FromCapability:                "detail_preview",
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
		},
		RetryRequest: &RetryGenerationTasksRequest{
			TaskIDs:               []string{"task-1", "task-2"},
			Slots:                 []string{"main", "detail"},
			ExecutionQuality:      "hq",
			ExecutionQualityLabel: "High Quality",
			QualityGrade:          "ideal",
			QualityGradeLabel:     "Ideal",
			FallbackOnly:          true,
		},
		ExpectedImpact: &AssetGenerationActionImpact{
			Platforms:     []string{"shein", "temu"},
			QualityGrades: []string{"ideal", "good"},
			States:        []string{"ready", "retryable"},
		},
	}

	cloned := cloneAssetGenerationActionTarget(original)
	if cloned == nil {
		t.Fatal("cloneAssetGenerationActionTarget() = nil, want clone")
	}
	if cloned == original {
		t.Fatal("cloneAssetGenerationActionTarget() returned original pointer")
	}
	if cloned.ActionKey != original.ActionKey ||
		cloned.InteractionMode != original.InteractionMode {
		t.Fatalf("cloneAssetGenerationActionTarget() = %+v, want top-level fields copied from %+v", cloned, original)
	}
	if cloned.Filters == nil || !reflect.DeepEqual(cloned.Filters, original.Filters) {
		t.Fatalf("cloneAssetGenerationActionTarget().Filters = %+v, want field-for-field clone of %+v", cloned.Filters, original.Filters)
	}
	if cloned.QueueQuery == nil || !reflect.DeepEqual(cloned.QueueQuery, original.QueueQuery) {
		t.Fatalf("cloneAssetGenerationActionTarget().QueueQuery = %+v, want field-for-field clone of %+v", cloned.QueueQuery, original.QueueQuery)
	}
	if cloned.RetryRequest == nil || !reflect.DeepEqual(cloned.RetryRequest, original.RetryRequest) {
		t.Fatalf("cloneAssetGenerationActionTarget().RetryRequest = %+v, want field-for-field clone of %+v", cloned.RetryRequest, original.RetryRequest)
	}
	if cloned.ExpectedImpact == nil || !reflect.DeepEqual(cloned.ExpectedImpact, original.ExpectedImpact) {
		t.Fatalf("cloneAssetGenerationActionTarget().ExpectedImpact = %+v, want field-for-field clone of %+v", cloned.ExpectedImpact, original.ExpectedImpact)
	}
	if cloned.NavigationTarget == nil {
		t.Fatal("cloneAssetGenerationActionTarget().NavigationTarget = nil, want clone")
	}
	if cloned.NavigationTarget.DispatchKind != original.NavigationTarget.DispatchKind {
		t.Fatalf("cloneAssetGenerationActionTarget().NavigationTarget.DispatchKind = %q, want %q", cloned.NavigationTarget.DispatchKind, original.NavigationTarget.DispatchKind)
	}
	if cloned.NavigationTarget.QueueQuery == nil || !reflect.DeepEqual(cloned.NavigationTarget.QueueQuery, original.NavigationTarget.QueueQuery) {
		t.Fatalf("cloneAssetGenerationActionTarget().NavigationTarget.QueueQuery = %+v, want field-for-field clone of %+v", cloned.NavigationTarget.QueueQuery, original.NavigationTarget.QueueQuery)
	}
	if cloned.NavigationTarget.SessionQuery == nil || !reflect.DeepEqual(cloned.NavigationTarget.SessionQuery, original.NavigationTarget.SessionQuery) {
		t.Fatalf("cloneAssetGenerationActionTarget().NavigationTarget.SessionQuery = %+v, want field-for-field clone of %+v", cloned.NavigationTarget.SessionQuery, original.NavigationTarget.SessionQuery)
	}
	if cloned.NavigationTarget.PreviewQuery == nil || !reflect.DeepEqual(cloned.NavigationTarget.PreviewQuery, original.NavigationTarget.PreviewQuery) {
		t.Fatalf("cloneAssetGenerationActionTarget().NavigationTarget.PreviewQuery = %+v, want field-for-field clone of %+v", cloned.NavigationTarget.PreviewQuery, original.NavigationTarget.PreviewQuery)
	}
	if cloned.NavigationTarget.ActionTarget == nil {
		t.Fatal("cloneAssetGenerationActionTarget().NavigationTarget.ActionTarget = nil, want clone")
	}
	if cloned.NavigationTarget.ActionTarget.ActionKey != original.NavigationTarget.ActionTarget.ActionKey ||
		cloned.NavigationTarget.ActionTarget.InteractionMode != original.NavigationTarget.ActionTarget.InteractionMode {
		t.Fatalf("cloneAssetGenerationActionTarget().NavigationTarget.ActionTarget = %+v, want copied action metadata from %+v", cloned.NavigationTarget.ActionTarget, original.NavigationTarget.ActionTarget)
	}
	if cloned.NavigationTarget.ActionTarget.QueueQuery == nil || !reflect.DeepEqual(cloned.NavigationTarget.ActionTarget.QueueQuery, original.NavigationTarget.ActionTarget.QueueQuery) {
		t.Fatalf("cloneAssetGenerationActionTarget().NavigationTarget.ActionTarget.QueueQuery = %+v, want field-for-field clone of %+v", cloned.NavigationTarget.ActionTarget.QueueQuery, original.NavigationTarget.ActionTarget.QueueQuery)
	}
	if cloned.NavigationTarget.ActionTarget.RetryRequest == nil || !reflect.DeepEqual(cloned.NavigationTarget.ActionTarget.RetryRequest, original.NavigationTarget.ActionTarget.RetryRequest) {
		t.Fatalf("cloneAssetGenerationActionTarget().NavigationTarget.ActionTarget.RetryRequest = %+v, want field-for-field clone of %+v", cloned.NavigationTarget.ActionTarget.RetryRequest, original.NavigationTarget.ActionTarget.RetryRequest)
	}
	if cloned.NavigationTarget.ActionTarget.ExpectedImpact == nil || !reflect.DeepEqual(cloned.NavigationTarget.ActionTarget.ExpectedImpact, original.NavigationTarget.ActionTarget.ExpectedImpact) {
		t.Fatalf("cloneAssetGenerationActionTarget().NavigationTarget.ActionTarget.ExpectedImpact = %+v, want field-for-field clone of %+v", cloned.NavigationTarget.ActionTarget.ExpectedImpact, original.NavigationTarget.ActionTarget.ExpectedImpact)
	}
	if cloned.Filters == original.Filters ||
		cloned.NavigationTarget == original.NavigationTarget ||
		cloned.NavigationTarget.QueueQuery == original.NavigationTarget.QueueQuery ||
		cloned.NavigationTarget.SessionQuery == original.NavigationTarget.SessionQuery ||
		cloned.NavigationTarget.PreviewQuery == original.NavigationTarget.PreviewQuery ||
		cloned.NavigationTarget.ActionTarget == original.NavigationTarget.ActionTarget ||
		cloned.QueueQuery == original.QueueQuery ||
		cloned.RetryRequest == original.RetryRequest ||
		cloned.ExpectedImpact == original.ExpectedImpact {
		t.Fatalf("cloneAssetGenerationActionTarget() = %+v, want defensive clone of nested pointers", cloned)
	}
	if &cloned.Filters.Platforms[0] == &original.Filters.Platforms[0] ||
		&cloned.RetryRequest.TaskIDs[0] == &original.RetryRequest.TaskIDs[0] ||
		&cloned.RetryRequest.Slots[0] == &original.RetryRequest.Slots[0] ||
		&cloned.ExpectedImpact.Platforms[0] == &original.ExpectedImpact.Platforms[0] ||
		&cloned.ExpectedImpact.QualityGrades[0] == &original.ExpectedImpact.QualityGrades[0] ||
		&cloned.ExpectedImpact.States[0] == &original.ExpectedImpact.States[0] ||
		&cloned.NavigationTarget.ActionTarget.RetryRequest.TaskIDs[0] == &original.NavigationTarget.ActionTarget.RetryRequest.TaskIDs[0] ||
		&cloned.NavigationTarget.ActionTarget.ExpectedImpact.Platforms[0] == &original.NavigationTarget.ActionTarget.ExpectedImpact.Platforms[0] {
		t.Fatalf("cloneAssetGenerationActionTarget() = %+v, want defensive clone of nested slices", cloned)
	}

	cloned.Filters.Platforms[0] = "amazon"
	cloned.NavigationTarget.QueueQuery.Platform = "amazon"
	cloned.NavigationTarget.SessionQuery.Platform = "amazon"
	cloned.NavigationTarget.PreviewQuery.AssetID = "asset-preview-2"
	cloned.NavigationTarget.ActionTarget.QueueQuery.Platform = "amazon"
	cloned.NavigationTarget.ActionTarget.RetryRequest.TaskIDs[0] = "child-task-2"
	cloned.NavigationTarget.ActionTarget.ExpectedImpact.Platforms[0] = "amazon"
	cloned.QueueQuery.Platform = "amazon"
	cloned.RetryRequest.TaskIDs[0] = "task-99"
	cloned.RetryRequest.Slots[0] = "alt"
	cloned.ExpectedImpact.Platforms[0] = "amazon"
	cloned.ExpectedImpact.QualityGrades[0] = "missing"
	cloned.ExpectedImpact.States[0] = "failed"

	if original.Filters.Platforms[0] != "shein" ||
		original.NavigationTarget.QueueQuery.Platform != "shein" ||
		original.NavigationTarget.SessionQuery.Platform != "shein" ||
		original.NavigationTarget.PreviewQuery.AssetID != "asset-preview-1" ||
		original.NavigationTarget.ActionTarget.QueueQuery.Platform != "shein" ||
		original.NavigationTarget.ActionTarget.RetryRequest.TaskIDs[0] != "child-task-1" ||
		original.NavigationTarget.ActionTarget.ExpectedImpact.Platforms[0] != "shein" ||
		original.QueueQuery.Platform != "shein" ||
		original.RetryRequest.TaskIDs[0] != "task-1" ||
		original.RetryRequest.Slots[0] != "main" ||
		original.ExpectedImpact.Platforms[0] != "shein" ||
		original.ExpectedImpact.QualityGrades[0] != "ideal" ||
		original.ExpectedImpact.States[0] != "ready" {
		t.Fatalf("original mutated after clone update = %+v", original)
	}
}

func TestCloneAssetGenerationActionImpact(t *testing.T) {
	t.Parallel()

	if cloned := cloneAssetGenerationActionImpact(nil); cloned != nil {
		t.Fatalf("cloneAssetGenerationActionImpact(nil) = %+v, want nil", cloned)
	}

	original := &AssetGenerationActionImpact{
		Platforms:     []string{"shein", "temu"},
		QualityGrades: []string{"ideal", "good"},
		States:        []string{"ready", "retryable"},
	}

	cloned := cloneAssetGenerationActionImpact(original)
	if cloned == nil {
		t.Fatal("cloneAssetGenerationActionImpact() = nil, want clone")
	}
	if cloned == original {
		t.Fatal("cloneAssetGenerationActionImpact() returned original pointer")
	}
	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf("cloneAssetGenerationActionImpact() = %+v, want field-for-field clone of %+v", cloned, original)
	}
	if &cloned.Platforms[0] == &original.Platforms[0] ||
		&cloned.QualityGrades[0] == &original.QualityGrades[0] ||
		&cloned.States[0] == &original.States[0] {
		t.Fatalf("cloneAssetGenerationActionImpact() = %+v, want defensive clone of slices", cloned)
	}

	cloned.Platforms[0] = "amazon"
	cloned.QualityGrades[0] = "missing"
	cloned.States[0] = "failed"

	if original.Platforms[0] != "shein" || original.QualityGrades[0] != "ideal" || original.States[0] != "ready" {
		t.Fatalf("original mutated after clone update = %+v", original)
	}
}
