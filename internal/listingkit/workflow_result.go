package listingkit

import "strings"

func initResult(task *Task) *ListingKitResult {
	if task == nil || task.Request == nil {
		return &ListingKitResult{Summary: &GenerationSummary{}}
	}
	return &ListingKitResult{
		TaskID:    task.ID,
		Status:    string(TaskStatusProcessing),
		Platforms: append([]string(nil), task.Request.Platforms...),
		Country:   task.Request.Country,
		Language:  task.Request.Language,
		CreatedAt: task.CreatedAt,
		UpdatedAt: task.UpdatedAt,
		Summary: &GenerationSummary{
			SourceType: detectSourceType(task.Request),
			ImageCount: len(task.Request.ImageURLs),
		},
	}
}

func markChildTask(result *ListingKitResult, kind, taskID, status, errorMsg string) {
	if result == nil {
		return
	}
	// Compatibility shim for legacy task payloads and older UI paths.
	// New workflow state should be recorded through workflowRecorder stages/issues.
	for i := range result.ChildTasks {
		if result.ChildTasks[i].Kind == kind {
			result.ChildTasks[i].TaskID = taskID
			result.ChildTasks[i].Status = status
			result.ChildTasks[i].Error = errorMsg
			return
		}
	}
	result.ChildTasks = append(result.ChildTasks, ChildTaskState{Kind: kind, TaskID: taskID, Status: status, Error: errorMsg})
}

func appendWarning(result *ListingKitResult, warning string) {
	if result == nil || result.Summary == nil || strings.TrimSpace(warning) == "" {
		return
	}
	// Compatibility shim for legacy summary warnings. New warnings should be
	// represented as workflow issues and surfaced through aggregate counts.
	result.Summary.Warnings = append(result.Summary.Warnings, warning)
}

func detectSourceType(req *GenerateRequest) string {
	if req == nil {
		return ""
	}
	switch {
	case strings.TrimSpace(req.ProductURL) != "" && len(req.ImageURLs) > 0 && strings.TrimSpace(req.Text) != "":
		return "mixed"
	case strings.TrimSpace(req.ProductURL) != "":
		return "product_url"
	case len(req.ImageURLs) > 0 && strings.TrimSpace(req.Text) != "":
		return "images_and_text"
	case len(req.ImageURLs) > 0:
		return "images"
	case strings.TrimSpace(req.Text) != "":
		return "text"
	default:
		return "unknown"
	}
}
