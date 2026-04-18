package listingkit

import "testing"

func TestBuildGenerationReviewSessionPatchOmitsUnchangedSubpatches(t *testing.T) {
	t.Parallel()

	before := &GenerationReviewSession{
		SelectedPlatform:  "shein",
		SelectedSlot:      "main",
		FocusCapability:   "detail_preview",
		FocusedSectionKey: "detail_preview",
		Overview: &AssetGenerationOverview{
			PrimaryActionKey: "review_ready_assets",
		},
		ReviewSummary: &GenerationReviewSummary{
			ApprovedSections:      1,
			DeferredSections:      0,
			ReviewPendingSections: 0,
		},
		Queue: &GenerationWorkQueue{
			Summary: &GenerationWorkQueueSummary{
				TotalItems:            1,
				ApprovedSections:      1,
				DeferredSections:      0,
				ReviewPendingSections: 0,
			},
		},
		Sections: []GenerationReviewSection{{
			SectionKey:     "detail_preview",
			Capability:     "detail_preview",
			ReviewDecision: "approve",
			ReviewStatus:   "approved",
		}},
		SlotNavigation: []GenerationReviewSlot{{
			Platform:       "shein",
			Slot:           "main",
			ReviewDecision: "approve",
			ReviewStatus:   "approved",
		}},
		PlatformCards: []ListingKitPlatformCard{{
			Platform: "shein",
		}},
		PlatformRenderPreviews: []PlatformAssetRenderPreviews{{
			Platform: "shein",
		}},
	}
	after := cloneGenerationReviewSessionForTest(before)

	patch := buildGenerationReviewSessionPatch(before, after)
	if patch == nil {
		t.Fatal("patch = nil, want patch")
	}
	if patch.Focus != nil {
		t.Fatalf("patch.Focus = %+v, want nil for unchanged focus", patch.Focus)
	}
	if patch.SelectedPlatform != "" || patch.SelectedSlot != "" || patch.FocusCapability != "" || patch.FocusedSectionKey != "" {
		t.Fatalf("patch = %+v, want no root focus fields for unchanged focus", patch)
	}
	if patch.Queue != nil {
		t.Fatalf("patch.Queue = %+v, want nil for unchanged queue patch", patch.Queue)
	}
	if patch.Overview != nil || patch.ReviewSummary != nil || patch.QueueSummary != nil {
		t.Fatalf("patch = %+v, want no root summary fields for unchanged summaries", patch)
	}
	if patch.FocusedTarget != nil || patch.FocusedRenderPreview != nil || patch.FocusedToolbar != nil {
		t.Fatalf("patch = %+v, want no root focused payload for unchanged focus", patch)
	}
	if patch.PlatformCards != nil {
		t.Fatalf("patch.PlatformCards = %+v, want nil for unchanged platform cards", patch.PlatformCards)
	}
	if patch.RenderPreviews != nil {
		t.Fatalf("patch.RenderPreviews = %+v, want nil for unchanged render previews", patch.RenderPreviews)
	}
	if !isGenerationReviewSessionPatchEmpty(patch) {
		t.Fatalf("patch = %+v, want empty patch classification", patch)
	}
}

func cloneGenerationReviewSessionForTest(session *GenerationReviewSession) *GenerationReviewSession {
	if session == nil {
		return nil
	}
	cloned := *session
	if session.Overview != nil {
		overview := *session.Overview
		cloned.Overview = &overview
	}
	cloned.ReviewSummary = cloneGenerationReviewSummary(session.ReviewSummary)
	if session.Queue != nil {
		queue := *session.Queue
		if session.Queue.Summary != nil {
			summary := *session.Queue.Summary
			queue.Summary = &summary
		}
		queue.Items = append([]GenerationWorkQueueItem(nil), session.Queue.Items...)
		cloned.Queue = &queue
	}
	cloned.Sections = append([]GenerationReviewSection(nil), session.Sections...)
	cloned.SlotNavigation = append([]GenerationReviewSlot(nil), session.SlotNavigation...)
	cloned.PlatformCards = append([]ListingKitPlatformCard(nil), session.PlatformCards...)
	cloned.PlatformRenderPreviews = append([]PlatformAssetRenderPreviews(nil), session.PlatformRenderPreviews...)
	return &cloned
}
