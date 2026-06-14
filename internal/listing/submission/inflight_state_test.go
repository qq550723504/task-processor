package submission

import "testing"

func TestShouldClearInFlight(t *testing.T) {
	t.Parallel()

	if !ShouldClearInFlight("publish", "req-1", "publish", "req-1") {
		t.Fatal("expected matching action/request to clear in-flight state")
	}
	if ShouldClearInFlight("publish", "req-1", "save_draft", "req-1") {
		t.Fatal("mismatched action should not clear in-flight state")
	}
	if ShouldClearInFlight("publish", "req-1", "publish", "req-2") {
		t.Fatal("mismatched request should not clear in-flight state")
	}
}
