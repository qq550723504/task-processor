package workspace

import (
	"errors"
	"testing"
)

func TestBuildFreshnessAuthFailureCheck(t *testing.T) {
	t.Parallel()

	got := BuildFreshnessAuthFailureCheck(errors.New(" cookie expired "))

	if got.Key != FreshnessAuthKey {
		t.Fatalf("Key = %q, want %q", got.Key, FreshnessAuthKey)
	}
	if got.OK {
		t.Fatal("OK = true, want false")
	}
	if got.Message != "SHEIN 提交店铺当前不可用，请先刷新登录态后再提交：cookie expired" {
		t.Fatalf("Message = %q, want trimmed auth failure message", got.Message)
	}
	if len(got.FieldPaths) != 2 || got.FieldPaths[0] != "shein.store_resolution" || got.FieldPaths[1] != "shein.review_notes" {
		t.Fatalf("FieldPaths = %#v, want auth-related fields", got.FieldPaths)
	}
}

func TestBuildFreshnessTemplateChecks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		got             ReadinessCheckSpec
		key             string
		label           string
		suggestedAction string
	}{
		{
			name:            "category",
			got:             BuildFreshnessCategoryCheck(false, "category stale"),
			key:             FreshnessCategoryKey,
			label:           "类目模板新鲜度",
			suggestedAction: "刷新类目模板",
		},
		{
			name:            "attribute",
			got:             BuildFreshnessAttributeCheck(false, "attribute stale"),
			key:             FreshnessAttributeKey,
			label:           "普通属性模板新鲜度",
			suggestedAction: "刷新属性模板",
		},
		{
			name:            "sale attribute",
			got:             BuildFreshnessSaleAttributeCheck(false, "sale stale"),
			key:             FreshnessSaleAttributeKey,
			label:           "销售属性模板新鲜度",
			suggestedAction: "刷新销售属性",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.got.Key != tc.key {
				t.Fatalf("Key = %q, want %q", tc.got.Key, tc.key)
			}
			if tc.got.Label != tc.label {
				t.Fatalf("Label = %q, want %q", tc.got.Label, tc.label)
			}
			if tc.got.OK {
				t.Fatal("OK = true, want false")
			}
			if tc.got.SuggestedAction != tc.suggestedAction {
				t.Fatalf("SuggestedAction = %q, want %q", tc.got.SuggestedAction, tc.suggestedAction)
			}
			if len(tc.got.FieldPaths) == 0 {
				t.Fatal("FieldPaths is empty")
			}
		})
	}
}
