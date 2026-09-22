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
// Flushing only the queue's directory therefore leaves a directory this run created
// unregistered, and a power loss after a save that returned success can remove the
// whole tree.
//
// The consequence is not a lost file: the next run reads no queue at all, reports
// ErrQueueMissing, and ensureQueue rebuilds the item as queued - so an item that may
// already have been published becomes willing to be published again. That is the
// duplicate publication the durable pre-write exists to prevent, which is why these
// tests pin the flush of every created parent rather than only the leaf.
func TestMkdirQueueDirReportsEveryDirectoryItCreated(t *testing.T) {
	base := t.TempDir()
	deep := filepath.Join(base, "a", "b", "c")

	created, err := mkdirQueueDir(deep)
	if err != nil {
		t.Fatalf("mkdirQueueDir: %v", err)
	}
	// Shallowest-first, so Save registers each parent before the child it names.
	want := []string{
		filepath.Join(base, "a"),
		filepath.Join(base, "a", "b"),
		deep,
	}
	if len(created) != len(want) {
		t.Fatalf("created=%v, want %v", created, want)
	}
	for i := range want {
		if created[i] != want[i] {
			t.Fatalf("created=%v, want %v", created, want)
		}
	}
	if info, err := os.Stat(deep); err != nil || !info.IsDir() {
		t.Fatalf("the directory was not created: %v", err)
	}

	// Idempotent: a directory that already exists is not re-created and does not need
	// its parent flushed again.
	again, err := mkdirQueueDir(deep)
	if err != nil {
		t.Fatalf("mkdirQueueDir (second call): %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("a second call reported created=%v, want none", again)
	}
}

// TestSaveFlushesTheParentOfEveryNewlyCreatedDirectory is the regression test for the
// durability hole: Save flushed only the queue's own directory, so a directory it had
// just created was never registered in its parent.
func TestSaveFlushesTheParentOfEveryNewlyCreatedDirectory(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "a", "b")
	path := filepath.Join(dir, "batch.json")

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

	// The leaf directory is always flushed (its contents changed); on top of that, one
	// flush per directory this call created, shallowest-first: base registers a itself,
	// then base/a registers a/b (and a's contents, a/b among them).
	want := []string{dir, base, filepath.Join(base, "a")}
	if len(flushed) != len(want) {
		t.Fatalf("flushed parents %v, want %v", flushed, want)
	}
	for i := range want {
		if flushed[i] != want[i] {
			t.Fatalf("flushed parents %v, want %v", flushed, want)
		}
	}

	// The queue is readable, so the flush did not disturb the write.
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if len(loaded.Items) != 1 || loaded.Items[0].State != ItemQueued {
		t.Fatalf("unexpected queue after Save: %+v", loaded.Items)
	}
}

// TestSaveFlushesOnlyTheQueueDirectoryWhenItAlreadyExists keeps the previous test
// honest: a directory that already exists needs its own flush (its contents changed)
// but not its parent's, so it must be observing the creation path rather than a parent
// flush that happens on every save.
func TestSaveFlushesOnlyTheQueueDirectoryWhenItAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "batch.json")

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
	if len(flushed) != 1 || flushed[0] != dir {
		t.Fatalf("flushed %v, want only the queue directory %q", flushed, dir)
	}
}

// TestSavePropagatesANewlyCreatedDirectoryParentFlushFailure keeps the new flush on
// the same fail-closed footing as the leaf flush: if the parent that has to remember
// the directory cannot be flushed, the save must not report success, because the
// caller uses that value to decide whether the item may become submittable.
//
// The injected failure is selective on purpose: the leaf directory (dir) still flushes
// successfully, so only the parent flush can produce the error. Failing everything
// would be caught by the leaf flush and would leave this path untested.
func TestSavePropagatesANewlyCreatedDirectoryParentFlushFailure(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "a")
	path := filepath.Join(dir, "batch.json")

	queue := NewQueue("batch-1")
	queue.Items = append(queue.Items, Item{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemQueued})
	err := queue.save(path, func(d string) error {
		if d == base {
			return os.ErrPermission
		}
		return nil
	})
	if err == nil {
		t.Fatal("save reported success although a newly created directory's parent could not be flushed")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("save error %v does not carry the parent flush failure", err)
	}
}

// TestQueueSaveRoutesDirectoryCreationThroughMkdirQueueDir pins that Save cannot go
// back to a plain MkdirAll, which never reports which directories were created and
// therefore cannot flush their parents.
// TestSavePropagatesADirectoryFlushFailure is the sibling of the newly created
// directory case, and the regression test for the first directory-durability defect:
// a discarded flush error let Save report success for a rename that was not durable.
func TestSavePropagatesADirectoryFlushFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "batch.json")

	queue := NewQueue("batch-1")
	queue.Items = append(queue.Items, Item{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemQueued})
	if err := queue.save(path, func(string) error { return os.ErrPermission }); err == nil {
		t.Fatal("save reported success although the queue directory could not be flushed")
	}
}

func TestQueueSaveRoutesDirectoryCreationThroughMkdirQueueDir(t *testing.T) {
	source, err := os.ReadFile("queue.go")
	if err != nil {
		t.Fatalf("read queue.go: %v", err)
	}
	text := string(source)
	// Scope the MkdirAll ban to Save's body: mkdirQueueDir is allowed to use it, but the
	// caller must not, because it cannot report which directories were created.
	saveBody, _, found := strings.Cut(text, "func mkdirQueueDir(")
	if !found {
		t.Fatal("queue.go no longer defines mkdirQueueDir")
	}
	if strings.Contains(saveBody, "os.MkdirAll(") {
		t.Fatal("Save creates the queue directory with a bare MkdirAll, which cannot report which directories were created")
	}
	if !strings.Contains(text, "createdDirs, err := mkdirQueueDir(dir)") {
		t.Fatal("queue.go no longer obtains the created directories before writing")
	}
	if !strings.Contains(text, "syncDirFn(filepath.Dir(created))") {
		t.Fatal("queue.go no longer flushes the parent of each directory it created")
	}
	if !strings.Contains(text, "func (q *Queue) Save(path string) error { return q.save(path, syncDir) }") {
		t.Fatal("Save no longer injects syncDir into the durability path")
	}
}
