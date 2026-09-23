package batchcapture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Save replaces the queue file on every write after the first, so the replacement
// must work against an existing destination. Both branches matter: creating the
// file and replacing it.
func TestReplaceFileReplacesAnExistingTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "queue.tmp")
	target := filepath.Join(dir, "queue.json")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.WriteFile(source, []byte("new"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := replaceFile(source, target); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("target=%q, want %q", got, "new")
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("the temporary file survived the replace: %v", err)
	}
}

func TestReplaceFileCreatesAMissingTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "queue.tmp")
	target := filepath.Join(dir, "queue.json")
	if err := os.WriteFile(source, []byte("first"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := replaceFile(source, target); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "first" {
		t.Fatalf("target=%q, want %q", got, "first")
	}
}

// Reporting success for a replace that cannot have happened would let Save claim a
// durable record it never wrote, so a missing source must be an error.
func TestReplaceFileReportsAMissingSource(t *testing.T) {
	dir := t.TempDir()
	if err := replaceFile(filepath.Join(dir, "absent.tmp"), filepath.Join(dir, "queue.json")); err == nil {
		t.Fatal("replacing from a missing source reported success")
	}
}

// replaceFile is only a guarantee if Save actually goes through it.
//
// Reverting the replacement to os.Rename would keep every functional test passing —
// os.Rename also replaces the file, it just does not make the replacement durable —
// and the flag assertion above would still pass because the constant would remain
// declared and unused. Since durability cannot be observed from a running process,
// the one thing that can be checked is which call Save makes.
func TestQueueSaveUsesTheDurableReplace(t *testing.T) {
	source, err := os.ReadFile("queue.go")
	if err != nil {
		t.Fatalf("read queue.go: %v", err)
	}
	text := string(source)
	if strings.Contains(text, "os.Rename(") {
		t.Fatal("queue.go replaces the queue file with os.Rename, which is not a confirmed durable replacement")
	}
	if !strings.Contains(text, "replaceFile(tmpName, path)") {
		t.Fatal("queue.go no longer performs its replacement through replaceFile")
	}
}
