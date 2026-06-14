package workspace

import "testing"

func TestBuildStoreResolutionSummary(t *testing.T) {
	t.Parallel()

	summary := BuildStoreResolutionSummary(
		903,
		"GB",
		"rule_match",
		"matched profile",
		[]string{"site", "category"},
		88,
		true,
		false,
		"2026-06-14T12:00:00Z",
	)
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.StoreID != 903 || summary.Site != "GB" || summary.MatchedProfileID != 88 {
		t.Fatalf("summary = %+v", summary)
	}
	if len(summary.MatchedRuleKinds) != 2 || summary.MatchedRuleKinds[1] != "category" {
		t.Fatalf("matched rule kinds = %+v", summary.MatchedRuleKinds)
	}
}
