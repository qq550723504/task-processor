package workspace

import (
	"testing"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

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

func TestBuildSubmissionStoreResolution(t *testing.T) {
	t.Parallel()

	resolvedAt := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	resolution := BuildSubmissionStoreResolution(
		903,
		"GB",
		"rule_match",
		"matched profile",
		[]string{"site", "category"},
		88,
		true,
		false,
		&resolvedAt,
	)
	if resolution == nil {
		t.Fatal("expected resolution")
	}
	if resolution.StoreID != 903 || resolution.Site != "GB" || resolution.MatchedProfileID != 88 {
		t.Fatalf("resolution = %+v", resolution)
	}
	if resolution.ResolvedAt == nil || !resolution.ResolvedAt.Equal(resolvedAt) {
		t.Fatalf("resolved_at = %v, want %v", resolution.ResolvedAt, resolvedAt)
	}
}

func TestAttachSubmissionEventStoreResolution(t *testing.T) {
	t.Parallel()

	storeResolution := &sheinpub.SubmissionStoreResolution{StoreID: 903, Site: "GB"}
	events := []sheinpub.SubmissionEvent{
		{Action: "publish"},
		{Action: "save_draft", StoreResolution: &sheinpub.SubmissionStoreResolution{StoreID: 1001, Site: "US"}},
	}

	items := AttachSubmissionEventStoreResolution(events, storeResolution)

	if items[0].StoreResolution == nil || items[0].StoreResolution.StoreID != 903 {
		t.Fatalf("first item = %+v", items[0])
	}
	if items[1].StoreResolution == nil || items[1].StoreResolution.StoreID != 1001 {
		t.Fatalf("second item = %+v", items[1])
	}
	if events[0].StoreResolution != nil {
		t.Fatalf("expected original events to stay unchanged, got %+v", events[0])
	}
}
