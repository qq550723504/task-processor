package listingkit

import "strings"

func buildGenerationWorkQueueSummary(items []GenerationWorkQueueItem) *GenerationWorkQueueSummary {
	summary := &GenerationWorkQueueSummary{
		TotalItems:                      len(items),
		PlatformCounts:                  map[string]int{},
		PlatformPreviewableCounts:       map[string]int{},
		PreviewCapabilityCounts:         map[string]int{},
		PlatformPreviewCapabilityCounts: map[string]map[string]int{},
		StateCounts:                     map[string]int{},
		PlatformStateCounts:             map[string]map[string]int{},
		ExecutionQualityCounts:          map[string]int{},
		ExecutionQualityLabels:          map[string]string{},
		PlatformExecutionQualityCounts:  map[string]map[string]int{},
		QualityGradeCounts:              map[string]int{},
		QualityGradeLabels:              map[string]string{},
		PlatformQualityGradeCounts:      map[string]map[string]int{},
		GradeStateCounts:                map[string]map[string]int{},
		PlatformGradeStateCounts:        map[string]map[string]map[string]int{},
	}
	platforms := make([]string, 0, len(items))
	for _, item := range items {
		if platform := strings.TrimSpace(item.Platform); platform != "" {
			summary.PlatformCounts[platform]++
			if _, ok := summary.PlatformStateCounts[platform]; !ok {
				summary.PlatformStateCounts[platform] = map[string]int{}
			}
			if _, ok := summary.PlatformExecutionQualityCounts[platform]; !ok {
				summary.PlatformExecutionQualityCounts[platform] = map[string]int{}
			}
			if _, ok := summary.PlatformQualityGradeCounts[platform]; !ok {
				summary.PlatformQualityGradeCounts[platform] = map[string]int{}
			}
			if _, ok := summary.PlatformPreviewCapabilityCounts[platform]; !ok {
				summary.PlatformPreviewCapabilityCounts[platform] = map[string]int{}
			}
			if _, ok := summary.PlatformGradeStateCounts[platform]; !ok {
				summary.PlatformGradeStateCounts[platform] = map[string]map[string]int{}
			}
		}
		if item.RenderPreviewAvailable {
			summary.PreviewableItems++
			if platform := strings.TrimSpace(item.Platform); platform != "" {
				summary.PlatformPreviewableCounts[platform]++
			}
			for _, capability := range item.PreviewCapabilities {
				summary.PreviewCapabilityCounts[capability]++
				if platform := strings.TrimSpace(item.Platform); platform != "" {
					summary.PlatformPreviewCapabilityCounts[platform][capability]++
				}
			}
		}
		if state := strings.TrimSpace(item.State); state != "" {
			summary.StateCounts[state]++
			if platform := strings.TrimSpace(item.Platform); platform != "" {
				summary.PlatformStateCounts[platform][state]++
			}
		}
		if quality := strings.TrimSpace(item.ExecutionQuality); quality != "" {
			summary.ExecutionQualityCounts[quality]++
			if platform := strings.TrimSpace(item.Platform); platform != "" {
				summary.PlatformExecutionQualityCounts[platform][quality]++
			}
			if label := firstNonEmpty(strings.TrimSpace(item.ExecutionQualityLabel), generationExecutionQualityLabel(quality)); label != "" {
				summary.ExecutionQualityLabels[quality] = label
			}
		}
		if grade := strings.TrimSpace(item.QualityGrade); grade != "" {
			summary.QualityGradeCounts[grade]++
			if platform := strings.TrimSpace(item.Platform); platform != "" {
				summary.PlatformQualityGradeCounts[platform][grade]++
			}
			if label := firstNonEmpty(strings.TrimSpace(item.QualityGradeLabel), generationQualityGradeLabel(grade)); label != "" {
				summary.QualityGradeLabels[grade] = label
			}
			if _, ok := summary.GradeStateCounts[grade]; !ok {
				summary.GradeStateCounts[grade] = map[string]int{}
			}
			if state := strings.TrimSpace(item.State); state != "" {
				summary.GradeStateCounts[grade][state]++
				if platform := strings.TrimSpace(item.Platform); platform != "" {
					if _, ok := summary.PlatformGradeStateCounts[platform][grade]; !ok {
						summary.PlatformGradeStateCounts[platform][grade] = map[string]int{}
					}
					summary.PlatformGradeStateCounts[platform][grade][state]++
				}
			}
		}
		switch item.State {
		case "ready":
			summary.ReadyItems++
		case "fallback_in_use":
			summary.FallbackItems++
		case "missing":
			summary.MissingItems++
		case "queued":
			summary.QueuedItems++
		case "running":
			summary.RunningItems++
		case "completed":
			summary.CompletedItems++
		case "failed":
			summary.FailedItems++
		case "stubbed":
			summary.StubbedItems++
		}
		if item.Retryable {
			summary.RetryableItems++
		}
		switch strings.ToLower(strings.TrimSpace(item.ReviewStatus)) {
		case "approved":
			summary.ApprovedSections++
		case "deferred":
			summary.DeferredSections++
		case "pending":
			summary.ReviewPendingSections++
		default:
			if item.RenderPreviewAvailable {
				summary.ReviewPendingSections++
			}
		}
		platforms = append(platforms, item.Platform)
	}
	summary.Platforms = uniqueStrings(platforms)
	summary.DominantQualityGrade = dominantGenerationQualityGrade(summary)
	summary.DominantQualityGradeLabel = generationQualityGradeLabel(summary.DominantQualityGrade)
	return summary
}
