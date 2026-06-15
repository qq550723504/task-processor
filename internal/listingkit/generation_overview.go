package listingkit

type AssetGenerationOverview struct {
	PrimaryAction             string                             `json:"primary_action,omitempty"`
	PrimaryActionKey          string                             `json:"primary_action_key,omitempty"`
	PrimaryActionTarget       *AssetGenerationActionTarget       `json:"primary_action_target,omitempty"`
	PrimaryCTAKind            string                             `json:"primary_cta_kind,omitempty"`
	PrimaryNavigationTarget   *GenerationReviewNavigationTarget  `json:"primary_navigation_target,omitempty"`
	PrimaryActionReason       string                             `json:"primary_action_reason,omitempty"`
	ResolvedActionSummary     *GenerationResolvedActionSummary   `json:"resolved_action_summary,omitempty"`
	SecondaryActions          []string                           `json:"secondary_actions,omitempty"`
	SecondaryActionKeys       []string                           `json:"secondary_action_keys,omitempty"`
	SecondaryActionTargets    []*AssetGenerationActionTarget     `json:"secondary_action_targets,omitempty"`
	DominantQualityGrade      string                             `json:"dominant_quality_grade,omitempty"`
	DominantQualityGradeLabel string                             `json:"dominant_quality_grade_label,omitempty"`
	BlockingPlatforms         []string                           `json:"blocking_platforms,omitempty"`
	BlockingQualityGrades     []string                           `json:"blocking_quality_grades,omitempty"`
	PreviewableItems          int                                `json:"previewable_items"`
	PreviewReadyPlatforms     []string                           `json:"preview_ready_platforms,omitempty"`
	PreviewCapabilityCounts   map[string]int                     `json:"preview_capability_counts,omitempty"`
	PreviewReadyCapabilities  []string                           `json:"preview_ready_capabilities,omitempty"`
	RecommendedFilters        *AssetGenerationRecommendedFilters `json:"recommended_filters,omitempty"`
	RetryableCount            int                                `json:"retryable_count"`
	ApprovedSections          int                                `json:"approved_sections"`
	DeferredSections          int                                `json:"deferred_sections"`
	ReviewPendingSections     int                                `json:"review_pending_sections"`
	RecoverySummary           *GenerationRecoverySummary         `json:"recovery_summary,omitempty"`
}

func buildAssetGenerationOverview(queue *GenerationWorkQueue) *AssetGenerationOverview {
	if queue == nil || queue.Summary == nil {
		return nil
	}
	summary := queue.Summary
	grade := dominantGenerationQualityGrade(summary)
	overview := &AssetGenerationOverview{
		DominantQualityGrade:      grade,
		DominantQualityGradeLabel: generationQualityGradeLabel(grade),
		BlockingPlatforms:         blockingPlatformsForQueue(queue),
		BlockingQualityGrades:     blockingQualityGradesForQueue(summary),
		PreviewableItems:          summary.PreviewableItems,
		PreviewReadyPlatforms:     previewReadyPlatformsForQueue(summary),
		PreviewCapabilityCounts:   cloneStringIntMap(summary.PreviewCapabilityCounts),
		PreviewReadyCapabilities:  previewReadyCapabilitiesForQueue(summary),
		RetryableCount:            summary.RetryableItems,
		ApprovedSections:          summary.ApprovedSections,
		DeferredSections:          summary.DeferredSections,
		ReviewPendingSections:     summary.ReviewPendingSections,
	}
	switch {
	case summary.MissingItems > 0:
		overview.PrimaryAction = "Generate Missing Assets"
		overview.PrimaryActionKey = "generate_missing_assets"
		overview.PrimaryActionReason = "Some required image slots still have no selected or generated asset."
		overview.SecondaryActions = []string{"Review missing slots by platform"}
		overview.SecondaryActionKeys = []string{"review_missing_slots"}
		overview.RecommendedFilters = &AssetGenerationRecommendedFilters{
			QualityGrade:      "missing",
			QualityGradeLabel: generationQualityGradeLabel("missing"),
			Platforms:         append([]string(nil), overview.BlockingPlatforms...),
			RetryableOnly:     true,
		}
	case summary.FailedItems > 0 && summary.FailedItems >= qualityGradeCount(summary, "provisional"):
		overview.PrimaryAction = "Retry Failed Generation"
		overview.PrimaryActionKey = "retry_failed_generation"
		overview.PrimaryActionReason = "Some generation tasks failed and should be retried before publish review."
		overview.SecondaryActions = []string{"Inspect failed renderer tasks"}
		overview.SecondaryActionKeys = []string{"inspect_failed_renderer_tasks"}
		overview.RecommendedFilters = &AssetGenerationRecommendedFilters{
			QualityGrade:      "provisional",
			QualityGradeLabel: generationQualityGradeLabel("provisional"),
			Platforms:         append([]string(nil), overview.BlockingPlatforms...),
			RetryableOnly:     true,
			ExecutionQuality:  "failed",
		}
	case qualityGradeCount(summary, "provisional") > 0:
		overview.PrimaryAction = "Upgrade Fallback Assets"
		overview.PrimaryActionKey = "upgrade_fallback_assets"
		overview.PrimaryActionReason = "Fallback or stubbed assets are still covering publish-critical slots."
		overview.SecondaryActions = []string{"Retry provisional slots"}
		overview.SecondaryActionKeys = []string{"retry_provisional_slots"}
		overview.RecommendedFilters = &AssetGenerationRecommendedFilters{
			QualityGrade:      "provisional",
			QualityGradeLabel: generationQualityGradeLabel("provisional"),
			Platforms:         append([]string(nil), overview.BlockingPlatforms...),
			RetryableOnly:     true,
		}
	case qualityGradeCount(summary, "ideal")+qualityGradeCount(summary, "source_backed") > 0:
		overview.PrimaryAction = "Review Ready Assets"
		overview.PrimaryActionKey = reviewActionKey(summary)
		overview.PrimaryActionReason = reviewActionReason(summary)
		overview.SecondaryActions, overview.SecondaryActionKeys = buildPreviewCapabilitySecondaryActions(summary)
		if len(overview.SecondaryActionKeys) == 0 && overview.PrimaryActionKey != assetGenerationActionContinuePublishReview {
			overview.SecondaryActions = []string{"Continue publish review"}
			overview.SecondaryActionKeys = []string{assetGenerationActionContinuePublishReview}
		}
		overview.RecommendedFilters = &AssetGenerationRecommendedFilters{
			QualityGrade:           grade,
			QualityGradeLabel:      generationQualityGradeLabel(grade),
			Platforms:              append([]string(nil), overview.BlockingPlatforms...),
			RetryableOnly:          false,
			RenderPreviewAvailable: summary.PreviewableItems > 0,
		}
	default:
		overview.PrimaryAction = "Review Asset Queue"
		overview.PrimaryActionKey = "review_ready_assets"
		overview.PrimaryActionReason = "Inspect the current generation queue before the next publish step."
	}
	overview.SecondaryActions = uniqueStrings(overview.SecondaryActions)
	overview.SecondaryActionKeys = uniqueStrings(overview.SecondaryActionKeys)
	overview.PrimaryActionTarget = buildAssetGenerationActionTarget(queue, overview.PrimaryActionKey, overview.RecommendedFilters)
	overview.SecondaryActionTargets = buildSecondaryActionTargets(queue, overview.SecondaryActionKeys, overview.RecommendedFilters)
	overview.RecoverySummary = buildGenerationRecoverySummaryFromQueue(queue)
	return finalizeGenerationOverviewActionSummary(applyGenerationRecoveryArbitrationToOverview(overview))
}

func reviewActionKey(summary *GenerationWorkQueueSummary) string {
	if qualityGradeCount(summary, "ideal") > 0 && qualityGradeCount(summary, "source_backed") == 0 {
		return "continue_publish_review"
	}
	return "review_ready_assets"
}

func cloneAssetGenerationFilters(filters *AssetGenerationRecommendedFilters) *AssetGenerationRecommendedFilters {
	if filters == nil {
		return nil
	}
	cloned := *filters
	applyAssetGenerationFiltersPlatformsClone(filters, &cloned)
	return &cloned
}

func applyAssetGenerationFiltersPlatformsClone(filters *AssetGenerationRecommendedFilters, cloned *AssetGenerationRecommendedFilters) {
	if filters == nil || cloned == nil {
		return
	}
	cloned.Platforms = append([]string(nil), filters.Platforms...)
}

func actionFiltersForKey(actionKey string, base *AssetGenerationRecommendedFilters) *AssetGenerationRecommendedFilters {
	filters := cloneAssetGenerationFilters(base)
	if filters == nil {
		filters = &AssetGenerationRecommendedFilters{}
	}
	applyAssetGenerationActionFiltersMutation(actionKey, filters)
	return filters
}

func applyAssetGenerationActionFiltersMutation(actionKey string, filters *AssetGenerationRecommendedFilters) {
	if filters == nil {
		return
	}
	if applyAssetGenerationPreviewCapabilityFilterMutation(actionKey, filters) {
		return
	}
	applyAssetGenerationRegularActionKeyFilterMutation(actionKey, filters)
}

func applyAssetGenerationRegularActionKeyFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) {
	switch {
	case applyAssetGenerationFailedRetryFilterMutation(actionKey, filters):
		return
	case applyAssetGenerationProvisionalRetryFilterMutation(actionKey, filters):
		return
	case applyAssetGenerationReviewReadyFilterMutation(actionKey, filters):
		return
	}
	switch actionKey {
	case "generate_missing_assets", "review_missing_slots":
		filters.QualityGrade = "missing"
		filters.QualityGradeLabel = generationQualityGradeLabel("missing")
		if actionKey == "generate_missing_assets" {
			filters.RetryableOnly = true
		}
		filters.ExecutionQuality = ""
	}
}
