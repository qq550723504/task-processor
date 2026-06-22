package listingkit

import (
	listingsubmission "task-processor/internal/listing/submission"
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
)

type sheinSubmitReadinessProjection struct {
	listingsubmission.ReadinessProjection[*SheinSubmitReadiness, *SheinSubmitChecklist, *sheinworkspace.SubmitStateInput, *sheinworkspace.StatusOverview]
}

func buildSheinSubmitReadinessProjection(pkg *SheinPackage) *sheinSubmitReadinessProjection {
	return buildSheinSubmitReadinessProjectionWithPod(pkg, nil)
}

func buildSheinSubmitReadinessProjectionWithPod(pkg *SheinPackage, pod *PodExecutionSummary) *sheinSubmitReadinessProjection {
	if pkg == nil {
		return nil
	}
	readiness := buildSheinSubmitReadinessWithPod(pkg, pod)
	projection := listingsubmission.BuildReadinessProjection(
		listingsubmission.ReadinessProjectionInput[*SheinSubmitReadiness, *SheinSubmitChecklist, *sheinworkspace.SubmitStateInput, *sheinworkspace.StatusOverview]{
			Readiness: readiness,
			BuildChecklist: func(readiness *SheinSubmitReadiness) *SheinSubmitChecklist {
				return sheinworkspace.BuildSubmitChecklist(readiness, sheinworkspace.SubmitChecklistGroupForKey)
			},
			BuildSubmitState: func(readiness *SheinSubmitReadiness) *sheinworkspace.SubmitStateInput {
				return sheinworkspace.BuildSubmitStateInput(readiness)
			},
			BuildStatusOverview: func(submitState *sheinworkspace.SubmitStateInput) *sheinworkspace.StatusOverview {
				return sheinworkspace.BuildStatusOverview(pkg.Inspection, submitState)
			},
		},
	)
	return &sheinSubmitReadinessProjection{ReadinessProjection: projection}
}
