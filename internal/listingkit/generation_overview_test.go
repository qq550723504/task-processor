package listingkit

import (
	"strings"
	"testing"
)

func TestBuildAssetGenerationOverviewPrefersMissingAction(t *testing.T) {
	t.Parallel()

	queue := &GenerationWorkQueue{
		Summary: &GenerationWorkQueueSummary{
			MissingItems:       1,
			RetryableItems:     2,
			QualityGradeCounts: map[string]int{"missing": 1},
			PlatformQualityGradeCounts: map[string]map[string]int{
				"amazon": {"missing": 1},
			},
		},
	}

	overview := buildAssetGenerationOverview(queue)
	if overview == nil || overview.PrimaryActionKey != "generate_missing_assets" {
		t.Fatalf("overview = %+v, want missing-assets action key", overview)
	}
	if overview.RecommendedFilters == nil || overview.RecommendedFilters.QualityGrade != "missing" {
		t.Fatalf("overview = %+v, want missing grade filter", overview)
	}
	if overview.PrimaryActionTarget == nil || overview.PrimaryActionTarget.QueueQuery == nil || overview.PrimaryActionTarget.QueueQuery.QualityGrade != "missing" {
		t.Fatalf("overview = %+v, want executable queue target", overview)
	}
	if overview.PrimaryActionTarget.RetryRequest == nil || overview.PrimaryActionTarget.RetryRequest.QualityGrade != "missing" {
		t.Fatalf("overview = %+v, want executable retry target", overview)
	}
	if overview.PrimaryActionTarget.InteractionMode != "retryable" {
		t.Fatalf("overview = %+v, want retryable interaction mode", overview)
	}
	if overview.PrimaryActionTarget.ExpectedImpact == nil || overview.PrimaryActionTarget.ExpectedImpact.MatchedItems != 0 {
		t.Fatalf("overview = %+v, want impact summary present", overview)
	}
}

func TestBuildAssetGenerationOverviewPrefersProvisionalUpgrade(t *testing.T) {
	t.Parallel()

	queue := &GenerationWorkQueue{
		Summary: &GenerationWorkQueueSummary{
			RetryableItems:     1,
			QualityGradeCounts: map[string]int{"provisional": 1},
			PlatformQualityGradeCounts: map[string]map[string]int{
				"shein": {"provisional": 1},
			},
		},
	}

	overview := buildAssetGenerationOverview(queue)
	if overview == nil || overview.PrimaryActionKey != "upgrade_fallback_assets" {
		t.Fatalf("overview = %+v, want upgrade-fallback action key", overview)
	}
	if overview.BlockingQualityGrades[0] != "provisional" {
		t.Fatalf("overview = %+v, want provisional blocking grade", overview)
	}
	if overview.PrimaryActionTarget == nil || overview.PrimaryActionTarget.RetryRequest == nil || overview.PrimaryActionTarget.RetryRequest.QualityGrade != "provisional" {
		t.Fatalf("overview = %+v, want provisional retry target", overview)
	}
	if overview.PrimaryActionTarget.InteractionMode != "retryable" {
		t.Fatalf("overview = %+v, want retryable interaction mode", overview)
	}
	if overview.PrimaryActionTarget.ExpectedImpact == nil {
		t.Fatalf("overview = %+v, want impact summary", overview)
	}
}

func TestBuildAssetGenerationOverviewReturnsReviewActionForHealthyQueue(t *testing.T) {
	t.Parallel()

	queue := &GenerationWorkQueue{
		Summary: &GenerationWorkQueueSummary{
			QualityGradeCounts: map[string]int{"ideal": 1},
			PreviewableItems:   1,
			PreviewCapabilityCounts: map[string]int{
				"detail_preview": 1,
				"badge_preview":  1,
			},
			PlatformQualityGradeCounts: map[string]map[string]int{
				"amazon": {"ideal": 1},
			},
			PlatformPreviewableCounts: map[string]int{
				"amazon": 1,
			},
		},
	}

	overview := buildAssetGenerationOverview(queue)
	if overview == nil || overview.PrimaryActionKey != "continue_publish_review" {
		t.Fatalf("overview = %+v, want continue-publish action key", overview)
	}
	if overview.PrimaryActionTarget == nil || overview.PrimaryActionTarget.RetryRequest != nil {
		t.Fatalf("overview = %+v, want review action without retry target", overview)
	}
	if overview.PrimaryActionTarget.InteractionMode != "review_only" {
		t.Fatalf("overview = %+v, want review_only interaction mode", overview)
	}
	if overview.PrimaryActionTarget.ExpectedImpact == nil {
		t.Fatalf("overview = %+v, want impact summary", overview)
	}
	if overview.PreviewableItems != 1 || len(overview.PreviewReadyPlatforms) != 1 || overview.PreviewReadyPlatforms[0] != "amazon" {
		t.Fatalf("overview = %+v, want preview-ready summary", overview)
	}
	if overview.PreviewCapabilityCounts["detail_preview"] != 1 || len(overview.PreviewReadyCapabilities) != 2 {
		t.Fatalf("overview = %+v, want preview capability summary", overview)
	}
	if overview.PrimaryActionReason == "" || !containsIgnoreCase(overview.PrimaryActionReason, "preview") {
		t.Fatalf("overview = %+v, want preview-aware review reason", overview)
	}
	if overview.RecommendedFilters == nil || !overview.RecommendedFilters.RenderPreviewAvailable {
		t.Fatalf("overview = %+v, want review filter to prefer previewable items", overview)
	}
	if overview.PrimaryActionTarget == nil || overview.PrimaryActionTarget.QueueQuery == nil || !overview.PrimaryActionTarget.QueueQuery.RenderPreviewAvailable || !overview.PrimaryActionTarget.QueueQuery.RenderPreviewAvailablePresent {
		t.Fatalf("overview = %+v, want executable preview-aware queue target", overview)
	}
	if len(overview.SecondaryActionKeys) < 2 {
		t.Fatalf("overview = %+v, want preview capability secondary actions", overview)
	}
	if overview.SecondaryActionKeys[0] != "review_detail_previews" {
		t.Fatalf("secondary action keys = %+v, want detail review action first", overview.SecondaryActionKeys)
	}
	if overview.SecondaryActionTargets == nil || len(overview.SecondaryActionTargets) < 2 {
		t.Fatalf("overview = %+v, want executable secondary action targets", overview)
	}
	detailTarget := overview.SecondaryActionTargets[0]
	if detailTarget.QueueQuery == nil || detailTarget.QueueQuery.PreviewCapability != "detail_preview" {
		t.Fatalf("detail target = %+v, want detail preview queue query", detailTarget)
	}
	if !detailTarget.QueueQuery.RenderPreviewAvailable || !detailTarget.QueueQuery.RenderPreviewAvailablePresent {
		t.Fatalf("detail target = %+v, want preview-available queue filter", detailTarget)
	}
	if detailTarget.RetryRequest != nil {
		t.Fatalf("detail target = %+v, want review-only target without retry", detailTarget)
	}
}

func TestBuildAssetGenerationOverviewIsDeterministicForMapBackedSummaries(t *testing.T) {
	t.Parallel()

	queue := &GenerationWorkQueue{
		Summary: &GenerationWorkQueueSummary{
			QualityGradeCounts: map[string]int{"ideal": 1, "source_backed": 1},
			PlatformQualityGradeCounts: map[string]map[string]int{
				"walmart": {"ideal": 1},
				"amazon":  {"provisional": 1},
				"shein":   {"missing": 1},
			},
			PlatformPreviewableCounts: map[string]int{
				"walmart": 1,
				"amazon":  1,
				"shein":   1,
			},
			PreviewCapabilityCounts: map[string]int{
				"subject_preview":     1,
				"detail_preview":      1,
				"measurement_preview": 1,
			},
		},
	}

	first := buildAssetGenerationOverview(queue)
	for i := 0; i < 20; i++ {
		next := buildAssetGenerationOverview(queue)
		if strings.Join(first.BlockingPlatforms, ",") != strings.Join(next.BlockingPlatforms, ",") {
			t.Fatalf("blocking platforms changed: first=%v next=%v", first.BlockingPlatforms, next.BlockingPlatforms)
		}
		if strings.Join(first.PreviewReadyPlatforms, ",") != strings.Join(next.PreviewReadyPlatforms, ",") {
			t.Fatalf("preview ready platforms changed: first=%v next=%v", first.PreviewReadyPlatforms, next.PreviewReadyPlatforms)
		}
		if strings.Join(first.PreviewReadyCapabilities, ",") != strings.Join(next.PreviewReadyCapabilities, ",") {
			t.Fatalf("preview ready capabilities changed: first=%v next=%v", first.PreviewReadyCapabilities, next.PreviewReadyCapabilities)
		}
	}
}

func containsIgnoreCase(input, fragment string) bool {
	return strings.Contains(strings.ToLower(input), strings.ToLower(fragment))
}
