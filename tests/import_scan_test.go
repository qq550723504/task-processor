package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGoFileIndexCollectsImportsAndSelectors(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "sample.go")
	source := `package sample

import (
	productenrich "task-processor/internal/productenrich"
	"fmt"
)

func example() {
	_ = fmt.Sprintf("%v", productenrich.CanonicalProduct{})
}
`
	if err := os.WriteFile(filePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	file, ok := index.files[filepath.Clean(filePath)]
	if !ok {
		t.Fatalf("expected indexed file %s", filePath)
	}
	if _, ok := file.imports[`"fmt"`]; !ok {
		t.Fatalf("expected fmt import to be indexed")
	}
	if _, ok := file.imports[`"task-processor/internal/productenrich"`]; !ok {
		t.Fatalf("expected productenrich import to be indexed")
	}
	if !strings.Contains(string(file.source), "productenrich.CanonicalProduct") {
		t.Fatalf("expected source to include productenrich.CanonicalProduct selector")
	}
}

func TestLoadGoFileIndexSkipsSubtree(t *testing.T) {
	root := t.TempDir()
	keepFile := filepath.Join(root, "keep.go")
	skipDir := filepath.Join(root, "skip")
	skipFile := filepath.Join(skipDir, "skip.go")

	if err := os.MkdirAll(skipDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keepFile, []byte("package sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skipFile, []byte("package sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	index, err := loadGoFileIndex(root, skipDir)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := index.files[filepath.Clean(keepFile)]; !ok {
		t.Fatalf("expected keep.go to be indexed")
	}
	if _, ok := index.files[filepath.Clean(skipFile)]; ok {
		t.Fatalf("expected skip.go to be excluded from index")
	}
}

func TestPathAllowedMatchesFileAndDirectoryAllowlist(t *testing.T) {
	root := t.TempDir()
	allowedFile := filepath.Join(root, "allowed.go")
	allowedDirFile := filepath.Join(root, "adapters", "allowed.go")
	blockedFile := filepath.Join(root, "blocked.go")

	allowed := map[string]struct{}{
		filepath.Clean(allowedFile): {},
		filepath.Clean(filepath.Dir(allowedDirFile)) + string(os.PathSeparator): {},
	}

	for _, tc := range []struct {
		name string
		path string
		want bool
	}{
		{name: "file", path: allowedFile, want: true},
		{name: "directory", path: allowedDirFile, want: true},
		{name: "blocked", path: blockedFile, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathAllowed(tc.path, allowed); got != tc.want {
				t.Fatalf("pathAllowed(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
