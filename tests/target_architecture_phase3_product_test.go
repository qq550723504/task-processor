package tests

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPhase3ProductRootContainsNoGoPackage(t *testing.T) {
	entries, err := filepath.Glob(filepath.Join("..", "internal", "product", "*.go"))
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestPhase3LegacyProductRootsAreAbsent(t *testing.T) {
	legacyRoots := []string{"catalog", "asset", "imageasset", "productenrich", "productimage"}
	require.Empty(t, phase3LegacyProductRootViolations(trackedFiles(t, "internal"), legacyRoots), "retired roots must stay absent from Git's tracked production set")
	for _, name := range legacyRoots {
		path := filepath.Join("..", "internal", name)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s still exists; keep the retired product task root deleted", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("stat %s: %v", path, err)
		}
	}
	for _, path := range []string{
		filepath.Join("..", "hack", "debug", "test-analyzeimage"),
		filepath.Join("..", "hack", "debug", "test-productenrich"),
	} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s still exists; keep the retired product debug root deleted", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("stat %s: %v", path, err)
		}
	}
}

func TestPhase3LegacyProductTrackedRootGuardRejectsEveryRetiredRoot(t *testing.T) {
	legacyRoots := []string{"catalog", "asset", "imageasset", "productenrich", "productimage"}
	for _, name := range legacyRoots {
		violations := phase3LegacyProductRootViolations([]string{"internal/" + name + "/reintroduced.go"}, legacyRoots)
		require.Equal(t, []string{"internal/" + name + "/reintroduced.go"}, violations)
	}
}

func phase3LegacyProductRootViolations(files, legacyRoots []string) []string {
	rootSet := make(map[string]struct{}, len(legacyRoots))
	for _, root := range legacyRoots {
		rootSet[root] = struct{}{}
	}
	var violations []string
	for _, path := range files {
		parts := strings.Split(filepath.ToSlash(path), "/")
		if len(parts) < 3 || parts[0] != "internal" {
			continue
		}
		if _, retired := rootSet[parts[1]]; retired {
			violations = append(violations, filepath.ToSlash(path))
		}
	}
	return violations
}

func TestPhase3LegacyProductImportDeclarationsAreAbsent(t *testing.T) {
	banned := []string{
		"task-processor/internal/catalog",
		"task-processor/internal/asset",
		"task-processor/internal/imageasset",
		"task-processor/internal/productenrich",
		"task-processor/internal/productimage",
		"task-processor/internal/product/asset/assettest",
	}
	for _, root := range []string{
		filepath.Join("..", "internal"),
		filepath.Join("..", "cmd"),
		filepath.Join("..", "hack"),
	} {
		assertNoBannedImportPrefixes(t, root, banned, nil)
	}
}

func TestPhase3ConsumersCannotOrchestrateProductImage(t *testing.T) {
	for _, root := range []string{"listingkit", "sds", "amazonlisting"} {
		assertNoBannedImportPrefixes(t, filepath.Join("..", "internal", root), []string{
			"task-processor/internal/product/image",
			"task-processor/internal/imageagent/store",
			"task-processor/internal/imageagent/temporal",
		}, nil)
	}
}

func TestPhase3ProductTargetDependencies(t *testing.T) {
	for _, name := range []string{"catalog", "sourcing", "enrichment", "asset", "image"} {
		root := filepath.Join("..", "internal", "product", name)
		if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			t.Fatal(err)
		}

		assertNoBannedImportPrefixes(t, root, []string{
			"task-processor/internal/app",
			"task-processor/internal/platform",
			"task-processor/internal/integration",
			"gorm.io/gorm",
			"go.temporal.io",
			"github.com/redis",
			"github.com/rabbitmq",
			"github.com/aws/aws-sdk-go-v2",
			"github.com/aws/smithy-go",
			"github.com/sashabaranov/go-openai",
			"github.com/tencentcloud/tencentcloud-sdk-go",
		}, nil)
	}

	assertNoBannedImportPrefixes(t, filepath.Join("..", "internal", "product", "sourcing"), []string{
		"task-processor/internal/asset",
		"task-processor/internal/crawler",
		"task-processor/internal/model",
		"task-processor/internal/product/asset",
		"task-processor/internal/product/enrichment",
		"task-processor/internal/product/image",
		"task-processor/internal/productenrich",
	}, nil)
}

func TestPhase3ProductEnrichmentRuntimeSemanticGuardCoversDeclarationIdentifiers(t *testing.T) {
	fixture := filepath.Join(t.TempDir(), "runtime_semantics.go")
	source := `package fixture
type QueueEnvelope struct{}
type Candidate struct { ProviderClient string }
func DispatchRetry() {}
func (providerReceiver Candidate) SubmitTask(taskParameter string) (retryResult string) { return "" }
var WorkerTaskState string
const ProviderMode = "active"
func local() {
	var queueVariable string
	const taskConstant = "local"
	retryQueue := "scheduled"
	for taskIndex, providerValue := range []string{"value"} {
		_, _, _, _, _ = queueVariable, taskConstant, retryQueue, taskIndex, providerValue
	}
}
`
	require.NoError(t, os.WriteFile(fixture, []byte(source), 0o600))

	violations, err := phase3ProductRuntimeSemanticViolations([]string{fixture})
	require.NoError(t, err)
	for _, identifier := range []string{
		"QueueEnvelope", "ProviderClient", "DispatchRetry", "providerReceiver", "SubmitTask",
		"taskParameter", "retryResult", "WorkerTaskState", "ProviderMode", "queueVariable",
		"taskConstant", "retryQueue", "taskIndex", "providerValue",
	} {
		require.Contains(t, violations, identifier)
	}
}

func TestPhase3ProductEnrichmentHasNoRuntimeSemanticDeclarations(t *testing.T) {
	files, err := phase3ProductProductionFiles(filepath.Join("..", "internal", "product", "enrichment"))
	require.NoError(t, err)

	violations, err := phase3ProductRuntimeSemanticViolations(files)
	require.NoError(t, err)
	require.Empty(t, violations)
}

func phase3ProductRuntimeSemanticViolations(files []string) ([]string, error) {
	violations := []string{}
	for _, path := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return nil, err
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			visit := func(identifier *ast.Ident) {
				if identifier != nil && phase3ProductForbiddenRuntimeIdentifier(identifier.Name) {
					violations = append(violations, identifier.Name)
				}
			}
			visitIdentifiers := func(identifiers []*ast.Ident) {
				for _, identifier := range identifiers {
					visit(identifier)
				}
			}

			switch declaration := node.(type) {
			case *ast.FuncDecl:
				visit(declaration.Name)
			case *ast.TypeSpec:
				visit(declaration.Name)
			case *ast.ValueSpec:
				visitIdentifiers(declaration.Names)
			case *ast.Field:
				visitIdentifiers(declaration.Names)
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
	return violations, nil
}

func phase3ProductForbiddenRuntimeIdentifier(identifier string) bool {
	lower := strings.ToLower(identifier)
	for _, forbidden := range []string{"task", "repository", "retry", "queue", "provider"} {
		if strings.Contains(lower, forbidden) {
			return true
		}
	}
	return false
}

func phase3ProductProductionFiles(root string) ([]string, error) {
	production := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		production = append(production, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return production, nil
}

func TestPhase3PipelineDoesNotGrow(t *testing.T) {
	const baselineMax = 10
	if got := productionGoFileCount(t, filepath.Join("..", "internal", "pipeline")); got > baselineMax {
		t.Errorf("internal/pipeline production files = %d, baseline max = %d", got, baselineMax)
	}
}
