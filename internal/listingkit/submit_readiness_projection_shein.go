package listingkit

import sheinworkspace "task-processor/internal/listingkit/workspace/shein"

type sheinSubmitReadinessProjection struct {
	Readiness      *SheinSubmitReadiness
	Checklist      *SheinSubmitChecklist
	SubmitState    *sheinworkspace.SubmitStateInput
	StatusOverview *sheinworkspace.StatusOverview
}

func buildSheinSubmitReadinessProjection(pkg *SheinPackage) *sheinSubmitReadinessProjection {
	return buildSheinSubmitReadinessProjectionWithPod(pkg, nil)
}

func buildSheinSubmitReadinessProjectionWithPod(pkg *SheinPackage, pod *PodExecutionSummary) *sheinSubmitReadinessProjection {
	if pkg == nil {
		return nil
	}
	readiness := buildSheinSubmitReadinessWithPod(pkg, pod)
	submitState := sheinworkspace.BuildSubmitStateInput(readiness)
	return &sheinSubmitReadinessProjection{
		Readiness:      readiness,
		Checklist:      buildSheinSubmitChecklist(readiness),
		SubmitState:    submitState,
		StatusOverview: sheinworkspace.BuildStatusOverview(pkg.Inspection, submitState),
	}
}
