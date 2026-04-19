package listingkit

import "strings"

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

func cloneStringIntMap(input map[string]int) map[string]int {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]int, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func previewReadyPlatformsForQueue(summary *GenerationWorkQueueSummary) []string {
	if summary == nil || len(summary.PlatformPreviewableCounts) == 0 {
		return nil
	}
	out := make([]string, 0, len(summary.PlatformPreviewableCounts))
	for platform, count := range summary.PlatformPreviewableCounts {
		if count > 0 {
			out = append(out, platform)
		}
	}
	return sortedUniqueStrings(out)
}

func previewReadyCapabilitiesForQueue(summary *GenerationWorkQueueSummary) []string {
	if summary == nil || len(summary.PreviewCapabilityCounts) == 0 {
		return nil
	}
	out := make([]string, 0, len(summary.PreviewCapabilityCounts))
	for capability, count := range summary.PreviewCapabilityCounts {
		if count > 0 {
			out = append(out, capability)
		}
	}
	return sortedUniqueStrings(out)
}

func reviewActionReason(summary *GenerationWorkQueueSummary) string {
	if summary != nil && summary.PreviewableItems > 0 {
		return "Current asset coverage is sufficient and preview sidecars are available for review."
	}
	return "Current asset coverage is sufficient to continue publish review."
}

func dominantGenerationQualityGrade(summary *GenerationWorkQueueSummary) string {
	if summary == nil {
		return ""
	}
	order := []string{"missing", "provisional", "source_backed", "ideal"}
	best := ""
	bestCount := 0
	for _, grade := range order {
		if count := qualityGradeCount(summary, grade); count > bestCount {
			best = grade
			bestCount = count
		}
	}
	return best
}

func qualityGradeCount(summary *GenerationWorkQueueSummary, grade string) int {
	if summary == nil || summary.QualityGradeCounts == nil {
		return 0
	}
	return summary.QualityGradeCounts[strings.ToLower(strings.TrimSpace(grade))]
}

func blockingPlatformsForQueue(queue *GenerationWorkQueue) []string {
	if queue == nil || queue.Summary == nil {
		return nil
	}
	out := make([]string, 0, len(queue.Summary.PlatformQualityGradeCounts))
	for platform, counts := range queue.Summary.PlatformQualityGradeCounts {
		if counts["missing"] > 0 || counts["provisional"] > 0 {
			out = append(out, platform)
		}
	}
	return sortedUniqueStrings(out)
}

func blockingQualityGradesForQueue(summary *GenerationWorkQueueSummary) []string {
	if summary == nil {
		return nil
	}
	out := make([]string, 0, 2)
	if qualityGradeCount(summary, "missing") > 0 {
		out = append(out, "missing")
	}
	if qualityGradeCount(summary, "provisional") > 0 {
		out = append(out, "provisional")
	}
	return uniqueStrings(out)
}

func reviewActionKey(summary *GenerationWorkQueueSummary) string {
	if qualityGradeCount(summary, "ideal") > 0 && qualityGradeCount(summary, "source_backed") == 0 {
		return "continue_publish_review"
	}
	return "review_ready_assets"
}

func buildAssetGenerationActionTarget(queue *GenerationWorkQueue, actionKey string, filters *AssetGenerationRecommendedFilters) *AssetGenerationActionTarget {
	actionKey = strings.TrimSpace(actionKey)
	if actionKey == "" {
		return nil
	}
	target := &AssetGenerationActionTarget{
		ActionKey:       actionKey,
		InteractionMode: actionInteractionMode(actionKey),
	}
	cloned := cloneAssetGenerationFilters(actionFiltersForKey(actionKey, filters))
	if cloned == nil {
		return target
	}
	target.Filters = cloned
	target.QueueQuery = &GenerationQueueQuery{
		QualityGrade:                  cloned.QualityGrade,
		QualityGradeLabel:             cloned.QualityGradeLabel,
		ExecutionQuality:              cloned.ExecutionQuality,
		PreviewCapability:             cloned.PreviewCapability,
		RenderPreviewAvailable:        cloned.RenderPreviewAvailable,
		RenderPreviewAvailablePresent: cloned.RenderPreviewAvailable,
		Retryable:                     cloned.RetryableOnly,
		RetryablePresent:              cloned.RetryableOnly,
		SortBy:                        "quality_grade",
		SortOrder:                     "asc",
	}
	if len(cloned.Platforms) == 1 {
		target.QueueQuery.Platform = cloned.Platforms[0]
	}
	target.ExpectedImpact = buildAssetGenerationActionImpact(queue, target.QueueQuery)
	target.RetryRequest = &RetryGenerationTasksRequest{
		QualityGrade:      cloned.QualityGrade,
		QualityGradeLabel: cloned.QualityGradeLabel,
		ExecutionQuality:  cloned.ExecutionQuality,
	}
	switch target.InteractionMode {
	case "queue_only", "review_only":
		target.RetryRequest = nil
	}
	target.NavigationTarget = buildGenerationReviewActionNavigationTarget(target)
	return target
}

func buildSecondaryActionTargets(queue *GenerationWorkQueue, actionKeys []string, filters *AssetGenerationRecommendedFilters) []*AssetGenerationActionTarget {
	if len(actionKeys) == 0 {
		return nil
	}
	out := make([]*AssetGenerationActionTarget, 0, len(actionKeys))
	for _, actionKey := range uniqueStrings(actionKeys) {
		target := buildAssetGenerationActionTarget(queue, actionKey, filters)
		if target == nil {
			continue
		}
		out = append(out, target)
	}
	return out
}

func cloneAssetGenerationFilters(filters *AssetGenerationRecommendedFilters) *AssetGenerationRecommendedFilters {
	if filters == nil {
		return nil
	}
	cloned := *filters
	cloned.Platforms = append([]string(nil), filters.Platforms...)
	return &cloned
}

func actionFiltersForKey(actionKey string, base *AssetGenerationRecommendedFilters) *AssetGenerationRecommendedFilters {
	filters := cloneAssetGenerationFilters(base)
	if filters == nil {
		filters = &AssetGenerationRecommendedFilters{}
	}
	if spec := previewCapabilityActionSpecForKey(actionKey); spec != nil {
		filters.ExecutionQuality = ""
		filters.RetryableOnly = false
		filters.RenderPreviewAvailable = true
		filters.PreviewCapability = spec.Capability
		if strings.TrimSpace(filters.QualityGrade) == "" {
			filters.QualityGrade = "ideal"
			filters.QualityGradeLabel = generationQualityGradeLabel("ideal")
		}
		return filters
	}
	switch actionKey {
	case "generate_missing_assets", "review_missing_slots":
		filters.QualityGrade = "missing"
		filters.QualityGradeLabel = generationQualityGradeLabel("missing")
		if actionKey == "generate_missing_assets" {
			filters.RetryableOnly = true
		}
		filters.ExecutionQuality = ""
	case "retry_failed_generation", "inspect_failed_renderer_tasks":
		filters.QualityGrade = "provisional"
		filters.QualityGradeLabel = generationQualityGradeLabel("provisional")
		filters.ExecutionQuality = "failed"
		filters.RetryableOnly = true
	case "upgrade_fallback_assets", "retry_provisional_slots":
		filters.QualityGrade = "provisional"
		filters.QualityGradeLabel = generationQualityGradeLabel("provisional")
		filters.ExecutionQuality = ""
		filters.RetryableOnly = true
	case "review_ready_assets", "continue_publish_review":
		if strings.TrimSpace(filters.QualityGrade) == "" {
			filters.QualityGrade = "ideal"
			filters.QualityGradeLabel = generationQualityGradeLabel("ideal")
		}
		filters.ExecutionQuality = ""
		filters.RetryableOnly = false
	case "retry_section_generation":
		filters.QualityGrade = "provisional"
		filters.QualityGradeLabel = generationQualityGradeLabel("provisional")
		filters.RetryableOnly = true
	case "defer_section_review", "approve_section_review":
		if strings.TrimSpace(filters.QualityGrade) == "" {
			filters.QualityGrade = "ideal"
			filters.QualityGradeLabel = generationQualityGradeLabel("ideal")
		}
		filters.ExecutionQuality = ""
		filters.RetryableOnly = false
	}
	return filters
}

func actionInteractionMode(actionKey string) string {
	if previewCapabilityActionSpecForKey(actionKey) != nil {
		return "review_only"
	}
	switch actionKey {
	case "generate_missing_assets", "retry_failed_generation", "upgrade_fallback_assets", "retry_provisional_slots":
		return "retryable"
	case "retry_section_generation":
		return "retryable"
	case "review_missing_slots", "inspect_failed_renderer_tasks":
		return "queue_only"
	case "review_ready_assets", "continue_publish_review", "defer_section_review", "approve_section_review":
		return "review_only"
	default:
		return "queue_only"
	}
}
