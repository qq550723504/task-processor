package listingkit

import (
	"reflect"
	"testing"
)

func TestCloneGenerationReviewNavigationTarget(t *testing.T) {
	t.Parallel()

	if cloned := cloneGenerationReviewNavigationTarget(nil); cloned != nil {
		t.Fatalf("cloneGenerationReviewNavigationTarget(nil) = %+v, want nil", cloned)
	}

	original := testGenerationReviewNavigationTargetForClone()
	cloned := cloneGenerationReviewNavigationTarget(original)
	if cloned == nil {
		t.Fatal("cloneGenerationReviewNavigationTarget() = nil, want clone")
	}
	if cloned == original {
		t.Fatal("cloneGenerationReviewNavigationTarget() returned original pointer")
	}
	if cloned.DispatchKind != original.DispatchKind {
		t.Fatalf("cloneGenerationReviewNavigationTarget().DispatchKind = %q, want %q", cloned.DispatchKind, original.DispatchKind)
	}
	if cloned.Conditional == nil || !reflect.DeepEqual(cloned.Conditional, original.Conditional) {
		t.Fatalf("cloneGenerationReviewNavigationTarget().Conditional = %+v, want field-for-field clone of %+v", cloned.Conditional, original.Conditional)
	}
	if cloned.Descriptor == nil || !reflect.DeepEqual(cloned.Descriptor, original.Descriptor) {
		t.Fatalf("cloneGenerationReviewNavigationTarget().Descriptor = %+v, want field-for-field clone of %+v", cloned.Descriptor, original.Descriptor)
	}
	if cloned.QueueQuery == nil || !reflect.DeepEqual(cloned.QueueQuery, original.QueueQuery) {
		t.Fatalf("cloneGenerationReviewNavigationTarget().QueueQuery = %+v, want field-for-field clone of %+v", cloned.QueueQuery, original.QueueQuery)
	}
	if cloned.SessionQuery == nil || !reflect.DeepEqual(cloned.SessionQuery, original.SessionQuery) {
		t.Fatalf("cloneGenerationReviewNavigationTarget().SessionQuery = %+v, want field-for-field clone of %+v", cloned.SessionQuery, original.SessionQuery)
	}
	if cloned.PreviewQuery == nil || !reflect.DeepEqual(cloned.PreviewQuery, original.PreviewQuery) {
		t.Fatalf("cloneGenerationReviewNavigationTarget().PreviewQuery = %+v, want field-for-field clone of %+v", cloned.PreviewQuery, original.PreviewQuery)
	}
	if cloned.ActionTarget == nil || !reflect.DeepEqual(cloned.ActionTarget, original.ActionTarget) {
		t.Fatalf("cloneGenerationReviewNavigationTarget().ActionTarget = %+v, want field-for-field clone of %+v", cloned.ActionTarget, original.ActionTarget)
	}
	if cloned.Conditional == original.Conditional ||
		cloned.Descriptor == original.Descriptor ||
		cloned.QueueQuery == original.QueueQuery ||
		cloned.SessionQuery == original.SessionQuery ||
		cloned.PreviewQuery == original.PreviewQuery ||
		cloned.ActionTarget == original.ActionTarget {
		t.Fatalf("cloneGenerationReviewNavigationTarget() = %+v, want defensive clone of nested pointers", cloned)
	}
	if &cloned.Descriptor.Invalidates[0] == &original.Descriptor.Invalidates[0] ||
		&cloned.Descriptor.FollowUpReads[0] == &original.Descriptor.FollowUpReads[0] ||
		&cloned.ActionTarget.RetryRequest.TaskIDs[0] == &original.ActionTarget.RetryRequest.TaskIDs[0] ||
		&cloned.ActionTarget.ExpectedImpact.Platforms[0] == &original.ActionTarget.ExpectedImpact.Platforms[0] {
		t.Fatalf("cloneGenerationReviewNavigationTarget() = %+v, want defensive clone of nested slices", cloned)
	}

	expected := applyIdentityToNavigationTarget(&GenerationReviewNavigationTarget{
		DispatchKind: "action",
		Conditional:  cloneGenerationConditionalState(original.Conditional),
		Descriptor:   cloneGenerationNavigationDescriptor(original.Descriptor),
		QueueQuery:   cloneGenerationQueueQuery(original.QueueQuery),
		SessionQuery: cloneGenerationQueueQuery(original.SessionQuery),
		PreviewQuery: cloneGenerationQueueQuery(original.PreviewQuery),
		ActionTarget: cloneAssetGenerationActionTarget(original.ActionTarget),
	})
	if cloned.ResourceKind != expected.ResourceKind ||
		cloned.CacheKey != expected.CacheKey ||
		cloned.CachePolicy != expected.CachePolicy ||
		cloned.RevalidateAfterAction != expected.RevalidateAfterAction ||
		!reflect.DeepEqual(cloned.Descriptor, expected.Descriptor) {
		t.Fatalf("cloneGenerationReviewNavigationTarget() = %+v, want outward identity aligned with %+v", cloned, expected)
	}

	originalConditionalDelta := original.Conditional.DeltaToken
	originalDescriptorInvalidate := original.Descriptor.Invalidates[0]
	originalDescriptorFollowUpPlatform := original.Descriptor.FollowUpReads[0].Query.Platform
	originalQueuePlatform := original.QueueQuery.Platform
	originalSessionPlatform := original.SessionQuery.Platform
	originalPreviewAssetID := original.PreviewQuery.AssetID
	originalActionTargetQueuePlatform := original.ActionTarget.QueueQuery.Platform
	originalActionTargetRetryTaskID := original.ActionTarget.RetryRequest.TaskIDs[0]
	originalActionTargetImpactPlatform := original.ActionTarget.ExpectedImpact.Platforms[0]

	cloned.Conditional.DeltaToken = "delta-2"
	cloned.Descriptor.Invalidates[0] = "queue:temu"
	cloned.Descriptor.FollowUpReads[0].Query.Platform = "temu"
	cloned.QueueQuery.Platform = "temu"
	cloned.SessionQuery.Platform = "temu"
	cloned.PreviewQuery.AssetID = "asset-preview-2"
	cloned.ActionTarget.QueueQuery.Platform = "temu"
	cloned.ActionTarget.RetryRequest.TaskIDs[0] = "child-task-2"
	cloned.ActionTarget.ExpectedImpact.Platforms[0] = "temu"

	if original.Conditional.DeltaToken != originalConditionalDelta ||
		original.Descriptor.Invalidates[0] != originalDescriptorInvalidate ||
		original.Descriptor.FollowUpReads[0].Query.Platform != originalDescriptorFollowUpPlatform ||
		original.QueueQuery.Platform != originalQueuePlatform ||
		original.SessionQuery.Platform != originalSessionPlatform ||
		original.PreviewQuery.AssetID != originalPreviewAssetID ||
		original.ActionTarget.QueueQuery.Platform != originalActionTargetQueuePlatform ||
		original.ActionTarget.RetryRequest.TaskIDs[0] != originalActionTargetRetryTaskID ||
		original.ActionTarget.ExpectedImpact.Platforms[0] != originalActionTargetImpactPlatform {
		t.Fatalf("original mutated after clone update = %+v", original)
	}
}

func TestCloneAssetGenerationActionTargetForNavigation(t *testing.T) {
	t.Parallel()

	if cloned := cloneAssetGenerationActionTargetForNavigation(nil); cloned != nil {
		t.Fatalf("cloneAssetGenerationActionTargetForNavigation(nil) = %+v, want nil", cloned)
	}

	original := testAssetGenerationActionTargetForNavigation()

	cloned := cloneAssetGenerationActionTargetForNavigation(original)
	if cloned == nil {
		t.Fatal("cloneAssetGenerationActionTargetForNavigation() = nil, want clone")
	}
	if cloned == original {
		t.Fatal("cloneAssetGenerationActionTargetForNavigation() returned original pointer")
	}
	if cloned.ActionKey != original.ActionKey || cloned.InteractionMode != original.InteractionMode {
		t.Fatalf("cloneAssetGenerationActionTargetForNavigation() = %+v, want copied action metadata from %+v", cloned, original)
	}
	if cloned.NavigationTarget != nil {
		t.Fatalf("cloneAssetGenerationActionTargetForNavigation().NavigationTarget = %+v, want nil", cloned.NavigationTarget)
	}
	if original.NavigationTarget == nil {
		t.Fatal("original.NavigationTarget = nil, want original navigation target preserved")
	}
	if cloned.Filters == nil || !reflect.DeepEqual(cloned.Filters, original.Filters) {
		t.Fatalf("cloneAssetGenerationActionTargetForNavigation().Filters = %+v, want field-for-field clone of %+v", cloned.Filters, original.Filters)
	}
	if cloned.QueueQuery == nil || !reflect.DeepEqual(cloned.QueueQuery, original.QueueQuery) {
		t.Fatalf("cloneAssetGenerationActionTargetForNavigation().QueueQuery = %+v, want field-for-field clone of %+v", cloned.QueueQuery, original.QueueQuery)
	}
	if cloned.RetryRequest == nil || !reflect.DeepEqual(cloned.RetryRequest, original.RetryRequest) {
		t.Fatalf("cloneAssetGenerationActionTargetForNavigation().RetryRequest = %+v, want field-for-field clone of %+v", cloned.RetryRequest, original.RetryRequest)
	}
	if cloned.ExpectedImpact == nil || !reflect.DeepEqual(cloned.ExpectedImpact, original.ExpectedImpact) {
		t.Fatalf("cloneAssetGenerationActionTargetForNavigation().ExpectedImpact = %+v, want field-for-field clone of %+v", cloned.ExpectedImpact, original.ExpectedImpact)
	}
	if cloned.Filters == original.Filters ||
		cloned.QueueQuery == original.QueueQuery ||
		cloned.RetryRequest == original.RetryRequest ||
		cloned.ExpectedImpact == original.ExpectedImpact {
		t.Fatalf("cloneAssetGenerationActionTargetForNavigation() = %+v, want defensive clone of nested pointers", cloned)
	}
	if &cloned.Filters.Platforms[0] == &original.Filters.Platforms[0] ||
		&cloned.RetryRequest.TaskIDs[0] == &original.RetryRequest.TaskIDs[0] ||
		&cloned.RetryRequest.Slots[0] == &original.RetryRequest.Slots[0] ||
		&cloned.ExpectedImpact.Platforms[0] == &original.ExpectedImpact.Platforms[0] ||
		&cloned.ExpectedImpact.QualityGrades[0] == &original.ExpectedImpact.QualityGrades[0] ||
		&cloned.ExpectedImpact.States[0] == &original.ExpectedImpact.States[0] {
		t.Fatalf("cloneAssetGenerationActionTargetForNavigation() = %+v, want defensive clone of nested slices", cloned)
	}

	cloned.Filters.Platforms[0] = "amazon"
	cloned.QueueQuery.Platform = "amazon"
	cloned.RetryRequest.TaskIDs[0] = "task-99"
	cloned.RetryRequest.Slots[0] = "alt"
	cloned.ExpectedImpact.Platforms[0] = "amazon"
	cloned.ExpectedImpact.QualityGrades[0] = "missing"
	cloned.ExpectedImpact.States[0] = "failed"

	if original.Filters.Platforms[0] != "shein" ||
		original.QueueQuery.Platform != "shein" ||
		original.RetryRequest.TaskIDs[0] != "task-1" ||
		original.RetryRequest.Slots[0] != "main" ||
		original.ExpectedImpact.Platforms[0] != "shein" ||
		original.ExpectedImpact.QualityGrades[0] != "ideal" ||
		original.ExpectedImpact.States[0] != "ready" {
		t.Fatalf("original mutated after clone update = %+v", original)
	}
}

func TestGenerationReviewActionNavigationTarget(t *testing.T) {
	t.Parallel()

	if target := buildGenerationReviewActionNavigationTarget(nil); target != nil {
		t.Fatalf("buildGenerationReviewActionNavigationTarget(nil) = %+v, want nil", target)
	}

	original := testAssetGenerationActionTargetForNavigation()

	actual := buildGenerationReviewActionNavigationTarget(original)
	if actual == nil {
		t.Fatal("buildGenerationReviewActionNavigationTarget() = nil, want navigation target")
	}
	if actual.DispatchKind != "action" {
		t.Fatalf("buildGenerationReviewActionNavigationTarget().DispatchKind = %q, want %q", actual.DispatchKind, "action")
	}
	if actual.ActionTarget == nil {
		t.Fatal("buildGenerationReviewActionNavigationTarget().ActionTarget = nil, want cloned action target")
	}
	if actual.ActionTarget.NavigationTarget != nil {
		t.Fatalf("buildGenerationReviewActionNavigationTarget().ActionTarget.NavigationTarget = %+v, want nil", actual.ActionTarget.NavigationTarget)
	}
	if actual.QueueQuery == nil || !reflect.DeepEqual(actual.QueueQuery, original.QueueQuery) {
		t.Fatalf("buildGenerationReviewActionNavigationTarget().QueueQuery = %+v, want field-for-field clone of %+v", actual.QueueQuery, original.QueueQuery)
	}
	if actual.QueueQuery == original.QueueQuery {
		t.Fatal("buildGenerationReviewActionNavigationTarget().QueueQuery reused original pointer")
	}
	if actual.ActionTarget.Filters == nil || !reflect.DeepEqual(actual.ActionTarget.Filters, original.Filters) {
		t.Fatalf("buildGenerationReviewActionNavigationTarget().ActionTarget.Filters = %+v, want field-for-field clone of %+v", actual.ActionTarget.Filters, original.Filters)
	}
	if actual.ActionTarget.QueueQuery == nil || !reflect.DeepEqual(actual.ActionTarget.QueueQuery, original.QueueQuery) {
		t.Fatalf("buildGenerationReviewActionNavigationTarget().ActionTarget.QueueQuery = %+v, want field-for-field clone of %+v", actual.ActionTarget.QueueQuery, original.QueueQuery)
	}
	if actual.ActionTarget.RetryRequest == nil || !reflect.DeepEqual(actual.ActionTarget.RetryRequest, original.RetryRequest) {
		t.Fatalf("buildGenerationReviewActionNavigationTarget().ActionTarget.RetryRequest = %+v, want field-for-field clone of %+v", actual.ActionTarget.RetryRequest, original.RetryRequest)
	}
	if actual.ActionTarget.ExpectedImpact == nil || !reflect.DeepEqual(actual.ActionTarget.ExpectedImpact, original.ExpectedImpact) {
		t.Fatalf("buildGenerationReviewActionNavigationTarget().ActionTarget.ExpectedImpact = %+v, want field-for-field clone of %+v", actual.ActionTarget.ExpectedImpact, original.ExpectedImpact)
	}
	if actual.ActionTarget == original ||
		actual.ActionTarget.Filters == original.Filters ||
		actual.ActionTarget.QueueQuery == original.QueueQuery ||
		actual.ActionTarget.RetryRequest == original.RetryRequest ||
		actual.ActionTarget.ExpectedImpact == original.ExpectedImpact {
		t.Fatalf("buildGenerationReviewActionNavigationTarget().ActionTarget = %+v, want defensive clone of nested pointers", actual.ActionTarget)
	}
	if &actual.ActionTarget.Filters.Platforms[0] == &original.Filters.Platforms[0] ||
		&actual.ActionTarget.RetryRequest.TaskIDs[0] == &original.RetryRequest.TaskIDs[0] ||
		&actual.ActionTarget.RetryRequest.Slots[0] == &original.RetryRequest.Slots[0] ||
		&actual.ActionTarget.ExpectedImpact.Platforms[0] == &original.ExpectedImpact.Platforms[0] ||
		&actual.ActionTarget.ExpectedImpact.QualityGrades[0] == &original.ExpectedImpact.QualityGrades[0] ||
		&actual.ActionTarget.ExpectedImpact.States[0] == &original.ExpectedImpact.States[0] {
		t.Fatalf("buildGenerationReviewActionNavigationTarget().ActionTarget = %+v, want defensive clone of nested slices", actual.ActionTarget)
	}

	expected := applyIdentityToNavigationTarget(&GenerationReviewNavigationTarget{
		DispatchKind: "action",
		ActionTarget: cloneAssetGenerationActionTargetForNavigation(original),
		QueueQuery:   cloneGenerationQueueQuery(original.QueueQuery),
	})
	if actual.ResourceKind != expected.ResourceKind ||
		actual.CacheKey != expected.CacheKey ||
		actual.CachePolicy != expected.CachePolicy ||
		actual.RevalidateAfterAction != expected.RevalidateAfterAction ||
		!reflect.DeepEqual(actual.Descriptor, expected.Descriptor) {
		t.Fatalf("buildGenerationReviewActionNavigationTarget() = %+v, want outward identity aligned with %+v", actual, expected)
	}

	actual.QueueQuery.Platform = "amazon"
	actual.ActionTarget.Filters.Platforms[0] = "amazon"
	actual.ActionTarget.QueueQuery.Platform = "amazon"
	actual.ActionTarget.RetryRequest.TaskIDs[0] = "task-99"
	actual.ActionTarget.ExpectedImpact.Platforms[0] = "amazon"

	if original.QueueQuery.Platform != "shein" ||
		original.Filters.Platforms[0] != "shein" ||
		original.RetryRequest.TaskIDs[0] != "task-1" ||
		original.ExpectedImpact.Platforms[0] != "shein" {
		t.Fatalf("original mutated after navigation target update = %+v", original)
	}
	if original.NavigationTarget == nil {
		t.Fatal("original.NavigationTarget = nil, want original navigation target preserved")
	}
}

func TestGenerationReviewActionNavigationTargetQueueQueryClone(t *testing.T) {
	t.Parallel()

	original := testAssetGenerationActionTargetForNavigation()

	actual := buildGenerationReviewActionNavigationTarget(original)
	if actual == nil {
		t.Fatal("buildGenerationReviewActionNavigationTarget() = nil, want navigation target")
	}
	if actual.QueueQuery == nil {
		t.Fatal("buildGenerationReviewActionNavigationTarget().QueueQuery = nil, want clone")
	}
	if actual.QueueQuery == original.QueueQuery {
		t.Fatal("buildGenerationReviewActionNavigationTarget().QueueQuery reused original pointer")
	}
	if !reflect.DeepEqual(actual.QueueQuery, original.QueueQuery) {
		t.Fatalf("buildGenerationReviewActionNavigationTarget().QueueQuery = %+v, want field-for-field clone of %+v", actual.QueueQuery, original.QueueQuery)
	}

	expected := applyIdentityToNavigationTarget(&GenerationReviewNavigationTarget{
		DispatchKind: "action",
		ActionTarget: cloneAssetGenerationActionTargetForNavigation(original),
		QueueQuery:   cloneGenerationQueueQuery(original.QueueQuery),
	})
	if actual.ResourceKind != expected.ResourceKind ||
		actual.CacheKey != expected.CacheKey ||
		actual.CachePolicy != expected.CachePolicy ||
		actual.RevalidateAfterAction != expected.RevalidateAfterAction ||
		!reflect.DeepEqual(actual.Descriptor, expected.Descriptor) {
		t.Fatalf("buildGenerationReviewActionNavigationTarget() = %+v, want outward identity aligned with %+v", actual, expected)
	}

	actual.QueueQuery.Platform = "amazon"
	actual.QueueQuery.SortBy = "updated_at"
	actual.QueueQuery.Page = 99

	if original.QueueQuery.Platform != "shein" ||
		original.QueueQuery.SortBy != "updated_at" ||
		original.QueueQuery.Page != 2 {
		t.Fatalf("original.QueueQuery mutated after navigation queue update = %+v", original.QueueQuery)
	}
}

func testAssetGenerationActionTargetForNavigation() *AssetGenerationActionTarget {
	return &AssetGenerationActionTarget{
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
		NavigationTarget: applyIdentityToNavigationTarget(&GenerationReviewNavigationTarget{
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
		}),
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
}

func testGenerationReviewNavigationTargetForClone() *GenerationReviewNavigationTarget {
	return applyIdentityToNavigationTarget(&GenerationReviewNavigationTarget{
		DispatchKind: "action",
		Conditional: &GenerationConditionalState{
			DeltaToken: "delta-1",
			ETag:       "etag-1",
			NoChanges:  true,
		},
		Descriptor: &GenerationNavigationDescriptor{
			ResourceKind:                 "review_session",
			CacheKey:                     "cache-key-1",
			CachePolicy:                  "stale_while_revalidate",
			SupportsStaleWhileRevalidate: true,
			RevalidateAfterAction:        true,
			RefreshScope:                 "session",
			Invalidates:                  []string{"queue:shein", "preview:detail"},
			FollowUpReads: []GenerationNavigationFollowUpRead{
				{
					Kind:         "queue",
					ResponseMode: "patch_only",
					Query: &GenerationQueueQuery{
						Platform:          "shein",
						Slot:              "main",
						PreviewCapability: "detail_preview",
						ResponseMode:      "patch_only",
					},
				},
			},
			Conditional: &GenerationConditionalState{
				DeltaToken: "delta-descriptor",
				ETag:       "etag-descriptor",
				NoChanges:  true,
			},
		},
		QueueQuery: &GenerationQueueQuery{
			Platform:          "shein",
			Slot:              "main",
			PreviewCapability: "detail_preview",
			ResponseMode:      "full",
		},
		SessionQuery: &GenerationQueueQuery{
			Platform:          "shein",
			Slot:              "detail",
			PreviewCapability: "detail_preview",
			ResponseMode:      "patch_only",
		},
		PreviewQuery: &GenerationQueueQuery{
			Platform:          "shein",
			AssetID:           "asset-preview-1",
			PreviewCapability: "detail_preview",
			ResponseMode:      "patch_only",
		},
		ActionTarget: &AssetGenerationActionTarget{
			ActionKey:       "retry_section_generation",
			InteractionMode: "review_only",
			QueueQuery: &GenerationQueueQuery{
				Platform:          "shein",
				Slot:              "main",
				PreviewCapability: "detail_preview",
				ResponseMode:      "full",
			},
			RetryRequest: &RetryGenerationTasksRequest{
				TaskIDs:               []string{"child-task-1"},
				Slots:                 []string{"main"},
				ExecutionQuality:      "hq",
				ExecutionQualityLabel: "High Quality",
				QualityGrade:          "ideal",
				QualityGradeLabel:     "Ideal",
			},
			ExpectedImpact: &AssetGenerationActionImpact{
				Platforms:     []string{"shein"},
				QualityGrades: []string{"ideal"},
				States:        []string{"ready"},
			},
		},
	})
}
