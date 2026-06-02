package listingkit

import (
	"context"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) resolveDefaultSheinSubmitAction(ctx context.Context, taskID string) (string, error) {
	if s == nil || s.repo == nil {
		return "publish", nil
	}
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return "", err
	}
	if task == nil {
		return "publish", nil
	}
	if action := sheinPreferredSubmitAction(task, buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)); action != "" {
		return action, nil
	}
	return "publish", nil
}

func sheinPreferredSubmitAction(task *Task, settings SheinSettings) string {
	if task != nil && task.Result != nil {
		pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
		if pkg != nil && pkg.FinalSubmissionDraft != nil {
			if action := normalizePreferredSheinSubmitAction(pkg.FinalSubmissionDraft.SubmitMode); action != "" {
				return action
			}
		}
	}
	return normalizePreferredSheinSubmitAction(settings.DefaultSubmitMode)
}

func normalizePreferredSheinSubmitAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "publish":
		return "publish"
	case "save_draft":
		return "save_draft"
	default:
		return ""
	}
}
