package batchcapture

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Save flushes the file's directory, and every directory it created, AFTER the atomic
// replace. A flush that fails therefore happens while the new content is already
// visible, and every caller treats a failed save as "the file still holds the previous
// record" - the pre-handoff intent write reports that the item can be re-done on
// exactly that basis. The previous content is written back so the callers' assumption is
// true, and a save that cannot write it back says so instead of leaving the claim
// standing.

// failNthDirFlush fails the first n directory flush calls and then succeeds.
func failNthDirFlush(n int) (func(string) error, *int) {
	calls := new(int)
	return func(string) error {
		*calls++
		if *calls <= n {
			return os.ErrPermission
		}
		return nil
	}, calls
}

func TestSaveRestoresThePreviousContentWhenTheDirectoryFlushFailsAfterTheReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	queue := NewQueue("restore-batch")
	queue.Items = []Item{{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemCaptured}}
	if err := queue.Save(path); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read initial content: %v", err)
	}

	queue.Items[0].State = ItemSubmitting
	syncDirFn, calls := failNthDirFlush(1)
	err = queue.save(path, syncDirFn)
	if err == nil {
		t.Fatalf("a failed directory flush was reported as a successful save")
	}
	// The restore succeeded, so the caller may keep treating the file as holding the
	// previous record.
	if errors.Is(err, errQueueContentUnrestored) {
		t.Fatalf("a restored save was reported as unrestorable: %v", err)
	}
	if *calls < 2 {
		t.Fatalf("the directory was flushed %d times, want the failed flush plus the restore", *calls)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read content after the failed save: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("the file does not hold the previous content\n got: %s\nwant: %s", after, before)
	}
	stored, err := Load(path)
	if err != nil {
		t.Fatalf("reload queue: %v", err)
	}
	if stored.Items[0].State != ItemCaptured {
		t.Fatalf("state=%s, want %s", stored.Items[0].State, ItemCaptured)
	}

	// The restored file must still be a working queue file, not just bytes that happen
	// to match.
	if err := queue.Save(path); err != nil {
		t.Fatalf("save after a restore: %v", err)
	}
	if stored, err = Load(path); err != nil || stored.Items[0].State != ItemSubmitting {
		t.Fatalf("the restored file could not be replaced: state=%v err=%v", stored.Items[0].State, err)
	}
}

// The parent flush loop added for the directory-entry durability fix is a second
// post-replace failure point, and it needs the same treatment as the leaf flush.
//
// A parent flush can only fail for a save that created directories, and such a save is
// also one whose file did not exist before it, so the restore for this failure removes
// the file rather than rewriting a previous content.
func TestSaveRemovesTheFileWhenAParentDirectoryFlushFailsAfterTheReplace(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "a", "b", "queue.json")
	queue := NewQueue("parent-flush-batch")
	queue.Items = []Item{{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemSubmitting}}

	// The leaf directory flush succeeds; the parent of a directory this save created
	// fails. That is the flush the directory-entry fix added.
	syncDirFn := func(dir string) error {
		if dir == base {
			return os.ErrPermission
		}
		return nil
	}
	err := queue.save(path, syncDirFn)
	if err == nil {
		t.Fatalf("a failed parent directory flush was reported as a successful save")
	}
	if errors.Is(err, errQueueContentUnrestored) {
		t.Fatalf("the file was removable, so this was not an unrestorable save: %v", err)
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("the parent flush failure was lost: %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("the queue file the failed save created is still there: %v", statErr)
	}
}

// A queue save that fails for a reason unrelated to flushing (an unusable directory)
// must not be reported as unrestorable: nothing became visible, so the previous state
// is still what a reader sees.
func TestSaveDoesNotReportUnrestoredWhenNothingWasReplaced(t *testing.T) {
	base := t.TempDir()
	// Point the queue at something that cannot be a file.
	blocked := filepath.Join(base, "queue.json")
	if err := os.MkdirAll(blocked, 0o700); err != nil {
		t.Fatalf("seed directory: %v", err)
	}
	queue := NewQueue("blocked-batch")
	queue.Items = []Item{{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemSubmitting}}
	err := queue.save(blocked, func(string) error { return nil })
	if err == nil {
		t.Fatalf("replacing a directory was reported as a successful save")
	}
	if errors.Is(err, errQueueContentUnrestored) {
		t.Fatalf("a save that never replaced anything was reported as unrestorable: %v", err)
	}
}

// The marker means "the previous content could not be CONFIRMED", which is the
// fail-closed direction: a restore whose own flush fails has already renamed the
// previous content back, so the reader sees the previous record, but that state is not
// durable and the caller must not promise it. (A restore whose flush fails after a
// parent-directory failure is worse still, because the new content WAS durable before
// the restore, so the crash outcome depends on which flush failed.)
func TestSaveReportsUnrestoredContentWhenTheRestoreCannotBeConfirmed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	queue := NewQueue("unrestored-batch")
	queue.Items = []Item{{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemCaptured}}
	if err := queue.Save(path); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	queue.Items[0].State = ItemSubmitting
	syncDirFn, calls := failNthDirFlush(2) // the save's flush and the restore's flush
	err := queue.save(path, syncDirFn)
	if !errors.Is(err, errQueueContentUnrestored) {
		t.Fatalf("err=%v, want errQueueContentUnrestored", err)
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("the flush failure was lost: %v", err)
	}
	if *calls < 2 {
		t.Fatalf("directory flushed %d times, want the save's flush plus the restore's", *calls)
	}
	// The restore did rename the previous content back, so a reader sees the previous
	// record - but the save must still refuse to report the previous state as durable.
	stored, loadErr := Load(path)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.Items[0].State != ItemCaptured {
		t.Fatalf("state=%s, want %s in this scenario", stored.Items[0].State, ItemCaptured)
	}
}

// The dangerous shape: the save's replacement is still the visible and durable content
// (the previous content is unknown, so nothing could be written back). The marker is
// what stops the caller from reporting a re-capturable state the file does not hold.
func TestFlushFailedAfterReplaceMarksTheFileUnrestoredWhenNothingCouldBeWrittenBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	queue := NewQueue("unknown-previous-batch")
	queue.Items = []Item{{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemSubmitting}}
	if err := queue.Save(path); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	seeded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seeded content: %v", err)
	}

	flushErr := fmt.Errorf("flush queue directory: %w", os.ErrPermission)
	// os.ErrPermission is not os.ErrNotExist: the file exists but its previous content
	// is unknown, so removing it would lose a queue and rewriting it is impossible.
	err = flushFailedAfterReplace(path, nil, os.ErrPermission, func(string) error { return nil }, flushErr)

	if !errors.Is(err, errQueueContentUnrestored) {
		t.Fatalf("err=%v, want errQueueContentUnrestored", err)
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("the flush failure was lost: %v", err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read content after the failed save: %v", readErr)
	}
	if string(after) != string(seeded) {
		t.Fatalf("the file was changed although its previous content was unknown")
	}
	stored, loadErr := Load(path)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.Items[0].State != ItemSubmitting {
		t.Fatalf("state=%s, want %s: the marker is protecting nothing", stored.Items[0].State, ItemSubmitting)
	}
}

// A queue file that did not exist before the failed save must not be left behind: the
// previous state was "no file", and leaving a half-durable record would make the next
// run read an item nobody intended to record.
func TestSaveRemovesTheFileItCreatedWhenTheDirectoryFlushFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	queue := NewQueue("fresh-batch")
	queue.Items = []Item{{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemSubmitting}}

	syncDirFn, _ := failNthDirFlush(1)
	err := queue.save(path, syncDirFn)
	if err == nil {
		t.Fatalf("a failed directory flush was reported as a successful save")
	}
	if errors.Is(err, errQueueContentUnrestored) {
		t.Fatalf("the file was removable, so this was not an unrestorable save: %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("the queue file the failed save created is still there: %v", statErr)
	}
}

// An existing file whose content cannot be read must not be removed as if it had never
// existed: that would lose a queue rather than restore one.
func TestRestoreQueueFileLeavesAnUnreadableExistingFileAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	if err := os.WriteFile(path, []byte("{\"format\":\"unknown\"}"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	err := restoreQueueFile(path, nil, os.ErrPermission, func(string) error { return nil })
	if err == nil {
		t.Fatalf("an unreadable previous queue file was treated as restorable")
	}
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("the file was destroyed by a failed restore: %v", readErr)
	}
	if string(content) != "{\"format\":\"unknown\"}" {
		t.Fatalf("content=%s, want it untouched", content)
	}
}
