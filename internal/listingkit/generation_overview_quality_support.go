package listingkit

import (
	"strings"

	"task-processor/internal/listingkit/core"
)

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
	return core.SortedUniqueStrings(out)
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
	return core.SortedUniqueStrings(out)
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
	return core.SortedUniqueStrings(out)
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
