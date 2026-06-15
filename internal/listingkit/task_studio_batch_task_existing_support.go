package listingkit

import (
	"context"
	"strings"
)

func (s *taskStudioBatchService) findExistingStudioBatchTask(
	ctx context.Context,
	recorded SheinStudioCreatedTaskList,
	design StudioMaterializedDesignRecord,
	grouped SheinStudioGroupedSelection,
	fallbackTitle string,
) (SheinStudioCreatedTask, bool) {
	if s == nil || s.getTask == nil || len(recorded) == 0 {
		return SheinStudioCreatedTask{}, false
	}
	designID := strings.TrimSpace(design.ID)
	for _, created := range recorded {
		if strings.TrimSpace(created.DesignID) != designID || strings.TrimSpace(created.ID) == "" {
			continue
		}
		task, err := s.getTask(ctx, created.ID)
		if err != nil || task == nil || task.Status == TaskStatusFailed {
			continue
		}
		if !studioBatchTaskMatchesSelection(task, design, grouped.Selection) {
			continue
		}
		if strings.TrimSpace(created.Title) == "" {
			created.Title = fallbackTitle
		}
		return created, true
	}
	return SheinStudioCreatedTask{}, false
}

func studioBatchTaskMatchesSelection(
	task *Task,
	design StudioMaterializedDesignRecord,
	selection SheinStudioSelection,
) bool {
	if task == nil || task.Request == nil || task.Request.Options == nil {
		return false
	}
	studio := task.Request.Options.SheinStudio
	sds := task.Request.Options.SDS
	if studio == nil || sds == nil {
		return false
	}
	if strings.TrimSpace(studio.StyleID) != buildStudioBatchTaskStyleID(design.ID) {
		return false
	}
	if len(task.Request.ImageURLs) == 0 || strings.TrimSpace(task.Request.ImageURLs[0]) != strings.TrimSpace(design.ImageURL) {
		return false
	}
	return sds.VariantID == selection.VariantID &&
		sds.ParentProductID == selection.ParentProductID &&
		sds.PrototypeGroupID == selection.PrototypeGroupID &&
		strings.TrimSpace(sds.LayerID) == strings.TrimSpace(selection.LayerID)
}

func mergeStudioCreatedTasks(
	existing SheinStudioCreatedTaskList,
	created []SheinStudioCreatedTask,
) SheinStudioCreatedTaskList {
	if len(existing) == 0 && len(created) == 0 {
		return nil
	}
	merged := make(SheinStudioCreatedTaskList, 0, len(existing)+len(created))
	seen := make(map[string]struct{}, len(existing)+len(created))
	appendIfMissing := func(task SheinStudioCreatedTask) {
		id := strings.TrimSpace(task.ID)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		merged = append(merged, task)
	}
	for _, task := range existing {
		appendIfMissing(task)
	}
	for _, task := range created {
		appendIfMissing(task)
	}
	return merged
}
