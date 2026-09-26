package batchcapture

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// An item whose outcome cannot be decided locally must stop the batch before the
// executor touches a browser or submits anything else.
//
// Skipping past that item is the dangerous shape: it submits a later item while the
// earlier one may already have been published, and it replaces the mandatory
// "verify, do not re-run" instruction with whatever the later item's run reports.
// Design section 4 D2.2 plus the section 10 restart rows require the whole batch to
// stop; section 10's "only captured is re-capturable" row is what makes this
// checkable at all.
//
// The driver is deliberately nil: the gate has to return before any browser work, so
// a test that cannot survive touching the driver proves the executor never got there.
func TestImportOneStopsOnAnAmbiguousItemBeforeTouchingTheBrowser(t *testing.T) {
	scope := AppScope{ActorID: "actor-a", OrganizationID: "org-a"}
	path := filepath.Join(t.TempDir(), "queue.json")

	queue := NewQueue("restarted-batch")
	queue.Items = []Item{
		{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemSubmitting},
		{Seq: 2, URL: "https://detail.1688.com/offer/932524015351.html", State: ItemQueued},
	}
	if err := queue.ApproveScope(scope.ActorID, scope.OrganizationID); err != nil {
		t.Fatalf("approve scope: %v", err)
	}
	if err := queue.Save(path); err != nil {
		t.Fatalf("save queue: %v", err)
	}

	_, err := ImportOne(nil, ImportOptions{QueuePath: path, Approved: scope})
	if !errors.Is(err, ErrBatchBlocked) {
		t.Fatalf("err=%v, want ErrBatchBlocked", err)
	}
	// "Nothing can be done safely until a person verifies the application" must not
	// be reported as "there is nothing to do": the second message reads as safe.
	if errors.Is(err, ErrNoQueuedItem) {
		t.Fatalf("a blocked batch was reported as an empty queue: %v", err)
	}
	for _, want := range []string{"1", string(ItemSubmitting), "do not re-run"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("operator guidance is missing %q: %v", want, err)
		}
	}

	stored, err := Load(path)
	if err != nil {
		t.Fatalf("reload queue: %v", err)
	}
	if stored.Items[1].State != ItemQueued {
		t.Fatalf("item 2 was touched: %s", stored.Items[1].State)
	}
	if stored.Items[1].IdempotencyKey != "" {
		t.Fatalf("item 2 gained an idempotency key")
	}
	if stored.Items[0].State != ItemSubmitting {
		t.Fatalf("the blocking item was rewritten: %s", stored.Items[0].State)
	}
}

// The blocking item must be reported even when it is the only item: telling the
// operator about the item that needs checking is the entire point of stopping.
func TestImportOneReportsTheBlockingItemWhenItIsTheOnlyOne(t *testing.T) {
	scope := AppScope{ActorID: "actor-a", OrganizationID: "org-a"}
	path := filepath.Join(t.TempDir(), "queue.json")

	queue := NewQueue("restarted-batch")
	queue.Items = []Item{{
		Seq:    7,
		URL:    "https://detail.1688.com/offer/981645030344.html",
		State:  ItemOutcomeUnknown,
		Reason: "confirm and submit: no terminal result",
	}}
	if err := queue.ApproveScope(scope.ActorID, scope.OrganizationID); err != nil {
		t.Fatalf("approve scope: %v", err)
	}
	if err := queue.Save(path); err != nil {
		t.Fatalf("save queue: %v", err)
	}

	_, err := ImportOne(nil, ImportOptions{QueuePath: path, Approved: scope})
	if !errors.Is(err, ErrBatchBlocked) {
		t.Fatalf("err=%v, want ErrBatchBlocked", err)
	}
	if errors.Is(err, ErrNoQueuedItem) {
		t.Fatalf("a blocked batch was reported as an empty queue: %v", err)
	}
	if !strings.Contains(err.Error(), "7") {
		t.Fatalf("the blocking item's sequence number is missing: %v", err)
	}
	if !strings.Contains(err.Error(), "no terminal result") {
		t.Fatalf("the blocking item's recorded reason is missing: %v", err)
	}
}
