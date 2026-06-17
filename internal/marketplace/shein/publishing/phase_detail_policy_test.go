package publishing

import "testing"

func TestSubmissionPhaseDetailUsesSheinPublishingLabels(t *testing.T) {
	t.Parallel()

	if got := SubmissionPhaseDetail("publish", "submit_remote"); got != "提交 SHEIN 发布请求" {
		t.Fatalf("publish submit_remote detail = %q", got)
	}
	if got := SubmissionPhaseDetail("save_draft", "submit_remote"); got != "提交 SHEIN 草稿" {
		t.Fatalf("save_draft submit_remote detail = %q", got)
	}
	if got := SubmissionPhaseDetail("publish", "unknown_phase"); got != "unknown_phase" {
		t.Fatalf("unknown phase detail = %q", got)
	}
}
