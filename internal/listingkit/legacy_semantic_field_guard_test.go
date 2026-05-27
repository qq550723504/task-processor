package listingkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestLegacySemanticFieldsStayInsideCompatibilityBoundaries(t *testing.T) {
	t.Parallel()

	repoRoot := repositoryRootFromTestFile(t)
	legacyFields := map[string]string{
		"RequestDraft":   "use DraftPayload instead",
		"PreviewProduct": "use PreviewPayload or SetPreviewPayload instead",
		"FinalDraft":     "use FinalSubmissionDraft instead",
		"SDSSync":        "use SDSDesignResult instead",
	}
	allowedSelectors := map[string][]string{
		"internal/listingkit/preview_export_semantic_fields.go": {"RequestDraft", "PreviewProduct"},
		"internal/listingkit/revision_apply_shein.go":           {"RequestDraft"},
		"internal/listingkit/semantic_fields.go":                {"SDSSync"},
		"internal/listingkit/service_revision_recompute.go":     {"RequestDraft"},
		"internal/publishing/shein/semantic_fields.go":          {"RequestDraft", "PreviewProduct", "FinalDraft"},
	}

	var violations []string
	for _, relDir := range []string{"internal/listingkit", "internal/publishing/shein"} {
		absDir := filepath.Join(repoRoot, filepath.FromSlash(relDir))
		err := filepath.WalkDir(absDir, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			relPath, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			relPath = filepath.ToSlash(relPath)

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			importAliases := importedPackageAliases(file)
			allowed := allowedSelectors[relPath]

			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				fieldName := selector.Sel.Name
				reason, isLegacyField := legacyFields[fieldName]
				if !isLegacyField {
					return true
				}
				if ident, ok := selector.X.(*ast.Ident); ok && importAliases[ident.Name] {
					return true
				}
				if slices.Contains(allowed, fieldName) {
					return true
				}
				position := fset.Position(selector.Sel.Pos())
				violations = append(violations, position.String()+": legacy semantic field "+fieldName+" is restricted; "+reason)
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", relDir, err)
		}
	}

	if len(violations) > 0 {
		t.Fatalf("legacy semantic field guard failed:\n%s", strings.Join(violations, "\n"))
	}
}

func importedPackageAliases(file *ast.File) map[string]bool {
	aliases := make(map[string]bool, len(file.Imports))
	for _, spec := range file.Imports {
		if spec.Name != nil {
			if spec.Name.Name != "_" && spec.Name.Name != "." {
				aliases[spec.Name.Name] = true
			}
			continue
		}
		pathValue := strings.Trim(spec.Path.Value, "\"")
		if pathValue == "" {
			continue
		}
		aliases[filepath.Base(pathValue)] = true
	}
	return aliases
}

func repositoryRootFromTestFile(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file: runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}
