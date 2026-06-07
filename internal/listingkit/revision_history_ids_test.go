package listingkit

import "testing"

func TestBuildRevisionTimelineSummaryUsesFreshnessSpecificHeadlines(t *testing.T) {
	t.Parallel()

	record := ListingKitRevisionRecord{
		Platform:   "shein",
		ActionType: RevisionActionTypeEdit,
		Reason:     "Refresh SHEIN category",
	}

	summary := buildRevisionTimelineSummary(record)
	if summary == nil {
		t.Fatal("expected timeline summary")
	}
	if summary.Headline != "刷新 SHEIN 类目模板" {
		t.Fatalf("headline = %q", summary.Headline)
	}
	if summary.RelationText != "将重算类目 / 普通属性 / 销售属性" {
		t.Fatalf("relation text = %q", summary.RelationText)
	}
}

func TestBuildRevisionTimelineSummaryUsesRegenerateSpecificRelationText(t *testing.T) {
	t.Parallel()

	record := ListingKitRevisionRecord{
		Platform:   "shein",
		ActionType: RevisionActionTypeEdit,
		Reason:     "Regenerate SHEIN sale attributes",
	}

	summary := buildRevisionTimelineSummary(record)
	if summary == nil {
		t.Fatal("expected timeline summary")
	}
	if summary.Headline != "刷新 SHEIN 销售属性" {
		t.Fatalf("headline = %q", summary.Headline)
	}
	if summary.RelationText != "将按最新模板重新生成销售属性" {
		t.Fatalf("relation text = %q", summary.RelationText)
	}
}

func TestBuildRevisionTimelineSummaryFallsBackForGenericEdits(t *testing.T) {
	t.Parallel()

	record := ListingKitRevisionRecord{
		Platform:   "shein",
		ActionType: RevisionActionTypeEdit,
		Reason:     "manual change",
	}

	summary := buildRevisionTimelineSummary(record)
	if summary == nil {
		t.Fatal("expected timeline summary")
	}
	if summary.Headline != "更新 SHEIN 资料" {
		t.Fatalf("headline = %q", summary.Headline)
	}
}
