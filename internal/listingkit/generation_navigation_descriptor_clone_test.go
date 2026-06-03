package listingkit

import (
	"reflect"
	"testing"
)

func TestCloneGenerationNavigationDescriptor(t *testing.T) {
	t.Parallel()

	if cloned := cloneGenerationNavigationDescriptor(nil); cloned != nil {
		t.Fatalf("cloneGenerationNavigationDescriptor(nil) = %+v, want nil", cloned)
	}

	original := testGenerationNavigationDescriptorForClone()
	cloned := cloneGenerationNavigationDescriptor(original)
	if cloned == nil {
		t.Fatal("cloneGenerationNavigationDescriptor() = nil, want clone")
	}
	if cloned == original {
		t.Fatal("cloneGenerationNavigationDescriptor() returned original pointer")
	}
	if cloned.ResourceKind != original.ResourceKind ||
		cloned.CacheKey != original.CacheKey ||
		cloned.CachePolicy != original.CachePolicy ||
		cloned.RefreshScope != original.RefreshScope {
		t.Fatalf("cloneGenerationNavigationDescriptor() = %+v, want copied descriptor metadata from %+v", cloned, original)
	}
	if cloned.Conditional == nil || !reflect.DeepEqual(cloned.Conditional, original.Conditional) {
		t.Fatalf("cloneGenerationNavigationDescriptor().Conditional = %+v, want field-for-field clone of %+v", cloned.Conditional, original.Conditional)
	}
	if cloned.DispatchPlan == nil || !reflect.DeepEqual(cloned.DispatchPlan, original.DispatchPlan) {
		t.Fatalf("cloneGenerationNavigationDescriptor().DispatchPlan = %+v, want field-for-field clone of %+v", cloned.DispatchPlan, original.DispatchPlan)
	}
	if !reflect.DeepEqual(cloned.Invalidates, original.Invalidates) {
		t.Fatalf("cloneGenerationNavigationDescriptor().Invalidates = %+v, want field-for-field clone of %+v", cloned.Invalidates, original.Invalidates)
	}
	if !reflect.DeepEqual(cloned.FollowUpReads, original.FollowUpReads) {
		t.Fatalf("cloneGenerationNavigationDescriptor().FollowUpReads = %+v, want field-for-field clone of %+v", cloned.FollowUpReads, original.FollowUpReads)
	}
	if cloned.Conditional == original.Conditional ||
		cloned.DispatchPlan == original.DispatchPlan ||
		&cloned.Invalidates[0] == &original.Invalidates[0] ||
		&cloned.FollowUpReads[0] == &original.FollowUpReads[0] {
		t.Fatalf("cloneGenerationNavigationDescriptor() = %+v, want defensive clone of nested pointers", cloned)
	}
	if &cloned.DispatchPlan.Steps[0] == &original.DispatchPlan.Steps[0] ||
		&cloned.FollowUpReads[0].Query == &original.FollowUpReads[0].Query ||
		cloned.DispatchPlan.Steps[0].Query == original.DispatchPlan.Steps[0].Query ||
		cloned.FollowUpReads[0].Query == original.FollowUpReads[0].Query {
		t.Fatalf("cloneGenerationNavigationDescriptor() = %+v, want defensive clone of nested query pointers", cloned)
	}

	originalConditionalToken := original.Conditional.DeltaToken
	originalStepPlatform := original.DispatchPlan.Steps[0].Query.Platform
	originalInvalidate := original.Invalidates[0]
	originalFollowUpPlatform := original.FollowUpReads[0].Query.Platform

	cloned.Conditional.DeltaToken = "delta-2"
	cloned.DispatchPlan.Steps[0].Query.Platform = "temu"
	cloned.Invalidates[0] = "queue:temu"
	cloned.FollowUpReads[0].Query.Platform = "temu"

	if original.Conditional.DeltaToken != originalConditionalToken ||
		original.DispatchPlan.Steps[0].Query.Platform != originalStepPlatform ||
		original.Invalidates[0] != originalInvalidate ||
		original.FollowUpReads[0].Query.Platform != originalFollowUpPlatform {
		t.Fatalf("original mutated after clone update = %+v", original)
	}
}

func TestCloneGenerationNavigationDispatchPlan(t *testing.T) {
	t.Parallel()

	if cloned := cloneGenerationNavigationDispatchPlan(nil); cloned != nil {
		t.Fatalf("cloneGenerationNavigationDispatchPlan(nil) = %+v, want nil", cloned)
	}

	original := &GenerationNavigationDispatchPlan{
		Strategy:           "parallel",
		StopOnNotModified:  true,
		StopOnFirstSuccess: true,
		StopOnError:        true,
		FallbackStrategy:   "prefer_preview",
		MaxParallelism:     3,
		DedupePolicy:       "by_step_identity",
		WinnerPolicy:       "prefer_preview_then_session_then_queue",
		RequiresRevalidate: true,
		Steps: []GenerationNavigationDispatchStep{
			{
				Kind:               "queue",
				ResponseMode:       "full",
				CachePreference:    "prefer_cache",
				RequiresRevalidate: true,
				Query: &GenerationQueueQuery{
					Platform:          "shein",
					Slot:              "main",
					PreviewCapability: "detail_preview",
					ResponseMode:      "full",
				},
			},
		},
	}

	cloned := cloneGenerationNavigationDispatchPlan(original)
	if cloned == nil {
		t.Fatal("cloneGenerationNavigationDispatchPlan() = nil, want clone")
	}
	if cloned == original {
		t.Fatal("cloneGenerationNavigationDispatchPlan() returned original pointer")
	}
	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf("cloneGenerationNavigationDispatchPlan() = %+v, want field-for-field clone of %+v", cloned, original)
	}
	if &cloned.Steps[0] == &original.Steps[0] || cloned.Steps[0].Query == original.Steps[0].Query {
		t.Fatalf("cloneGenerationNavigationDispatchPlan() = %+v, want defensive clone of nested step/query pointers", cloned)
	}

	originalStepPlatform := original.Steps[0].Query.Platform
	cloned.Steps[0].Query.Platform = "temu"
	if original.Steps[0].Query.Platform != originalStepPlatform {
		t.Fatalf("original mutated after clone update = %+v", original)
	}
}

func testGenerationNavigationDescriptorForClone() *GenerationNavigationDescriptor {
	return &GenerationNavigationDescriptor{
		ResourceKind:                 "review_session",
		CacheKey:                     "cache-key-1",
		CachePolicy:                  "stale_while_revalidate",
		SupportsStaleWhileRevalidate: true,
		RevalidateAfterAction:        true,
		RefreshScope:                 "session",
		Invalidates:                  []string{"queue:shein", "preview:detail"},
		DispatchPlan: &GenerationNavigationDispatchPlan{
			Strategy:           "parallel",
			StopOnNotModified:  true,
			StopOnFirstSuccess: true,
			StopOnError:        true,
			FallbackStrategy:   "prefer_preview",
			MaxParallelism:     3,
			DedupePolicy:       "by_step_identity",
			WinnerPolicy:       "prefer_preview_then_session_then_queue",
			RequiresRevalidate: true,
			Steps: []GenerationNavigationDispatchStep{
				{
					Kind:               "queue",
					ResponseMode:       "full",
					CachePreference:    "prefer_cache",
					RequiresRevalidate: true,
					Query: &GenerationQueueQuery{
						Platform:          "shein",
						Slot:              "main",
						PreviewCapability: "detail_preview",
						ResponseMode:      "full",
					},
				},
			},
		},
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
			DeltaToken: "delta-1",
			ETag:       "etag-1",
			NoChanges:  true,
		},
	}
}
