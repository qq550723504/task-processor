package listingkit

import "testing"

func TestShapeSheinSubmitReadinessSummaryAppendsGeneralLabels(t *testing.T) {
	t.Parallel()

	readiness := &SheinSubmitReadiness{
		Summary: []string{"骨架已就绪"},
		BlockingItems: []SheinReadinessItem{
			{Key: "category", Label: "类目骨架", Message: "类目仍未确认"},
		},
		WarningItems: []SheinReadinessItem{
			{Key: "manual_notes", Label: "人工备注"},
		},
	}

	got := shapeSheinSubmitReadinessSummary(readiness, sheinSubmitReadinessSummaryShape{
		blockingLabel: "待补关键项：",
		warningLabel:  "待确认项：",
	})

	if got == nil {
		t.Fatal("got nil readiness")
	}
	if len(got.Summary) != 3 {
		t.Fatalf("summary = %#v, want 3 lines", got.Summary)
	}
	if got.Summary[1] != "待补关键项：类目骨架" {
		t.Fatalf("summary[1] = %q, want blocking label line", got.Summary[1])
	}
	if got.Summary[2] != "待确认项：人工备注" {
		t.Fatalf("summary[2] = %q, want warning label line", got.Summary[2])
	}
}

func TestShapeSheinSubmitReadinessSummaryPrependsFirstFreshnessBlocker(t *testing.T) {
	t.Parallel()

	readiness := &SheinSubmitReadiness{
		Summary: []string{"在线模板仍可用于当前提交"},
		BlockingItems: []SheinReadinessItem{
			{Key: "category", Label: "类目模板新鲜度", Message: "当前类目模板在线校验失败"},
		},
	}

	got := shapeSheinSubmitReadinessSummary(readiness, sheinSubmitReadinessSummaryShape{
		blockingLabel:       "在线阻断项：",
		prependFirstBlocker: true,
	})

	if got == nil {
		t.Fatal("got nil readiness")
	}
	if len(got.Summary) != 3 {
		t.Fatalf("summary = %#v, want 3 lines", got.Summary)
	}
	if got.Summary[0] != "当前类目模板在线校验失败" {
		t.Fatalf("summary[0] = %q, want first blocker message", got.Summary[0])
	}
	if got.Summary[2] != "在线阻断项：类目模板新鲜度" {
		t.Fatalf("summary[2] = %q, want freshness blocking label line", got.Summary[2])
	}
}
