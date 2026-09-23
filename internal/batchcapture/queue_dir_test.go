package batchcapture

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The queue's directory must be durable before a handoff, and a directory's CONTENTS
// and its NAME are made durable by two different flushes: flushing the directory
// registers what is inside it, flushing its PARENT registers the directory itself.
//
// Flushing only the queue's own directory therefore leaves every directory on the path
// unregistered, and a power loss after a save that returned success can remove the whole
// tree. The consequence is not a lost file: the next run reads no queue at all, reports
// ErrQueueMissing, and ensureQueue rebuilds the item as queued - so an item that may
// already have been published becomes willing to be published again. That is the
// duplicate publication the durable pre-write exists to prevent, which is why these
// tests pin the flush of the whole path rather than only the leaf.
//
// The flush set is derived from the path, not from the directories a single call
// created. A save can create directories and then fail before registering them, leaving
// them behind for a retry that finds them already present; only walking the path each
// time repairs that, which the "re-registers" test below pins.

// indexOf returns the position of want in list, or -1.
func indexOf(list []string, want string) int {
	for i, item := range list {
		if item == want {
			return i
		}
	}
	return -1
}

// TestSaveFlushesTheQueueDirectoryAndEveryAncestor pins the successful-save flush
// sequence: the queue's own directory first (its contents changed), then every ancestor
// from the filesystem root down.
func TestSaveFlushesTheQueueDirectoryAndEveryAncestor(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "a", "b")
	path := filepath.Join(dir, "queue.json")

	var flushed []string
	record := func(d string) error {
		flushed = append(flushed, d)
		return nil
	}

	queue := NewQueue("batch-1")
	queue.Items = append(queue.Items, Item{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemQueued})
	if err := queue.save(path, record); err != nil {
		t.Fatalf("save: %v", err)
	}

	if len(flushed) < 3 {
		t.Fatalf("flushed %v, want the queue directory plus its ancestors", flushed)
	}
	if flushed[0] != dir {
		t.Fatalf("flushed %v, want the queue directory %q first", flushed, dir)
	}
	if flushed[len(flushed)-1] != filepath.Join(base, "a") {
		t.Fatalf("flushed %v, want the immediate parent last", flushed)
	}
	// Shallowest-first above the leaf: the root is registered before base, and base
	// before its child that names the queue directory.
	root := filepath.VolumeName(dir) + string(filepath.Separator)
	iRoot, iBase, iA := indexOf(flushed, root), indexOf(flushed, base), indexOf(flushed, filepath.Join(base, "a"))
	if iRoot < 0 || iBase < 0 || iA < 0 {
		t.Fatalf("flushed %v, want %q, %q and %q among them", flushed, root, base, filepath.Join(base, "a"))
	}
	if !(iRoot > 0 && iRoot < iBase && iBase < iA) {
		t.Fatalf("flushed %v, want the ancestors shallowest-first (root=%d base=%d a=%d)", flushed, iRoot, iBase, iA)
	}

	// The flushes did not disturb the write.
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if len(loaded.Items) != 1 || loaded.Items[0].State != ItemQueued {
		t.Fatalf("unexpected queue after Save: %+v", loaded.Items)
	}
}

// TestSaveReRegistersTheAncestorsAFailedSaveLeftBehind is the regression test for the
// retry hole in the earlier directory-durability fix.
//
// A save that creates the queue directory tree and then fails before registering one of
// those directories in its parent leaves exactly the state seeded here: the directory is
// present and the queue file is absent, but the directory's own entry is not durable.
// A retry that flushed only the directories it created would find nothing to do and
// report a durable save for a tree whose ancestors still cannot survive a power loss.
func TestSaveReRegistersTheAncestorsAFailedSaveLeftBehind(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "a", "b")
	path := filepath.Join(dir, "queue.json")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("seed the directory a failed save would leave behind: %v", err)
	}

	queue := NewQueue("batch-1")
	queue.Items = append(queue.Items, Item{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemQueued})

	// This save creates nothing (the directory is already there), so the flush set has to
	// come from the path itself.
	var flushed []string
	record := func(d string) error {
		flushed = append(flushed, d)
		return nil
	}
	if err := queue.save(path, record); err != nil {
		t.Fatalf("save over a directory left by a failed attempt: %v", err)
	}
	if len(flushed) < 2 || flushed[0] != dir {
		t.Fatalf("flushed %v, want the queue directory followed by its ancestors", flushed)
	}
	if flushed[len(flushed)-1] != filepath.Join(base, "a") {
		t.Fatalf("flushed %v, want the immediate parent last", flushed)
	}
	// base is the directory whose flush registers base/a, and filepath.Dir(base) is above
	// it; both must be flushed or the directory tree is still not registered.
	for _, ancestor := range []string{base, filepath.Dir(base)} {
		if indexOf(flushed, ancestor) < 0 {
			t.Fatalf("flushed %v, want it to re-register %q", flushed, ancestor)
		}
	}
}

// TestSavePropagatesAnAncestorDirectoryFlushFailure keeps the ancestor flush on the same
// fail-closed footing as the leaf flush: if a directory that has to remember the queue
// directory cannot be flushed, the save must not report success, because the caller uses
// that value to decide whether the item may become submittable.
//
// The injected failure is selective on purpose: the queue's own directory (dir) still
// flushes successfully, so only an ancestor flush can produce the error. Failing
// everything would be caught by the leaf flush and would leave this path untested.
func TestSavePropagatesAnAncestorDirectoryFlushFailure(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "a")
	path := filepath.Join(dir, "queue.json")

	queue := NewQueue("batch-1")
	queue.Items = append(queue.Items, Item{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemQueued})
	err := queue.save(path, func(d string) error {
		if d == base {
			return os.ErrPermission
		}
		return nil
	})
	if err == nil {
		t.Fatal("save reported success although an ancestor of the queue directory could not be flushed")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("save error %v does not carry the ancestor flush failure", err)
	}
}

// TestSavePropagatesADirectoryFlushFailure is the sibling of the ancestor case, and the
// regression test for the first directory-durability defect: a discarded flush error let
// Save report success for a rename that was not durable.
//
// The injected failure is selective on purpose: only the queue's OWN directory fails, so
// the error can only come from the leaf flush. If that flush were dropped, nothing would
// fail and the save would report success, which is exactly the defect.
func TestSavePropagatesADirectoryFlushFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "batch.json")
	dir := filepath.Dir(path)

	queue := NewQueue("batch-1")
	queue.Items = append(queue.Items, Item{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemQueued})
	err := queue.save(path, func(d string) error {
		if d == dir {
			return os.ErrPermission
		}
		return nil
	})
	if err == nil {
		t.Fatal("save reported success although the queue directory itself could not be flushed")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("save error %v does not carry the directory flush failure", err)
	}
}

// TestQueueSaveFlushesTheWholePathThroughItsInjectedFlush pins that Save's durability
// path is the injected flush and that the whole path goes through it. A direct syncDir
// call would be untestable without a power loss, and flushing only the leaf is the
// durability hole the two tests above exist to prevent.
func TestQueueSaveFlushesTheWholePathThroughItsInjectedFlush(t *testing.T) {
	source, err := os.ReadFile("queue.go")
	if err != nil {
		t.Fatalf("read queue.go: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "func (q *Queue) Save(path string) error { return q.save(path, syncDir) }") {
		t.Fatal("Save no longer injects syncDir into the durability path")
	}
	// The two flushes are checked inside save's own body: cutting at the next "func "
	// keeps the check from being satisfied by an identical call in a helper such as
	// restoreQueueFile, which is how a dropped leaf flush could pass unnoticed.
	_, saveBody, found := strings.Cut(text, "func (q *Queue) save(")
	if !found {
		t.Fatal("queue.go no longer defines save")
	}
	saveBody, _, found = strings.Cut(saveBody, "\nfunc ")
	if !found {
		t.Fatal("save is the last function in queue.go; the contract check needs a boundary")
	}
	if !strings.Contains(saveBody, "if err := syncDirFn(dir); err != nil {") {
		t.Fatal("save no longer flushes the queue's own directory through the injected flush")
	}
	if !strings.Contains(saveBody, "if err := flushQueueDirAncestors(dir, syncDirFn); err != nil {") {
		t.Fatal("save no longer flushes every ancestor of the queue directory through the injected flush")
	}
}
