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

	productimage "task-processor/internal/product/image"

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

func TestSceneOptionsOwnershipSchemaIsExplicit(t *testing.T) {
	t.Parallel()

	typeOfOptions := reflect.TypeOf(productimage.SceneOptions{})
	expectedNames := []string{
		"SceneCategory", "SceneStyle", "BackgroundTone", "Composition", "PropsLevel",
		"AudienceHint", "CustomSceneHint", "SlotRole", "SlotBrief", "StyleReferenceIDs",
	}
	require.Equal(t, len(expectedNames), typeOfOptions.NumField(), "update validation and ownership when SceneOptions changes")
	for index, name := range expectedNames {
		field := typeOfOptions.Field(index)
		require.Equal(t, name, field.Name)
		if name == "StyleReferenceIDs" {
			require.Equal(t, reflect.TypeOf([]string(nil)), field.Type)
			continue
		}
		require.Equal(t, reflect.TypeOf(""), field.Type)
	}
}

func TestImagePolicyHasOnlyPureResolverDependencies(t *testing.T) {
	t.Parallel()

	allowed := map[string]struct{}{
		"errors":                                {},
		"math":                                  {},
		"strings":                               {},
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

func TestImagePolicyUsesNoHardcodedPolicyDispatch(t *testing.T) {
	t.Parallel()

	files := imagePolicyProductionFiles(t)
	dispatchViolations, err := hardcodedPolicyDispatchViolations(files)
	require.NoError(t, err)
	require.Empty(t, dispatchViolations)

	stringSelectorViolations, err := imagePolicyStringSelectorViolations(files)
	require.NoError(t, err)
	require.Empty(t, stringSelectorViolations)

	literalViolations, err := hardcodedPolicyStringViolations(files)
	require.NoError(t, err)
	require.Empty(t, literalViolations)
}

func TestNewResolverValidatesEntireSetBeforeOwningPolicyData(t *testing.T) {
	t.Parallel()

	var resolverFile string
	for _, file := range imagePolicyProductionFiles(t) {
		if filepath.Base(file) == "resolver.go" {
			resolverFile = file
			break
		}
	}
	require.NotEmpty(t, resolverFile)

	parsed, err := parser.ParseFile(token.NewFileSet(), resolverFile, nil, 0)
	require.NoError(t, err)
	var calls []string
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "NewResolver" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if identifier, ok := call.Fun.(*ast.Ident); ok {
				calls = append(calls, identifier.Name)
			}
			return true
		})
	}
	require.NotContains(t, calls, "make", "NewResolver must not allocate the owned index before validation completes")
	validationIndex := indexOfString(calls, "validatePolicySet")
	buildIndex := indexOfString(calls, "buildResolver")
	require.GreaterOrEqual(t, validationIndex, 0)
	require.Greater(t, buildIndex, validationIndex)
}

func TestImagePolicyBoundaryGuardRejectsConfigIOAndBuiltinCatalog(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join(t.TempDir(), "forbidden_policy.go")
	source := `package fixture
import (
	_ "os"
	str "strings"
	_ "task-processor/internal/core/config"
	_ "task-processor/internal/productimage"
)
type PolicySet struct{}
type Policy struct{}
type PolicyKey struct{}
type Thresholds struct{}
type catalog = map[PolicyKey]Policy
var builtinCategory = "category-a"
var lexemes = []string{"shoe", "bag"}
var builtin = PolicySet{}
var thresholds = map[string]Thresholds{"category-b": {}}
func local(input struct{ Marketplace string }) {
	_ = []Policy{}
	normalize := str.TrimSpace
	_ = normalize(input.Marketplace)
	if input.Marketplace == "marketplace-a" {}
	switch input.Marketplace {}
}
func returned() catalog { return catalog{} }
`
	require.NoError(t, os.WriteFile(fixture, []byte(source), 0o600))
	crossFileFixture := filepath.Join(filepath.Dir(fixture), "cross_file_policy.go")
	require.NoError(t, os.WriteFile(crossFileFixture, []byte("package fixture\nvar crossFile = catalog{}\n"), 0o600))
	fixtureFiles := []string{fixture, crossFileFixture}

	imports, err := imagePolicyDependencyViolations(fixtureFiles, map[string]struct{}{})
	require.NoError(t, err)
	require.Len(t, imports, 4)

	catalogs, err := builtinPolicyCatalogViolations(fixtureFiles)
	require.NoError(t, err)
	require.Len(t, catalogs, 4)

	dispatch, err := hardcodedPolicyDispatchViolations(fixtureFiles)
	require.NoError(t, err)
	require.Len(t, dispatch, 1)

	stringSelectors, err := imagePolicyStringSelectorViolations(fixtureFiles)
	require.NoError(t, err)
	require.Equal(t, []string{fixture + " references forbidden strings.TrimSpace"}, stringSelectors)

	stringLiterals, err := hardcodedPolicyStringViolations(fixtureFiles)
	require.NoError(t, err)
	require.Len(t, stringLiterals, 5)
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
	parsedFiles, err := parseGoFiles(files)
	if err != nil {
		return nil, err
	}
	aliases := policyTypeAliases(parsedFiles)
	var violations []string
	for index, parsed := range parsedFiles {
		ast.Inspect(parsed, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if ok && expressionReferencesPolicyType(literal.Type, aliases) {
				violations = append(violations, files[index]+" declares builtin policy catalog")
			}
			return true
		})
	}
	sort.Strings(violations)
	return violations, nil
}

func policyTypeAliases(files []*ast.File) map[string]struct{} {
	aliases := map[string]struct{}{"Policy": {}, "PolicySet": {}}
	changed := true
	for changed {
		changed = false
		for _, file := range files {
			for _, declaration := range file.Decls {
				general, ok := declaration.(*ast.GenDecl)
				if !ok || general.Tok != token.TYPE {
					continue
				}
				for _, specification := range general.Specs {
					typed, ok := specification.(*ast.TypeSpec)
					if !ok || !expressionReferencesPolicyType(typed.Type, aliases) {
						continue
					}
					if _, exists := aliases[typed.Name.Name]; !exists {
						aliases[typed.Name.Name] = struct{}{}
						changed = true
					}
				}
			}
		}
	}
	return aliases
}

func expressionReferencesPolicyType(expression ast.Expr, aliases map[string]struct{}) bool {
	if expression == nil {
		return false
	}
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok {
			_, found = aliases[identifier.Name]
		}
		return !found
	})
	return found
}

func hardcodedPolicyDispatchViolations(files []string) ([]string, error) {
	var violations []string
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			return nil, err
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			switch node.(type) {
			case *ast.SwitchStmt, *ast.TypeSwitchStmt:
				violations = append(violations, file+" declares hardcoded switch dispatch")
			}
			return true
		})
	}
	sort.Strings(violations)
	return violations, nil
}

func imagePolicyStringSelectorViolations(files []string) ([]string, error) {
	var violations []string
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			return nil, err
		}
		aliases := make(map[string]struct{})
		for _, imported := range parsed.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return nil, err
			}
			if path != "strings" || imported.Name != nil && imported.Name.Name == "_" {
				continue
			}
			if imported.Name != nil && imported.Name.Name == "." {
				violations = append(violations, file+" dot-imports strings")
				continue
			}
			name := "strings"
			if imported.Name != nil {
				name = imported.Name.Name
			}
			aliases[name] = struct{}{}
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, ok := aliases[identifier.Name]; ok && selector.Sel.Name != "Clone" {
				violations = append(violations, fmt.Sprintf("%s references forbidden strings.%s", file, selector.Sel.Name))
			}
			return true
		})
	}
	sort.Strings(violations)
	return violations, nil
}

func hardcodedPolicyStringViolations(files []string) ([]string, error) {
	var violations []string
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			return nil, err
		}
		allowed := make(map[*ast.BasicLit]struct{}, len(parsed.Imports)+3)
		for _, imported := range parsed.Imports {
			allowed[imported.Path] = struct{}{}
		}
		errorAliases := importAliases(parsed, "errors")
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "New" {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, ok := errorAliases[identifier.Name]; !ok {
				return true
			}
			if literal, ok := call.Args[0].(*ast.BasicLit); ok && literal.Kind == token.STRING {
				allowed[literal] = struct{}{}
			}
			return true
		})
		ast.Inspect(parsed, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			if _, ok := allowed[literal]; !ok {
				violations = append(violations, file+" declares hardcoded policy string "+literal.Value)
			}
			return true
		})
	}
	sort.Strings(violations)
	return violations, nil
}

func importAliases(file *ast.File, target string) map[string]struct{} {
	aliases := make(map[string]struct{})
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil || path != target || imported.Name != nil && (imported.Name.Name == "_" || imported.Name.Name == ".") {
			continue
		}
		name := filepath.Base(target)
		if imported.Name != nil {
			name = imported.Name.Name
		}
		aliases[name] = struct{}{}
	}
	return aliases
}

func parseGoFiles(files []string) ([]*ast.File, error) {
	parsed := make([]*ast.File, len(files))
	for index, file := range files {
		var err error
		parsed[index], err = parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			return nil, err
		}
	}
	return parsed, nil
}

func indexOfString(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}
