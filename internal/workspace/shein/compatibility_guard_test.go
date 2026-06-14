package shein

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLegacySheinWorkspaceStaysCompatibilityShell(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file: runtime caller unavailable")
	}
	packageDir := filepath.Dir(currentFile)
	allowedImports := map[string]bool{
		"task-processor/internal/marketplace/shein/workspace": true,
		"task-processor/internal/publishing/shein":            true,
	}

	var violations []string
	err := filepath.WalkDir(packageDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			importPath := strings.Trim(spec.Path.Value, "\"")
			if allowedImports[importPath] {
				continue
			}
			position := fset.Position(spec.Pos())
			violations = append(violations, position.String()+": legacy SHEIN workspace compatibility shell may only import marketplace SHEIN workspace or legacy SHEIN publishing models")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan legacy SHEIN workspace imports: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("legacy SHEIN workspace compatibility guard failed:\n%s", strings.Join(violations, "\n"))
	}
}
