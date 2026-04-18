package repository

import assetgeneration "task-processor/internal/asset/generation"

func normalizeGenerationTasks(taskID string, tasks []assetgeneration.Task) []assetgeneration.Task {
	if len(tasks) == 0 {
		return nil
	}
	out := make([]assetgeneration.Task, 0, len(tasks))
	for _, item := range tasks {
		item.TaskID = taskID
		out = append(out, item)
	}
	return out
}
