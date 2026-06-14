package studio

import "testing"

type batchStatusItem struct {
	status string
}

func TestAggregateBatchStatusDerivesKnownStates(t *testing.T) {
	tests := []struct {
		name  string
		items []batchStatusItem
		want  string
	}{
		{name: "empty", want: "draft"},
		{name: "all review ready", items: []batchStatusItem{{"review_ready"}, {"review_ready"}}, want: "review_ready"},
		{name: "all failed", items: []batchStatusItem{{"failed"}, {"failed"}}, want: "failed"},
		{name: "failed and ready", items: []batchStatusItem{{"failed"}, {"review_ready"}}, want: "partially_failed"},
		{name: "failed and active", items: []batchStatusItem{{"failed"}, {"generating"}}, want: "partially_failed"},
		{name: "ready and active", items: []batchStatusItem{{"review_ready"}, {"pending"}}, want: "partially_materialized"},
		{name: "active only", items: []batchStatusItem{{"pending"}, {"generating"}}, want: "generating"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AggregateBatchStatus(tt.items, func(item *batchStatusItem) string {
				return item.status
			}, testBatchStatusSet())
			if got != tt.want {
				t.Fatalf("AggregateBatchStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveBatchStatusPreservesCurrentStatus(t *testing.T) {
	got := ResolveBatchStatus(
		"tasks_created",
		[]batchStatusItem{{"review_ready"}},
		func(item *batchStatusItem) string { return item.status },
		testBatchStatusSet(),
		func(status string) bool { return status == "tasks_created" },
	)

	if got != "tasks_created" {
		t.Fatalf("ResolveBatchStatus() = %q, want tasks_created", got)
	}
}

func testBatchStatusSet() BatchStatusSet[string, string] {
	return BatchStatusSet[string, string]{
		Draft:                 "draft",
		Generating:            "generating",
		PartiallyMaterialized: "partially_materialized",
		ReviewReady:           "review_ready",
		PartiallyFailed:       "partially_failed",
		Failed:                "failed",
		ReviewReadyItem:       "review_ready",
		FailedItem:            "failed",
		ActiveItems:           []string{"pending", "generating", "awaiting_materialization"},
	}
}
