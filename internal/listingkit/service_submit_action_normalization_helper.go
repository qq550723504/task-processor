package listingkit

import (
	listingsubmission "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func sheinPreferredSubmitAction(task *Task, settings SheinSettings) string {
	if task != nil && task.Result != nil {
		pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
		if pkg != nil && pkg.FinalSubmissionDraft != nil {
			if action := listingsubmission.PreferredSubmitAction(pkg.FinalSubmissionDraft.SubmitMode); action != "" {
				return action
			}
		}
	}
	return listingsubmission.PreferredSubmitAction(settings.DefaultSubmitMode)
}
