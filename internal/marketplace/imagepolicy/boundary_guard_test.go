package imagepolicy

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProfileInputContainsOnlyStructuredPolicyKey(t *testing.T) {
	t.Parallel()

	typeOfInput := reflect.TypeOf(ProfileInput{})
	require.Equal(t, 4, typeOfInput.NumField())
	for index, name := range []string{"Marketplace", "Country", "Family", "SceneCategory"} {
		field := typeOfInput.Field(index)
		require.Equal(t, name, field.Name)
		require.Equal(t, reflect.TypeOf(""), field.Type)
	}
}

func TestImagePolicyHasOnlyPureResolverDependencies(t *testing.T) {
	t.Parallel()

	allowed := map[string]struct{}{
		"errors":                                {},
		"math":                                  {},
		"task-processor/internal/product/image": {},
	}
	violations, err := imagePolicyDependencyViolations(imagePolicyProductionFiles(t), allowed)
	require.NoError(t, err)
	require.Empty(t, violations)
}

func TestImagePolicyDeclaresNoBuiltinPolicyCatalog(t *testing.T) {
	t.Parallel()

	violations, err := builtinPolicyCatalogViolations(imagePolicyProductionFiles(t))
	require.NoError(t, err)
	require.Empty(t, violations)
}

func TestImagePolicyBoundaryGuardRejectsConfigIOAndBuiltinCatalog(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join(t.TempDir(), "forbidden_policy.go")
	source := `package fixture
import (
	_ "os"
	_ "strings"
	_ "task-processor/internal/core/config"
	_ "task-processor/internal/productimage"
)
type PolicySet struct{}
var builtin = PolicySet{}
`
	require.NoError(t, os.WriteFile(fixture, []byte(source), 0o600))

	imports, err := imagePolicyDependencyViolations([]string{fixture}, map[string]struct{}{})
	require.NoError(t, err)
	require.Len(t, imports, 4)

	catalogs, err := builtinPolicyCatalogViolations([]string{fixture})
	require.NoError(t, err)
	require.Equal(t, []string{fixture + " declares builtin PolicySet"}, catalogs)
}

func imagePolicyProductionFiles(t *testing.T) []string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	files, err := filepath.Glob(filepath.Join(filepath.Dir(currentFile), "*.go"))
	require.NoError(t, err)
	production := files[:0]
	for _, file := range files {
		if !strings.HasSuffix(file, "_test.go") {
			production = append(production, file)
		}
	}
	require.NotEmpty(t, production)
	return production
}

func imagePolicyDependencyViolations(files []string, allowed map[string]struct{}) ([]string, error) {
	var violations []string
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			return nil, fmt.Errorf("parse imports from %s: %w", file, err)
		}
		for _, imported := range parsed.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return nil, err
			}
			if _, ok := allowed[path]; !ok {
				violations = append(violations, fmt.Sprintf("%s imports %s", file, path))
			}
		}
	}
	sort.Strings(violations)
	return violations, nil
}

func builtinPolicyCatalogViolations(files []string) ([]string, error) {
	var violations []string
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			return nil, err
		}
		for _, declaration := range parsed.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR {
				continue
			}
			for _, specification := range general.Specs {
				value, ok := specification.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, expression := range value.Values {
					literal, ok := expression.(*ast.CompositeLit)
					if !ok || !isBuiltinPolicyCatalogType(literal.Type) {
						continue
					}
					violations = append(violations, file+" declares builtin PolicySet")
				}
			}
		}
	}
	sort.Strings(violations)
	return violations, nil
}

func isBuiltinPolicyCatalogType(expression ast.Expr) bool {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name == "PolicySet" || typed.Name == "Policy"
	case *ast.ArrayType:
		identifier, ok := typed.Elt.(*ast.Ident)
		return ok && identifier.Name == "Policy"
	default:
		return false
	}
}
