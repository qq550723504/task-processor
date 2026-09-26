package batchcapture

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestQueue(t *testing.T) *Queue {
	t.Helper()
	q := NewQueue("batch-1")
	q.Items = []Item{
		{Seq: 1, URL: "https://detail.1688.com/offer/1.html", State: ItemQueued},
		{Seq: 2, URL: "https://detail.1688.com/offer/2.html", State: ItemQueued},
	}
	return q
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "queue.json"))
	if !errors.Is(err, ErrQueueMissing) {
		t.Fatalf("want ErrQueueMissing, got %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q := newTestQueue(t)
	if err := q.ApproveScope("actor-1", "org-1"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	q.Items[0].State = ItemCaptured
	if err := q.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !got.ScopeApproved || got.ApprovedActorID != "actor-1" || got.ApprovedOrganizationID != "org-1" {
		t.Fatalf("scope not round-tripped: %+v", got)
	}
	if len(got.Items) != 2 || got.Items[0].State != ItemCaptured {
		t.Fatalf("items not round-tripped: %+v", got.Items)
	}
	if got.Items[0].ApprovedActorID != "actor-1" || got.Items[0].ApprovedOrganizationID != "org-1" {
		t.Fatalf("item scope not bound: %+v", got.Items[0])
	}
}

// TestSaveReplacesExistingFile proves the durable path never leaves a partially
// written queue behind: the previous file is either fully replaced or untouched.
func TestSaveReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q := newTestQueue(t)
	if err := q.Save(path); err != nil {
		t.Fatalf("save 1: %v", err)
	}
	q.Items = q.Items[:1]
	if err := q.Save(path); err != nil {
		t.Fatalf("save 2: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("expected replacement, got %d items", len(got.Items))
	}
	// No temp files may be left behind.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".batch-queue-") {
			t.Fatalf("temp queue file left behind: %s", e.Name())
		}
	}
}

func TestSaveRejectsUnapprovedEmptyScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q := newTestQueue(t)
	q.ScopeApproved = true // approved with empty values is inconsistent
	if err := q.Save(path); err == nil {
		t.Fatal("expected an inconsistent queue to be rejected")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inconsistent queue must not be written, stat err=%v", err)
	}
}

// TestLoadRejectsTamperedDigest covers the design's "corrupt queue file" case:
// editing durable content must be detected, not partially trusted.
func TestLoadRejectsTamperedDigest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q := newTestQueue(t)
	q.Items[0].State = ItemCaptured
	if err := q.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Flip a durable fact without recomputing the digest: an attacker or a
	// partially written file both look like this.
	tampered := strings.Replace(string(raw), `"state":"captured"`, `"state":"queued"`, 1)
	if tampered == string(raw) {
		t.Fatal("fixture did not contain the expected state")
	}
	if err := os.WriteFile(path, []byte(tampered), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); !errors.Is(err, ErrQueueCorrupt) {
		t.Fatalf("want ErrQueueCorrupt, got %v", err)
	}
}

// TestLoadRejectsTruncated covers a power loss mid-write being exposed.
func TestLoadRejectsTruncated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q := newTestQueue(t)
	if err := q.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := os.WriteFile(path, raw[:len(raw)/2], 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); !errors.Is(err, ErrQueueCorrupt) {
		t.Fatalf("want ErrQueueCorrupt, got %v", err)
	}
}

// TestLoadRejectsTrailingContent covers a queue with extra bytes appended.
func TestLoadRejectsTrailingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q := newTestQueue(t)
	if err := q.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := os.WriteFile(path, append(raw, []byte("\n{}")...), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); !errors.Is(err, ErrQueueCorrupt) {
		t.Fatalf("want ErrQueueCorrupt, got %v", err)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	var doc map[string]any
	doc = map[string]any{
		"format": QueueFormat, "batchId": "b", "items": []any{}, "digest": "x",
		"unexpected": true,
	}
	raw, _ := json.Marshal(doc)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); !errors.Is(err, ErrQueueCorrupt) {
		t.Fatalf("want ErrQueueCorrupt, got %v", err)
	}
}

// TestOnlyPreSubmitStatesAreRecapturable is the package's central safety rule:
// an item that may already have been published must never look re-doable.
func TestOnlyPreSubmitStatesAreRecapturable(t *testing.T) {
	recapturable := []ItemState{ItemQueued, ItemCapturing, ItemCaptured}
	for _, s := range recapturable {
		if !CanRecapture(s) {
			t.Fatalf("%s must be re-capturable (nothing was submitted)", s)
		}
		if RequiresHumanReview(s) {
			t.Fatalf("%s must not require human review", s)
		}
	}
	blocked := []ItemState{ItemSubmitting, ItemOutcomeUnknown}
	for _, s := range blocked {
		if CanRecapture(s) {
			t.Fatalf("%s must NEVER be re-capturable (may already be published)", s)
		}
		if !RequiresHumanReview(s) {
			t.Fatalf("%s must require human review", s)
		}
	}
	// Terminal, decided states are neither re-capturable nor blocking.
	for _, s := range []ItemState{ItemSubmitted, ItemFailed} {
		if CanRecapture(s) {
			t.Fatalf("%s must not be re-captured", s)
		}
		if RequiresHumanReview(s) {
			t.Fatalf("%s is decided and must not block the batch", s)
		}
	}
}

// TestBlockingItemStopsBatch covers the restart rule: a single submitting item
// halts the whole batch instead of being skipped.
func TestBlockingItemStopsBatch(t *testing.T) {
	q := newTestQueue(t)
	q.Items[1].State = ItemSubmitting
	item, blocked := q.BlockingItem()
	if !blocked || item.Seq != 2 {
		t.Fatalf("expected item 2 to block, got blocked=%v item=%+v", blocked, item)
	}
	q.Items[1].State = ItemSubmitted
	if _, blocked := q.BlockingItem(); blocked {
		t.Fatal("a submitted item must not block the batch")
	}
}

// TestScopeMatchRequiresApproval proves "observed session state" is not consent:
// an unapproved batch never matches, even against identical values.
func TestScopeMatchRequiresApproval(t *testing.T) {
	q := newTestQueue(t)
	if q.ScopeMatches("actor-1", "org-1") {
		t.Fatal("an unapproved batch must not match any scope")
	}
	// The guard must be the approval flag itself, not an incidental effect of
	// the fields being empty. A queue carrying populated values without the
	// approval flag is constructible (and is rejected by Validate), so the
	// comparison must still refuse it.
	unapproved := newTestQueue(t)
	unapproved.ApprovedActorID = "actor-1"
	unapproved.ApprovedOrganizationID = "org-1"
	if unapproved.ScopeMatches("actor-1", "org-1") {
		t.Fatal("populated-but-unapproved scope must not match: consent is the approval flag")
	}
	if err := q.ApproveScope("actor-1", "org-1"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if !q.ScopeMatches("actor-1", "org-1") {
		t.Fatal("approved scope must match itself")
	}
	if q.ScopeMatches("actor-1", "org-2") {
		t.Fatal("a different organization must not match")
	}
	if q.ScopeMatches("actor-2", "org-1") {
		t.Fatal("a different actor must not match")
	}
}

// TestValidateRejectsPopulatedButUnapprovedScope covers the fail-closed side of
// the same rule: an inconsistent file is rejected instead of being interpreted.
func TestValidateRejectsPopulatedButUnapprovedScope(t *testing.T) {
	q := newTestQueue(t)
	q.ApprovedActorID = "actor-1"
	q.ApprovedOrganizationID = "org-1"
	if err := q.Validate(); !errors.Is(err, ErrQueueCorrupt) {
		t.Fatalf("want ErrQueueCorrupt, got %v", err)
	}
}

// TestApproveScopeBindsEveryItem covers design D1.4: the approved scope is bound
// into each item so a restart can ask the right question.
func TestApproveScopeBindsEveryItem(t *testing.T) {
	q := newTestQueue(t)
	if err := q.ApproveScope("actor-9", "org-9"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	for _, item := range q.Items {
		if item.ApprovedActorID != "actor-9" || item.ApprovedOrganizationID != "org-9" {
			t.Fatalf("item %d was not bound to the approved scope", item.Seq)
		}
	}
	if err := q.ApproveScope("", "org"); err == nil {
		t.Fatal("an empty actor must be rejected")
	}
}

// TestValidateRejectsItemWithForeignScope covers an item that claims a scope the
// batch never approved, which would make read-back query the wrong tenant.
func TestValidateRejectsItemWithForeignScope(t *testing.T) {
	q := newTestQueue(t)
	if err := q.ApproveScope("actor-1", "org-1"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	q.Items[0].State = ItemSubmitting
	q.Items[0].ApprovedOrganizationID = "org-other"
	if err := q.Validate(); !errors.Is(err, ErrQueueCorrupt) {
		t.Fatalf("want ErrQueueCorrupt, got %v", err)
	}
}

// TestPhaseOneWriteCarriesNoKeyOrScope is the test the design calls
// "预写不含 key/scope": the pre-handoff marker must be writable before the
// handoff exists, so it cannot depend on handoff-produced values.
func TestPhaseOneWriteCarriesNoKeyOrScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q := newTestQueue(t)
	q.Items[0].State = ItemSubmitting
	if err := q.Save(path); err != nil {
		t.Fatalf("phase-one save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	item := got.Items[0]
	if item.IdempotencyKey != "" {
		t.Fatalf("phase one must not contain an idempotency key, got %q", item.IdempotencyKey)
	}
	if got.ScopeApproved {
		t.Fatal("phase one must not claim an approved scope it does not have")
	}
	if !RequiresHumanReview(item.State) {
		t.Fatalf("phase-one item must require review, got %s", item.State)
	}
}

// TestPhaseTwoBindsKeyAfterHandoff covers the second half of the two-phase
// pre-write: once the handoff URL exists, the key is persisted alongside the
// already-durable marker.
func TestPhaseTwoBindsKeyAfterHandoff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q := newTestQueue(t)
	if err := q.ApproveScope("actor-1", "org-1"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	q.Items[0].State = ItemSubmitting
	if err := q.Save(path); err != nil {
		t.Fatalf("phase-one save: %v", err)
	}
	// Phase two happens after the handoff page rendered.
	q.Items[0].IdempotencyKey = "11111111-1111-4111-8111-111111111111"
	if err := q.Save(path); err != nil {
		t.Fatalf("phase-two save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Items[0].IdempotencyKey != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("key not persisted: %+v", got.Items[0])
	}
	if CanRecapture(got.Items[0].State) {
		t.Fatal("an item with a bound key must still not be re-capturable")
	}
}

// TestSaveFailureIsNotDurable proves a failed write reports failure instead of
// silently succeeding, which is what makes "do not hand off unless the write is
// durable" implementable.
func TestSaveFailureIsNotDurable(t *testing.T) {
	dir := t.TempDir()
	// A path whose parent is a regular file cannot be created.
	blocker := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	q := newTestQueue(t)
	if err := q.Save(filepath.Join(blocker, "queue.json")); err == nil {
		t.Fatal("expected the save to fail so callers can refuse to hand off")
	}
}
