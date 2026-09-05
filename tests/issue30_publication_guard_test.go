package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIssue30PublicationGuardClosesHTTPImportGap(t *testing.T) {
	root := t.TempDir()
	text := `package worker
import src "task-processor/internal/product/sourcing"
var identity = src.PublicationIdentity
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "worker.go"), []byte(text), 0600))
	violations, err := findBannedImportViolations(root, []string{`"task-processor/internal/app/productsourcing/httpapi"`}, nil, true)
	require.NoError(t, err)
	// Red-before-green: requiring the old guard to detect this fixture failed.
	require.Empty(t, violations, "the HTTP-import-only guard misses direct Product use")
	references, err := issue30PublicationIdentityViolations([]listingKitImageBoundarySource{{path: "internal/worker/worker.go", text: text}})
	require.NoError(t, err)
	require.Len(t, references, 1)
}

func TestIssue30PublicationIdentityHasNoProductionReferences(t *testing.T) {
	// Git-tracked source inventory includes nested modules and every build tag/OS.
	sources := trackedProductionTextSources(t, []string{"."}, issue30ProductionGoSource)
	require.NotEmpty(t, sources)
	violations, err := issue30PublicationIdentityViolations(sources)
	require.NoError(t, err)
	require.Empty(t, violations, "PublicationIdentity cutover requires independent approval")
}

func TestIssue30PublicationGuardReferenceForms(t *testing.T) {
	for _, tc := range []struct {
		name, path, text string
		want             int
	}{
		{"direct", "cmd/service/main.go", `package main; import "task-processor/internal/product/sourcing"; func run() { sourcing.PublicationIdentity() }`, 1},
		{"alias", "internal/worker/use.go", `package worker; import src "task-processor/internal/product/sourcing"; func run() { src.PublicationIdentity() }`, 1},
		{"function value", "main.go", `package main; import s "task-processor/internal/product/sourcing"; var use = s.PublicationIdentity`, 1},
		{"assignment", "scripts/import/main.go", `package main; import s "task-processor/internal/product/sourcing"; func run() { f := (s.PublicationIdentity); _ = f }`, 1},
		{"tagged nested module", "hack/debug/run_linux.go", "//go:build linux && custom\n\npackage main; import s \"task-processor/internal/product/sourcing\"; var use = s.PublicationIdentity", 1},
		{"tools", "tools/run_windows.go", `package main; import s "task-processor/internal/product/sourcing"; var use = s.PublicationIdentity`, 1},
		{"dot call", "internal/worker/dot.go", `package worker; import . "task-processor/internal/product/sourcing"; func run() { PublicationIdentity() }`, 1},
		{"dot value", "internal/worker/dot.go", `package worker; import . "task-processor/internal/product/sourcing"; var use = PublicationIdentity`, 1},
		{"same package call", "internal/product/sourcing/use.go", `package sourcing; func run() { PublicationIdentity() }`, 1},
		{"same package value", "internal/product/sourcing/use.go", `package sourcing; var use = PublicationIdentity`, 1},
		{"same file declaration and value", "internal/product/sourcing/publication_identity.go", `package sourcing; func PublicationIdentity() {}; var use = PublicationIdentity`, 1},
		{"declaration only", "internal/product/sourcing/publication_identity.go", `package sourcing; func PublicationIdentity() {}`, 0},
		{"tests", "internal/product/sourcing/publication_identity_test.go", `package sourcing; var use = PublicationIdentity`, 0},
		{"external tests", "internal/worker/use_test.go", `package worker; import s "task-processor/internal/product/sourcing"; var use = s.PublicationIdentity`, 0},
		{"testdata", "tests/testdata/example.go", `package sourcing; var use = PublicationIdentity`, 0},
		{"comments and strings", "internal/product/sourcing/comment.go", "package sourcing; // PublicationIdentity()\n const text = \"sourcing.PublicationIdentity\"", 0},
		{"other package symbol", "internal/worker/use.go", `package worker; import sourcing "example.com/other"; var use = sourcing.PublicationIdentity`, 0},
		{"import name shadow", "internal/product/sourcing/use.go", `package sourcing; import PublicationIdentity "example.com/other"; var use = PublicationIdentity.Other`, 0},
		{"same package name elsewhere", "internal/other/use.go", `package sourcing; func PublicationIdentity() {}; var use = PublicationIdentity`, 0},
		{"shadowed alias", "internal/worker/use.go", `package worker; import s "task-processor/internal/product/sourcing"; func run(s struct{PublicationIdentity func()}) { s.PublicationIdentity() }`, 0},
		{"local shadow", "internal/product/sourcing/use.go", `package sourcing; func run() { PublicationIdentity := func() {}; PublicationIdentity() }`, 0},
		{"dot shadow", "internal/worker/use.go", `package worker; import . "task-processor/internal/product/sourcing"; func run(PublicationIdentity func()) { PublicationIdentity() }`, 0},
		{"fields methods and keyed literals", "internal/product/sourcing/use.go", `package sourcing; type other struct{PublicationIdentity func()}; type iface interface{PublicationIdentity()}; type impl int; func (impl) PublicationIdentity() {}; var value = other{PublicationIdentity: func(){}}; func run(x other) { x.PublicationIdentity() }`, 0},
		{"other dot symbol", "internal/worker/use.go", `package worker; import . "task-processor/internal/product/sourcing"; var envelope SourceEnvelope`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			violations, err := issue30PublicationIdentityViolations([]listingKitImageBoundarySource{{path: tc.path, text: tc.text}})
			require.NoError(t, err)
			require.Len(t, violations, tc.want, strings.Join(violations, "\n"))
		})
	}
}
