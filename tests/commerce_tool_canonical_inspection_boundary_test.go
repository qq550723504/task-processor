package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommerceToolCanonicalInspectionUsesOnlyNarrowDomainPorts(t *testing.T) {
	assertCanonicalInspectionImports(t, filepath.Join("..", "internal", "product", "catalog", "tools", "canonicalinspect"), map[string]struct{}{
		"task-processor/internal/authz":           {},
		"task-processor/internal/commercetool":    {},
		"task-processor/internal/listing/task":    {},
		"task-processor/internal/product/catalog": {},
	})
	assertCanonicalInspectionImports(t, filepath.Join("..", "internal", "listing", "task"), map[string]struct{}{})
}

func TestCommerceToolCanonicalInspectionDeclaresNoMutationOrCompatibilityOwner(t *testing.T) {
	toolRoot := filepath.Join("..", "internal", "product", "catalog", "tools", "canonicalinspect")
	files := canonicalInspectionProductionFiles(t, toolRoot)
	var violations []string
	for _, path := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		require.NoError(t, err)
		for _, declaration := range parsed.Decls {
			switch node := declaration.(type) {
			case *ast.FuncDecl:
				if forbiddenCanonicalInspectionOwnerName(node.Name.Name) {
					violations = append(violations, filepath.Base(path)+":"+node.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range node.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if ok && (typeSpec.Name.Name == "ProductSnapshot" || strings.Contains(typeSpec.Name.Name, "Repository")) {
						violations = append(violations, filepath.Base(path)+":"+typeSpec.Name.Name)
					}
				}
			}
		}
	}
	require.Empty(t, violations)

	listingFiles := canonicalInspectionProductionFiles(t, filepath.Join("..", "internal", "listing", "task"))
	for _, path := range listingFiles {
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		require.NoError(t, err)
		for _, declaration := range parsed.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range general.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				require.False(t, ok && strings.Contains(typeSpec.Name.Name, "Repository"), "%s declares a second task repository", path)
			}
		}
	}
}

func TestCommerceToolCanonicalInspectionGovernanceIsRecorded(t *testing.T) {
	config, err := os.ReadFile(filepath.Join("..", ".golangci.yml"))
	require.NoError(t, err)
	architecture, err := os.ReadFile(filepath.Join("..", "docs", "architecture", "project-target-architecture.md"))
	require.NoError(t, err)
	structure, err := os.ReadFile(filepath.Join("..", "docs", "development", "repository-structure.md"))
	require.NoError(t, err)

	for _, rule := range []string{"canonical_inspection_tool_boundaries", "listing_task_canonical_subject_boundaries", "commercetoolauth_boundaries"} {
		require.Contains(t, string(config), rule)
	}
	for _, text := range []string{"product.canonical.inspect", "internal/listing/task", "ProductSnapshot", "Phase 2B"} {
		require.Contains(t, string(architecture), text)
		require.Contains(t, string(structure), text)
	}
}

func assertCanonicalInspectionImports(t *testing.T, root string, internalAllowed map[string]struct{}) {
	t.Helper()
	var violations []string
	for _, path := range canonicalInspectionProductionFiles(t, root) {
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, spec := range parsed.Imports {
			imported := strings.Trim(spec.Path.Value, `"`)
			if !strings.HasPrefix(imported, "task-processor/internal/") {
				continue
			}
			if _, ok := internalAllowed[imported]; !ok {
				violations = append(violations, filepath.Base(path)+" -> "+imported)
			}
		}
	}
	sort.Strings(violations)
	require.Empty(t, violations)
}

func canonicalInspectionProductionFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(files)
	return files
}

func forbiddenCanonicalInspectionOwnerName(name string) bool {
	lower := strings.ToLower(name)
	for _, verb := range []string{"publish", "write", "mutate", "enqueue", "dispatch", "migrate", "handler", "route", "worker"} {
		if strings.Contains(lower, verb) {
			return true
		}
	}
	return false
}
