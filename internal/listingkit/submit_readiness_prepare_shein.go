package listingkit

type sheinSubmitReadinessPreparation struct {
	readiness    *SheinSubmitReadiness
	stateChanged bool
}

func prepareSheinSubmitReadinessForAction(
	task *Task,
	pkg *SheinPackage,
	req *SubmitTaskRequest,
	action string,
	normalize func(*Task, *SheinPackage, *SubmitTaskRequest, string),
) sheinSubmitReadinessPreparation {
	stateChanged := false
	if normalize != nil {
		finalWasConfirmed := pkg != nil && pkg.FinalSubmissionDraft != nil && pkg.FinalSubmissionDraft.Confirmed
		normalize(task, pkg, req, action)
		finalNowConfirmed := pkg != nil && pkg.FinalSubmissionDraft != nil && pkg.FinalSubmissionDraft.Confirmed
		if finalNowConfirmed != finalWasConfirmed {
			stateChanged = true
		}
	}

	if ensureTaskPodExecution(task) {
		stateChanged = true
	}
	return sheinSubmitReadinessPreparation{
		readiness:    buildSheinSubmitReadinessWithPodForAction(pkg, taskPodExecution(task), action),
		stateChanged: stateChanged,
	}
}

func taskPodExecution(task *Task) *PodExecutionSummary {
	if task == nil || task.Result == nil {
		return nil
	}
	return task.Result.PodExecution
}
