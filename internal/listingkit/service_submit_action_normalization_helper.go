package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

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
