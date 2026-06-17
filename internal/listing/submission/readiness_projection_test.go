package submission

import "testing"

func TestBuildReadinessProjection(t *testing.T) {
	t.Parallel()

	projection := BuildReadinessProjection(ReadinessProjectionInput[string, []string, string, string]{
		Readiness: "ready",
		BuildChecklist: func(readiness string) []string {
			return []string{readiness + "-check"}
		},
		BuildSubmitState: func(readiness string) string {
			return readiness + "-state"
		},
		BuildStatusOverview: func(state string) string {
			return state + "-overview"
		},
	})

	if projection.Readiness != "ready" {
		t.Fatalf("Readiness = %q, want ready", projection.Readiness)
	}
	if len(projection.Checklist) != 1 || projection.Checklist[0] != "ready-check" {
		t.Fatalf("Checklist = %#v, want ready-check", projection.Checklist)
	}
	if projection.SubmitState != "ready-state" {
		t.Fatalf("SubmitState = %q, want ready-state", projection.SubmitState)
	}
	if projection.StatusOverview != "ready-state-overview" {
		t.Fatalf("StatusOverview = %q, want ready-state-overview", projection.StatusOverview)
	}
}
