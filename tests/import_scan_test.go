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

func TestImportMatchesPrefixOnlyAtPackageBoundary(t *testing.T) {
	for _, tc := range []struct {
		name       string
		importPath string
		prefix     string
		want       bool
	}{
		{name: "exact", importPath: "task-processor/internal/listingkit", prefix: "task-processor/internal/listingkit", want: true},
		{name: "subpackage", importPath: "task-processor/internal/listingkit/core", prefix: "task-processor/internal/listingkit", want: true},
		{name: "sibling", importPath: "task-processor/internal/listingkitten", prefix: "task-processor/internal/listingkit", want: false},
		{name: "different", importPath: "task-processor/internal/catalog", prefix: "task-processor/internal/listingkit", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := importMatchesPrefix(tc.importPath, tc.prefix); got != tc.want {
				t.Fatalf("importMatchesPrefix(%q, %q) = %v, want %v", tc.importPath, tc.prefix, got, tc.want)
			}
		})
	}
}

func TestFindBannedImportViolationsDecodesLiteralsAndHonorsAllowlist(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"plain.go": `package fixture
import sdkclient "go.temporal.io/sdk/client"
var _ sdkclient.Client
`,
		"raw.go": "package fixture\nimport sdkclient `go.temporal.io/sdk/client`\nvar _ sdkclient.Client\n",
		"escaped.go": `package fixture
import sdkclient "go.temporal.io/sdk/\x63lient"
var _ sdkclient.Client
`,
		"client_test.go": `package fixture
import sdkclient "go.temporal.io/sdk/client"
var _ sdkclient.Client
`,
		"allowed.go": `package fixture
import sdkclient "go.temporal.io/sdk/client"
var _ sdkclient.Client
`,
		filepath.Join("allowed", "nested.go"): `package fixture
import sdkclient "go.temporal.io/sdk/client"
var _ sdkclient.Client
`,
	}
	for relative, source := range files {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	allowed := map[string]struct{}{
		filepath.Join(root, "allowed.go"):                         {},
		filepath.Join(root, "allowed") + string(os.PathSeparator): {},
	}

	violations, err := findBannedImportViolations(root, []string{`"go.temporal.io/sdk/client"`}, allowed, false)
	if err != nil {
		t.Fatal(err)
	}
	assertBannedImportViolationFiles(t, violations, "plain.go", "raw.go", "escaped.go", "client_test.go")

	productionViolations, err := findBannedImportViolations(root, []string{`"go.temporal.io/sdk/client"`}, allowed, true)
	if err != nil {
		t.Fatal(err)
	}
	assertBannedImportViolationFiles(t, productionViolations, "plain.go", "raw.go", "escaped.go")
}

func TestFindBannedImportViolationsRejectsMalformedBannedImportConfiguration(t *testing.T) {
	_, err := findBannedImportViolations(t.TempDir(), []string{"not-a-go-string-literal"}, nil, false)
	if err == nil || !strings.Contains(err.Error(), "decode banned import") {
		t.Fatalf("error = %v, want malformed banned import configuration error", err)
	}
}

func assertBannedImportViolationFiles(t *testing.T, violations []bannedImportViolation, wantFiles ...string) {
	t.Helper()
	got := make(map[string]struct{}, len(violations))
	for _, violation := range violations {
		if violation.importPath != "go.temporal.io/sdk/client" {
			t.Errorf("decoded import = %q, want go.temporal.io/sdk/client", violation.importPath)
		}
		got[filepath.Base(violation.path)] = struct{}{}
	}
	if len(got) != len(wantFiles) {
		t.Fatalf("violation files = %v, want %v", got, wantFiles)
	}
	for _, want := range wantFiles {
		if _, ok := got[want]; !ok {
			t.Errorf("missing violation for %s; got %v", want, got)
		}
	}
}
