package listingkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func readTaskGenerationSourceFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(content)
}

func assertFileAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s should not exist", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
}

func readNamedFunctionSource(t *testing.T, path, funcName string) string {
	t.Helper()
	return readFunctionSourceMatching(t, path, "function "+funcName, func(decl *ast.FuncDecl) bool {
		return decl.Name != nil && decl.Name.Name == funcName
	})
}

func readFunctionSourceMatching(t *testing.T, path, description string, match func(*ast.FuncDecl) bool) string {
	t.Helper()
	source := readTaskGenerationSourceFile(t, path)
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, source, parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", path, err)
	}
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || !match(funcDecl) {
			continue
		}
		start := fileSet.PositionFor(funcDecl.Pos(), false).Offset
		end := fileSet.PositionFor(funcDecl.End(), false).Offset
		if start < 0 || end < start || end > len(source) {
			t.Fatalf("%s should contain valid source offsets for %s", path, description)
		}
		return source[start:end]
	}
	t.Fatalf("%s should contain %s", path, description)
	return ""
}

func assertSourceContainsAll(t *testing.T, source string, required []string) {
	t.Helper()
	for _, needle := range required {
		if !strings.Contains(source, needle) {
			t.Fatalf("source should contain %q", needle)
		}
	}
}

func assertSourceExcludesAll(t *testing.T, source string, forbidden []string) {
	t.Helper()
	for _, needle := range forbidden {
		if strings.Contains(source, needle) {
			t.Fatalf("source should not contain %q", needle)
		}
	}
}

func assertSourceOccurrenceCount(t *testing.T, source, needle string, want int) {
	t.Helper()
	if got := strings.Count(source, needle); got != want {
		t.Fatalf("source should contain %q %d time(s), got %d", needle, want, got)
	}
}

func readNamedFunctionCallNames(t *testing.T, path, funcName string) []string {
	t.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", path, err)
	}
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name == nil || funcDecl.Name.Name != funcName {
			continue
		}
		var names []string
		ast.Inspect(funcDecl.Body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if name := calledFunctionName(call.Fun); name != "" {
					names = append(names, name)
				}
			}
			return true
		})
		return names
	}
	t.Fatalf("%s should contain function %q", path, funcName)
	return nil
}

func calledFunctionName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	default:
		return ""
	}
}

func assertFunctionCallsContainAll(t *testing.T, callNames []string, required []string) {
	t.Helper()
	for _, want := range required {
		found := false
		for _, got := range callNames {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("function calls should contain %q; got %v", want, callNames)
		}
	}
}

func assertFunctionCallsExcludeAll(t *testing.T, callNames []string, forbidden []string) {
	t.Helper()
	for _, want := range forbidden {
		for _, got := range callNames {
			if got == want {
				t.Fatalf("function calls should not contain %q; got %v", want, callNames)
			}
		}
	}
}

func assertFunctionCallsAppearInOrder(t *testing.T, callNames []string, expected []string) {
	t.Helper()
	next := 0
	for _, got := range callNames {
		if next < len(expected) && got == expected[next] {
			next++
		}
	}
	if next != len(expected) {
		t.Fatalf("function calls should contain ordered subsequence %v; got %v", expected, callNames)
	}
}

func readExactMethodSource(t *testing.T, path, signature string) string {
	t.Helper()
	source := readTaskGenerationSourceFile(t, path)
	start := strings.Index(source, signature)
	if start == -1 {
		t.Fatalf("%s should contain method signature %q", path, signature)
	}
	bodyStart := strings.Index(source[start:], "{")
	if bodyStart == -1 {
		t.Fatalf("%s should contain body for signature %q", path, signature)
	}
	bodyStart += start
	depth := 0
	for index := bodyStart; index < len(source); index++ {
		switch source[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[start : index+1]
			}
		}
	}
	t.Fatalf("%s should contain a complete body for signature %q", path, signature)
	return ""
}

func listingKitProductionGoFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.): %v", err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			paths = append(paths, entry.Name())
		}
	}
	return paths
}
