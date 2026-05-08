package generation

import (
	"reflect"
	"testing"
)

func TestNavigationRulesForGenerationAction(t *testing.T) {
	t.Parallel()

	if got := NavigationRefreshScope("generation_action"); got != "mutation" {
		t.Fatalf("NavigationRefreshScope() = %q, want mutation", got)
	}
	if got := NavigationCachePolicy("generation_action"); got != "network_only" {
		t.Fatalf("NavigationCachePolicy() = %q, want network_only", got)
	}
	if got := NavigationDispatchStrategy("generation_action", 3); got != "mutation_then_refresh" {
		t.Fatalf("NavigationDispatchStrategy() = %q, want mutation_then_refresh", got)
	}
	if !NavigationRevalidateAfterAction("generation_action") {
		t.Fatalf("NavigationRevalidateAfterAction() should be true for generation_action")
	}
	if got := NavigationInvalidates("generation_action"); !reflect.DeepEqual(got, []string{"review_session", "review_preview", "generation_queue"}) {
		t.Fatalf("NavigationInvalidates() = %+v", got)
	}
}

func TestNavigationDispatchReadStrategies(t *testing.T) {
	t.Parallel()

	if got := NavigationDispatchStrategy("review_preview", 1); got != "single_read" {
		t.Fatalf("NavigationDispatchStrategy(single) = %q, want single_read", got)
	}
	if got := NavigationDispatchStrategy("review_preview", 2); got != "fanout_read" {
		t.Fatalf("NavigationDispatchStrategy(fanout) = %q, want fanout_read", got)
	}
	if got := NavigationDispatchFallbackStrategy("review_session", 2); got != "prefer_preview_then_session_then_queue" {
		t.Fatalf("NavigationDispatchFallbackStrategy() = %q, want preview/session/queue", got)
	}
	if !NavigationDispatchStopOnNotModified("review_preview", 1) {
		t.Fatalf("NavigationDispatchStopOnNotModified() should stop for single read")
	}
	if NavigationDispatchStopOnNotModified("generation_action", 1) {
		t.Fatalf("NavigationDispatchStopOnNotModified() should not stop for generation_action")
	}
}

func TestNavigationDispatchCachePreference(t *testing.T) {
	t.Parallel()

	if got := NavigationDispatchMaxParallelism("fanout_read"); got != 3 {
		t.Fatalf("NavigationDispatchMaxParallelism() = %d, want 3", got)
	}
	if got := NavigationDispatchStepCachePreference(false, "preview"); got != "stale_while_revalidate" {
		t.Fatalf("NavigationDispatchStepCachePreference(preview) = %q, want stale_while_revalidate", got)
	}
	if got := NavigationDispatchStepCachePreference(true, "preview"); got != "revalidate" {
		t.Fatalf("NavigationDispatchStepCachePreference(revalidate) = %q, want revalidate", got)
	}
}
