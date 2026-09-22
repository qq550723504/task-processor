package batchcapture

import (
	"os"
	"strings"
	"testing"
)

// The submit-intent record must be written at the handoff boundary, not when the item
// is selected.
//
// Design section 4 D2.1 fixes the item state machine at queued -> capturing ->
// captured -> submitting and places phase one "发 popup.handoff 之前". Persisting
// submitting any earlier makes that record a lie: navigation, page classification and
// extension capture cannot reach the application, so a process killed there would leave
// an item that provably never left the executor permanently blocked on human
// verification. That contradicts the same section's rule that only a state that may
// have reached a submission stops auto-recapture, and D1.3's "the item stays
// re-doable" for a pre-handoff stop.
//
// The bug only shows up when the process is killed mid-navigation, which a producing
// test cannot reproduce, so this pins the source order instead: the record has to come
// after the handoff has been prepared and before it is dispatched.
func TestSubmittingIntentIsRecordedAtTheHandoffBoundary(t *testing.T) {
	raw, err := os.ReadFile("import.go")
	if err != nil {
		t.Fatalf("read import.go: %v", err)
	}
	text := string(raw)

	index := func(marker string) int {
		t.Helper()
		i := strings.Index(text, marker)
		if i < 0 {
			t.Fatalf("import.go no longer contains %q", marker)
		}
		return i
	}
	prepareItem := index("popup, err := driver.PrepareItem(item.URL)")
	prepare := index("popup.PrepareHandoff()")
	phaseOne := index("item.State = ItemSubmitting")
	dispatch := index("popup.DispatchHandoff()")

	if !(prepareItem < prepare && prepare < phaseOne && phaseOne < dispatch) {
		t.Fatalf("the submit intent is not recorded at the handoff boundary: "+
			"PrepareItem@%d PrepareHandoff@%d submitting@%d DispatchHandoff@%d",
			prepareItem, prepare, phaseOne, dispatch)
	}
	// Naming the state is not enough: the record has to be flushed before the click,
	// or a crash between the two would still allow the click without a durable record.
	between := text[phaseOne:dispatch]
	if !strings.Contains(between, "persist(queue, opts.QueuePath)") {
		t.Fatalf("the submit intent is assigned but not persisted before the dispatch")
	}
}
