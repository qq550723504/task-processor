package workspace

import "testing"

func TestBuildSubmitPayloadValidationReadinessChecksSkipsReadyPayload(t *testing.T) {
	t.Parallel()

	checks := BuildSubmitPayloadValidationReadinessChecks(SubmitPayloadValidationReadinessInput{
		Ready:   true,
		Message: "ready",
	})

	if len(checks) != 0 {
		t.Fatalf("checks length = %d, want 0; checks=%+v", len(checks), checks)
	}
}

func TestBuildSubmitPayloadValidationReadinessChecksReportsPreparedPayloadFailure(t *testing.T) {
	t.Parallel()

	checks := BuildSubmitPayloadValidationReadinessChecks(SubmitPayloadValidationReadinessInput{
		Message: "missing stock_info_list",
	})

	check := findTemplateReadinessCheck(t, checks, "variants")
	if check.OK {
		t.Fatalf("variants check OK = true, want false; check=%+v", check)
	}
	if check.Label != "发布载荷结构" {
		t.Fatalf("variants label = %q, want %q", check.Label, "发布载荷结构")
	}
	if check.Message != "missing stock_info_list" {
		t.Fatalf("variants message = %q, want %q", check.Message, "missing stock_info_list")
	}
	assertContainsFieldPath(t, check.FieldPaths, "shein.preview_product")
	assertContainsFieldPath(t, check.FieldPaths, "shein.request_draft.skc_list")
}
