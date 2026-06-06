package generation

import assetgeneration "task-processor/internal/asset/generation"

type TaskSummaryStats struct {
	TotalTasks          int
	PlannedTasks        int
	CompletedTasks      int
	FailedTasks         int
	RendererBackedTasks int
	FallbackTasks       int
	RetryableTasks      int
	Platforms           []string
}

func SummarizeTasks(tasks []assetgeneration.Task) TaskSummaryStats {
	stats := TaskSummaryStats{}
	if len(tasks) == 0 {
		return stats
	}
	platforms := make([]string, 0, len(tasks))
	for _, item := range tasks {
		stats.TotalTasks++
		switch item.ExecutionStatus {
		case "completed":
			stats.CompletedTasks++
		case "failed":
			stats.FailedTasks++
		default:
			stats.PlannedTasks++
		}
		switch item.ExecutionMode {
		case assetgeneration.ExecutionModeRendererBacked:
			stats.RendererBackedTasks++
		case assetgeneration.ExecutionModeDeferredStub:
			stats.FallbackTasks++
		}
		if TaskRetryable(item) {
			stats.RetryableTasks++
		}
		platforms = append(platforms, item.Platform)
	}
	stats.Platforms = uniqueStrings(platforms)
	return stats
}
