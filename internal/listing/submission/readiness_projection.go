package submission

// ReadinessProjection carries a generic readiness-derived projection bundle.
type ReadinessProjection[R, C, S, O any] struct {
	Readiness      R
	Checklist      C
	SubmitState    S
	StatusOverview O
}

// ReadinessProjectionInput provides platform-specific builders for a readiness projection.
type ReadinessProjectionInput[R, C, S, O any] struct {
	Readiness           R
	BuildChecklist      func(R) C
	BuildSubmitState    func(R) S
	BuildStatusOverview func(S) O
}

// BuildReadinessProjection assembles readiness-derived projection values in dependency order.
func BuildReadinessProjection[R, C, S, O any](input ReadinessProjectionInput[R, C, S, O]) ReadinessProjection[R, C, S, O] {
	submitState := input.BuildSubmitState(input.Readiness)
	return ReadinessProjection[R, C, S, O]{
		Readiness:      input.Readiness,
		Checklist:      input.BuildChecklist(input.Readiness),
		SubmitState:    submitState,
		StatusOverview: input.BuildStatusOverview(submitState),
	}
}
