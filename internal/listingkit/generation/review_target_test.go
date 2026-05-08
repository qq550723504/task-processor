package generation

import "testing"

func TestReviewFocusKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		platform   string
		slot       string
		capability string
		want       string
	}{
		{name: "all parts", platform: "shein", slot: "main", capability: "detail_preview", want: "shein:main:detail_preview"},
		{name: "missing platform", slot: "main", capability: "detail_preview", want: "main:detail_preview"},
		{name: "platform only", platform: "shein", want: "shein"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ReviewFocusKey(tt.platform, tt.slot, tt.capability); got != tt.want {
				t.Fatalf("ReviewFocusKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestActionInteractionMode(t *testing.T) {
	t.Parallel()

	if got := ActionInteractionMode(ActionGenerateMissingAssets); got != "retryable" {
		t.Fatalf("ActionInteractionMode(generate) = %q, want retryable", got)
	}
	if got := ActionInteractionMode(ActionReviewDetailPreviews); got != "review_only" {
		t.Fatalf("ActionInteractionMode(preview review) = %q, want review_only", got)
	}
	if got := ActionInteractionMode(ActionInspectFailedTasks); got != "queue_only" {
		t.Fatalf("ActionInteractionMode(inspect) = %q, want queue_only", got)
	}
	if got := ActionInteractionMode("unknown"); got != "queue_only" {
		t.Fatalf("ActionInteractionMode(unknown) = %q, want queue_only", got)
	}
}
