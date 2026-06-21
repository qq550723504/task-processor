package workspace

import "testing"

func TestBuildSubmitTemplateReadinessChecksUsesTemplateValidationInput(t *testing.T) {
	t.Parallel()

	checks := BuildSubmitTemplateReadinessChecks(SubmitTemplateReadinessInput{
		CategoryReady:        true,
		CategoryMessage:      "category ready",
		CategoryReviewReady:  false,
		AttributeReady:       false,
		AttributeMessage:     "attributes missing",
		SaleAttributeReady:   true,
		SaleAttributeMessage: "sale attributes ready",
	})

	assertTemplateReadinessCheck(t, checks, "category", true, "category ready")
	assertTemplateReadinessCheck(t, checks, "category_review", false, "当前类目仍被建议复核，提交前必须先确认 SHEIN 类目是否匹配")
	assertTemplateReadinessCheck(t, checks, "attributes", false, "attributes missing")
	assertTemplateReadinessCheck(t, checks, "attribute_review", false, "普通属性仍有模板必填项未确认，提交前必须补齐或人工确认")
	assertTemplateReadinessCheck(t, checks, "sale_attributes", true, "sale attributes ready")
}

func TestBuildSubmitTemplateReadinessChecksKeepsFieldPathsAndActions(t *testing.T) {
	t.Parallel()

	checks := BuildSubmitTemplateReadinessChecks(SubmitTemplateReadinessInput{})

	check := findTemplateReadinessCheck(t, checks, "category")
	if check.SuggestedAction != "确认类目" {
		t.Fatalf("category suggested action = %q, want %q", check.SuggestedAction, "确认类目")
	}
	assertContainsFieldPath(t, check.FieldPaths, "shein.product_type_id")

	check = findTemplateReadinessCheck(t, checks, "sale_attributes")
	if check.SuggestedAction != "确认规格" {
		t.Fatalf("sale_attributes suggested action = %q, want %q", check.SuggestedAction, "确认规格")
	}
	assertContainsFieldPath(t, check.FieldPaths, "shein.request_draft.skc_list")
}

func assertTemplateReadinessCheck(t *testing.T, checks []ReadinessCheckSpec, key string, ok bool, message string) {
	t.Helper()
	check := findTemplateReadinessCheck(t, checks, key)
	if check.OK != ok {
		t.Fatalf("check %q OK = %v, want %v; check=%+v", key, check.OK, ok, check)
	}
	if check.Message != message {
		t.Fatalf("check %q message = %q, want %q", key, check.Message, message)
	}
}

func findTemplateReadinessCheck(t *testing.T, checks []ReadinessCheckSpec, key string) ReadinessCheckSpec {
	t.Helper()
	for _, check := range checks {
		if check.Key == key {
			return check
		}
	}
	t.Fatalf("missing readiness check %q in %+v", key, checks)
	return ReadinessCheckSpec{}
}

func assertContainsFieldPath(t *testing.T, fieldPaths []string, want string) {
	t.Helper()
	for _, fieldPath := range fieldPaths {
		if fieldPath == want {
			return
		}
	}
	t.Fatalf("field paths %v should contain %q", fieldPaths, want)
}
