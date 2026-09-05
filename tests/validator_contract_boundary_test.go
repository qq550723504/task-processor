package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Keep this separate from the documentation tests owned by Issue #311.
func TestValidatorContractImportBoundary(t *testing.T) {
	roots := map[string]map[string]bool{
		"internal/marketplace/validator": {"cmp": true, "encoding/json": true, "slices": true, "strings": true, "time": true, "unicode/utf8": true},
		"internal/marketplace/shein/validator": {
			"crypto/sha256": true,
			"encoding/hex":  true,
			"encoding/json": true,
			"task-processor/internal/marketplace/validator":        true,
			"task-processor/internal/marketplace/shein/publishing": true,
			"task-processor/internal/marketplace/shein/workspace":  true,
			"task-processor/internal/publishing/shein":             true,
		},
	}
	for root, allowed := range roots {
		count := 0
		err := filepath.WalkDir(filepath.Join("..", root), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			count++
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, violation := range validatorBoundaryViolations(string(source), allowed) {
				t.Errorf("%s: %s", path, violation)
			}
			return nil
		})
		if err != nil || count == 0 {
			t.Fatalf("scan %s: files=%d err=%v", root, count, err)
		}
	}
}

func TestValidatorContractGuardRejectsRuntimeAndProviderDependencies(t *testing.T) {
	for _, dependency := range []string{"net/http", "os", "task-processor/internal/agent/runtime", "task-processor/internal/listingkit", "task-processor/internal/compatibility/listingkit", "github.com/openai/openai-go", "task-processor/internal/shein/api/product"} {
		if len(validatorBoundaryViolations("package fixture\nimport "+strconv.Quote(dependency), nil)) == 0 {
			t.Fatalf("accepted %s", dependency)
		}
	}
	for _, source := range []string{"package fixture\nimport clock \"time\"\nfunc f(){clock.Now()}", "package fixture\nimport . \"time\"\nfunc f(){Now()}", "package fixture\nimport t \"time\"\nvar clock = t.Now"} {
		if len(validatorBoundaryViolations(source, map[string]bool{"time": true})) == 0 {
			t.Fatal("accepted implicit wall clock")
		}
	}
}

func validatorBoundaryViolations(source string, allowed map[string]bool) []string {
	file, err := parser.ParseFile(token.NewFileSet(), "validator.go", source, 0)
	if err != nil {
		return []string{err.Error()}
	}
	var violations []string
	timeAlias := "time"
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			violations = append(violations, err.Error())
			continue
		}
		if !allowed[path] {
			violations = append(violations, "forbidden dependency: "+path)
		}
		if spec.Name != nil && (spec.Name.Name == "." || spec.Name.Name == "_") {
			violations = append(violations, "dot/blank imports forbidden")
		}
		if path == "time" && spec.Name != nil {
			timeAlias = spec.Name.Name
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name, ok := selector.X.(*ast.Ident)
		if ok && name.Name == timeAlias {
			switch selector.Sel.Name {
			case "Now", "Since", "Until", "Sleep", "After", "AfterFunc", "Tick", "NewTicker", "NewTimer":
				violations = append(violations, "implicit clock or scheduling forbidden")
			}
		}
		return true
	})
	return violations
}
