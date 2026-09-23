package batchcapture

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// The pre-handoff intent write is the record that makes an item submittable. Its failure
// handler tells the operator "nothing was handed off, so the item can be re-done" - a
// claim about the FILE, made on the assumption that a failed Save leaves the previous
// content in place.
//
// queue.Save upholds that assumption by writing the previous content back when a flush
// fails after the replacement, but it cannot always do so. When it cannot, the file may
// hold ItemSubmitting, and repeating the "you can re-do it" advice would send the
// operator at a record that forbids exactly that: BlockingItem stops the next run on a
// submitting item and requires a person to verify the application first.

func phaseOneTestItem() (*Item, ImportResult) {
	item := &Item{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemSubmitting}
	return item, ImportResult{Seq: item.Seq, URL: item.URL, State: ItemSubmitting, Verdict: VerdictProceed}
}

func TestPhaseOneFailedWriteDoesNotClaimTheItemCanBeRedoneWhenTheFileCouldNotBeRestored(t *testing.T) {
	item, result := phaseOneTestItem()
	err := fmt.Errorf("%w: flush queue directory: %v (and the previous content could not be restored: %v)",
		errQueueContentUnrestored, os.ErrPermission, os.ErrPermission)

	result, err = phaseOneFailedWrite(item, result, err, ItemCaptured)

	if !errors.Is(err, ErrItemStateUnconfirmed) {
		t.Fatalf("err=%v, want ErrItemStateUnconfirmed", err)
	}
	if !errors.Is(err, ErrQueueWriteFailed) {
		t.Fatalf("the write failure itself was lost: %v", err)
	}
	// Nothing reached the application, so this must not be reported as an unknown
	// outcome: that signal means "a payload may already be visible".
	if errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("a pre-handoff unrestored write was reported as an unknown outcome: %v", err)
	}
	// A wrapped cause would make NeedsVisibleSession offer "clear the gate and redo the
	// item", which is not available while the file may say submitting.
	if errors.Is(err, ErrVerdictStop) {
		t.Fatalf("the gate cause leaked into an unrestored write failure: %v", err)
	}
	if NeedsVisibleSession(result, err) {
		t.Fatalf("the item was offered for a redo although the file may record it as submitting")
	}
	if result.State != ItemSubmitting {
		t.Fatalf("state=%s, want %s", result.State, ItemSubmitting)
	}
	if CanRecapture(result.State) {
		t.Fatalf("result state %q is re-capturable although the file may hold submitting", result.State)
	}
	if item.State != ItemSubmitting {
		t.Fatalf("item state=%s, want %s", item.State, ItemSubmitting)
	}
	// The operator has to be told what to do about it, not just that something failed.
	if !strings.Contains(err.Error(), "verify in the application") {
		t.Fatalf("operator guidance is missing: %v", err)
	}
}

func TestPhaseOneFailedWriteKeepsTheItemRedoableWhenTheFileStillHoldsThePreviousState(t *testing.T) {
	item, result := phaseOneTestItem()
	err := fmt.Errorf("injected write failure")

	result, err = phaseOneFailedWrite(item, result, err, ItemCaptured)

	if !errors.Is(err, ErrQueueWriteFailed) {
		t.Fatalf("err=%v, want ErrQueueWriteFailed", err)
	}
	if errors.Is(err, ErrItemStateUnconfirmed) {
		t.Fatalf("an ordinary write failure was reported as a possibly-submitting record: %v", err)
	}
	if errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("a pre-handoff write failure was reported as an unknown outcome: %v", err)
	}
	if result.State != ItemCaptured {
		t.Fatalf("state=%s, want %s", result.State, ItemCaptured)
	}
	if !CanRecapture(result.State) {
		t.Fatalf("result state %q is not re-capturable", result.State)
	}
	if item.State != ItemCaptured || item.Reason != "" {
		t.Fatalf("item was left as %+v, want the previous state with no reason", item)
	}
	if !strings.Contains(err.Error(), "the item can be re-done") {
		t.Fatalf("operator guidance is missing: %v", err)
	}
}

// A pre-handoff stop that cannot restore the file may hold either state, and both are
// re-capturable - so the item stays re-doable, but the message must not assert which
// state the file contains.
func TestStopWithoutSubmitDoesNotAssertAStateItCouldNotRestore(t *testing.T) {
	item := &Item{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemCaptured}
	queue := NewQueue("stop-batch")
	queue.Items = []Item{*item}
	result := ImportResult{Seq: 1, URL: item.URL, Verdict: VerdictPauseBatch}
	cause := fmt.Errorf("%w: page classified as %s before any capture", ErrVerdictStop, VerdictPauseBatch)
	failRestore := func(*Queue, string) error {
		return fmt.Errorf("%w: flush queue directory: %v (and the previous content could not be restored: %v)",
			errQueueContentUnrestored, os.ErrPermission, os.ErrPermission)
	}

	result, err := stopWithoutSubmit(failRestore, queue, item, "queue.json", result, cause)

	if !errors.Is(err, ErrQueueWriteFailed) {
		t.Fatalf("err=%v, want ErrQueueWriteFailed", err)
	}
	// The cause must stay wrapped: NeedsVisibleSession is what tells the operator to
	// clear the gate in the still-open browser and redo the item.
	if !errors.Is(err, ErrVerdictStop) {
		t.Fatalf("the gate cause was dropped, so the redo affordance is gone: %v", err)
	}
	if !NeedsVisibleSession(result, err) {
		t.Fatalf("a gate stop that could not write lost its redo affordance")
	}
	if !CanRecapture(result.State) {
		t.Fatalf("result state %q is not re-capturable, but nothing was handed off", result.State)
	}
	if strings.Contains(err.Error(), "still records it as") {
		t.Fatalf("the message asserts a state the file may not hold: %v", err)
	}
	if !strings.Contains(err.Error(), "re-capturable") || !strings.Contains(err.Error(), "re-done") {
		t.Fatalf("operator guidance is missing: %v", err)
	}
}
