package image

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProductImageHasOnlyPureCapabilityDependencies(t *testing.T) {
	t.Parallel()

	allowed := map[string]struct{}{
		"context":          {},
		"crypto/sha256":    {},
		"embed":            {},
		"encoding/hex":     {},
		"errors":           {},
		"fmt":              {},
		"image":            {},
		"math":             {},
		"image/color":      {},
		"net":              {},
		"net/url":          {},
		"path":             {},
		"reflect":          {},
		"sort":             {},
		"strconv":          {},
		"strings":          {},
		"sync":             {},
		"gopkg.in/yaml.v3": {},
	}
	violations, err := productImageDependencyViolations(productImageProductionFiles(t), allowed)
	require.NoError(t, err)
	require.Empty(t, violations)
}

func TestProductImageDeclaresNoWorkflowOrPersistenceSemantics(t *testing.T) {
	t.Parallel()

	violations, err := productImageSemanticViolations(productImageProductionFiles(t))
	require.NoError(t, err)
	require.Empty(t, violations)
}

func TestProductImageSemanticGuardCoversEveryDeclarationKind(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join(t.TempDir(), "runtime_semantics.go")
	source := `package fixture
type WorkflowEnvelope struct{}
type Candidate struct { RepositoryClient string }
type RunEnvelope struct{}
type RevisionStore struct{}
func PublishTask() {}
func DispatchJob() {}
func (queueReceiver Candidate) Retry(retryParameter string) (workerResult string) { return "" }
var ApprovalState string
var SchedulerLifecycle string
var DispatcherMode string
const PersistenceMode = "active"
func local() {
	var publisherVariable string
	retryQueue := "scheduled"
	for taskIndex, workflowValue := range []string{"value"} {
		_, _, _, _ = publisherVariable, retryQueue, taskIndex, workflowValue
	}
}
`
	require.NoError(t, os.WriteFile(fixture, []byte(source), 0o600))

	violations, err := productImageSemanticViolations([]string{fixture})
	require.NoError(t, err)
	for _, identifier := range []string{
		"WorkflowEnvelope", "RepositoryClient", "PublishTask", "queueReceiver", "Retry",
		"retryParameter", "workerResult", "ApprovalState", "PersistenceMode", "publisherVariable",
		"retryQueue", "taskIndex", "workflowValue", "RunEnvelope", "RevisionStore", "DispatchJob",
		"SchedulerLifecycle", "DispatcherMode",
	} {
		require.True(t, containsImageSemanticViolation(violations, identifier), "guard missed %s: %v", identifier, violations)
	}
}

func TestProductImageSemanticGuardRequiresEveryForbiddenTermIndependently(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join(t.TempDir(), "independent_runtime_semantics.go")
	source := `package fixture
var TaskOnlyMarker string
var RepositoryOnlyMarker string
var PublisherOnlyMarker string
var QueueOnlyMarker string
var WorkerOnlyMarker string
var WorkflowOnlyMarker string
var RetryOnlyMarker string
var ApprovalOnlyMarker string
var PersistenceOnlyMarker string
var RunOnlyMarker string
var RevisionOnlyMarker string
var JobOnlyMarker string
var StoreOnlyMarker string
var SchedulerOnlyMarker string
var DispatcherOnlyMarker string
var LifecycleOnlyMarker string
`
	require.NoError(t, os.WriteFile(fixture, []byte(source), 0o600))

	markers := map[string]string{
		"task": "TaskOnlyMarker", "repository": "RepositoryOnlyMarker", "publisher": "PublisherOnlyMarker",
		"queue": "QueueOnlyMarker", "worker": "WorkerOnlyMarker", "workflow": "WorkflowOnlyMarker",
		"retry": "RetryOnlyMarker", "approval": "ApprovalOnlyMarker", "persistence": "PersistenceOnlyMarker",
		"run": "RunOnlyMarker", "revision": "RevisionOnlyMarker", "job": "JobOnlyMarker",
		"store": "StoreOnlyMarker", "scheduler": "SchedulerOnlyMarker", "dispatcher": "DispatcherOnlyMarker",
		"lifecycle": "LifecycleOnlyMarker",
	}
	full, err := productImageSemanticViolationsWithTerms([]string{fixture}, forbiddenImageSemanticTerms)
	require.NoError(t, err)
	for term, marker := range markers {
		require.Equal(t, []string{term}, matchingImageSemanticTerms(marker, forbiddenImageSemanticTerms), "%s must match only its own forbidden term", marker)
		require.True(t, containsImageSemanticViolation(full, marker), "full guard missed %s", marker)
	}

	for omitted, omittedMarker := range markers {
		terms := make([]string, 0, len(forbiddenImageSemanticTerms)-1)
		for _, term := range forbiddenImageSemanticTerms {
			if term != omitted {
				terms = append(terms, term)
			}
		}
		mutated, err := productImageSemanticViolationsWithTerms([]string{fixture}, terms)
		require.NoError(t, err)
		require.False(t, containsImageSemanticViolation(mutated, omittedMarker), "omitting %s must expose its independent marker", omitted)
		for term, marker := range markers {
			if term != omitted {
				require.True(t, containsImageSemanticViolation(mutated, marker), "omitting %s unexpectedly disabled %s", omitted, term)
			}
		}
	}
}

func TestValidateProductContextRunsRawResourcePreflightFirst(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(currentFile), "ports.go"), nil, 0)
	require.NoError(t, err)

	var target *ast.FuncDecl
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "validateProductContext" {
			target = function
			break
		}
	}
	require.NotNil(t, target)
	require.NotEmpty(t, target.Body.List)
	first, ok := target.Body.List[0].(*ast.IfStmt)
	require.True(t, ok, "resource preflight must be the first statement")
	assignment, ok := first.Init.(*ast.AssignStmt)
	require.True(t, ok, "first statement must bind the resource preflight error")
	require.Len(t, assignment.Rhs, 1)
	call, ok := assignment.Rhs[0].(*ast.CallExpr)
	require.True(t, ok)
	identifier, ok := call.Fun.(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, "preflightProductContextResources", identifier.Name)
}

func TestProductImageDependencyGuardRejectsRuntimeAndConcreteImageProviders(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join(t.TempDir(), "runtime_imports.go")
	source := `package fixture
import (
	_ "task-processor/internal/app"
	_ "task-processor/internal/integration/openai"
	_ "task-processor/internal/marketplace/imagepolicy"
	_ "task-processor/internal/platform"
	_ "task-processor/internal/productimage"
	_ "github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/sashabaranov/go-openai"
	_ "go.temporal.io/sdk/workflow"
	_ "gorm.io/gorm"
)
`
	require.NoError(t, os.WriteFile(fixture, []byte(source), 0o600))

	violations, err := productImageDependencyViolations([]string{fixture}, map[string]struct{}{})
	require.NoError(t, err)
	require.Len(t, violations, 9)
}

func productImageDependencyViolations(files []string, allowed map[string]struct{}) ([]string, error) {
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

func productImageSemanticViolations(files []string) ([]string, error) {
	return productImageSemanticViolationsWithTerms(files, forbiddenImageSemanticTerms)
}

func productImageSemanticViolationsWithTerms(files, forbidden []string) ([]string, error) {
	var violations []string
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			return nil, err
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			visit := func(identifier *ast.Ident) {
				if identifier != nil && isForbiddenImageSemanticIdentifierWithTerms(identifier.Name, forbidden) {
					violations = append(violations, fmt.Sprintf("%s declares %s", file, identifier.Name))
				}
			}
			switch declaration := node.(type) {
			case *ast.FuncDecl:
				visit(declaration.Name)
			case *ast.TypeSpec:
				visit(declaration.Name)
			case *ast.ValueSpec:
				for _, identifier := range declaration.Names {
					visit(identifier)
				}
			case *ast.Field:
				for _, identifier := range declaration.Names {
					visit(identifier)
				}
			case *ast.AssignStmt:
				if declaration.Tok == token.DEFINE {
					for _, expression := range declaration.Lhs {
						if identifier, ok := expression.(*ast.Ident); ok {
							visit(identifier)
						}
					}
				}
			case *ast.RangeStmt:
				if declaration.Tok == token.DEFINE {
					for _, expression := range []ast.Expr{declaration.Key, declaration.Value} {
						if identifier, ok := expression.(*ast.Ident); ok {
							visit(identifier)
						}
					}
				}
			}
			return true
		})
	}
	sort.Strings(violations)
	return violations, nil
}

var forbiddenImageSemanticTerms = []string{
	"task", "repository", "publisher", "queue", "worker", "workflow", "retry", "approval", "persistence",
	"run", "revision", "job", "store", "scheduler", "dispatcher", "lifecycle",
}

func isForbiddenImageSemanticIdentifierWithTerms(identifier string, forbidden []string) bool {
	lower := strings.ToLower(identifier)
	for _, term := range forbidden {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func matchingImageSemanticTerms(identifier string, forbidden []string) []string {
	lower := strings.ToLower(identifier)
	var matches []string
	for _, term := range forbidden {
		if strings.Contains(lower, term) {
			matches = append(matches, term)
		}
	}
	return matches
}

func containsImageSemanticViolation(violations []string, identifier string) bool {
	for _, violation := range violations {
		if strings.HasSuffix(violation, " "+identifier) {
			return true
		}
	}
	return false
}

func productImageProductionFiles(t *testing.T) []string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	directory := filepath.Dir(file)
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		files = append(files, filepath.Join(directory, entry.Name()))
	}
	sort.Strings(files)
	return files
}
