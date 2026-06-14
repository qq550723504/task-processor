package submission

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestSheinSubmitPhaseDetail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action string
		phase  string
		want   string
	}{
		{name: "validate", action: "publish", phase: sheinpub.SubmissionPhaseValidate, want: "检查 SHEIN 提交前状态"},
		{name: "publish submit remote", action: "publish", phase: sheinpub.SubmissionPhaseSubmitRemote, want: "提交 SHEIN 发布请求"},
		{name: "save draft submit remote", action: "save_draft", phase: sheinpub.SubmissionPhaseSubmitRemote, want: "提交 SHEIN 草稿"},
		{name: "unknown phase", action: "publish", phase: "custom_phase", want: "custom_phase"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SheinSubmitPhaseDetail(tt.action, tt.phase); got != tt.want {
				t.Fatalf("SheinSubmitPhaseDetail(%q, %q) = %q, want %q", tt.action, tt.phase, got, tt.want)
			}
		})
	}
}
