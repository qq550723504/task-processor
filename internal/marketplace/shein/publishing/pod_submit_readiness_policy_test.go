package publishing

import (
	"testing"

	sdspod "task-processor/internal/product/sourcing/sdspod"
)

func TestEvaluatePODSubmitReadiness(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		action    string
		execution sdspod.Execution
		want      PODSubmitReadiness
	}{
		"disabled pod is omitted": {
			execution: sdspod.Execution{DependencyMode: sdspod.DependencyDisabled},
			want:      PODSubmitReadiness{},
		},
		"required failed pod blocks publish": {
			action: "publish",
			execution: sdspod.Execution{
				Provider:       sdspod.ProviderSDS,
				DependencyMode: sdspod.DependencyRequired,
				Status:         sdspod.StatusFailedBlocking,
				FailureReason:  "design template unavailable",
			},
			want: PODSubmitReadiness{Applicable: true, Ready: false, WarningOnly: false},
		},
		"optional degraded pod warns for publish": {
			action: "publish",
			execution: sdspod.Execution{
				Provider:       sdspod.ProviderSDS,
				DependencyMode: sdspod.DependencyOptional,
				Status:         sdspod.StatusFailedDegraded,
				FailureReason:  "size image render unavailable",
			},
			want: PODSubmitReadiness{Applicable: true, Ready: false, WarningOnly: true},
		},
		"unfinished pod warns for save draft": {
			action: "save_draft",
			execution: sdspod.Execution{
				Provider:       sdspod.ProviderSDS,
				DependencyMode: sdspod.DependencyRequired,
				Status:         sdspod.StatusPending,
			},
			want: PODSubmitReadiness{Applicable: true, Ready: false, WarningOnly: true},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := EvaluatePODSubmitReadiness(test.action, test.execution)
			if got.Applicable != test.want.Applicable || got.Ready != test.want.Ready || got.WarningOnly != test.want.WarningOnly {
				t.Fatalf("EvaluatePODSubmitReadiness(%q, %+v) = %+v, want %+v", test.action, test.execution, got, test.want)
			}
			if got.Applicable && got.Message == "" {
				t.Fatal("applicable readiness decision should include a message")
			}
		})
	}
}
