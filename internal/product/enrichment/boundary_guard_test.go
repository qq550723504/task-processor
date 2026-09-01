package enrichment

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
)

func TestProductEnrichmentRuntimeSemanticGuardCoversEveryDeclarationKind(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join(t.TempDir(), "runtime_semantics.go")
	source := `package fixture
type QueueEnvelope struct{}
type Candidate struct { ProviderClient string }
func DispatchRetry() {}
func (Candidate) SubmitTask() {}
var WorkerTaskState string
const ProviderMode = "active"
func local() {
	retryQueue := "scheduled"
	_ = retryQueue
}
`
	if err := os.WriteFile(fixture, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	violations, err := runtimeSemanticViolations([]string{fixture})
	if err != nil {
		t.Fatal(err)
	}
	for _, identifier := range []string{"QueueEnvelope", "ProviderClient", "DispatchRetry", "SubmitTask", "WorkerTaskState", "ProviderMode", "retryQueue"} {
		if !containsRuntimeSemanticViolation(violations, identifier) {
			t.Errorf("runtime semantic guard missed %s; violations = %v", identifier, violations)
		}
	}
}

func TestProductEnrichmentDependencyGuardRejectsCurrentRuntimeAndProviderSDKs(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join(t.TempDir(), "provider_imports.go")
	source := `package fixture
import (
	_ "github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/rabbitmq/amqp091-go"
	_ "github.com/redis/go-redis/v9"
	_ "github.com/sashabaranov/go-openai"
	_ "go.temporal.io/sdk/workflow"
	_ "gorm.io/gorm"
)
`
	if err := os.WriteFile(fixture, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	violations, err := enrichmentDependencyViolations([]string{fixture}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 6 {
		t.Fatalf("runtime/provider import violations = %v, want all six current dependency roots", violations)
	}
}

func TestProductEnrichmentHasOnlyPureDomainDependencies(t *testing.T) {
	t.Parallel()

	allowedModuleImports := map[string]struct{}{
		"task-processor/internal/product/catalog":  {},
		"task-processor/internal/product/sourcing": {},
	}
	violations, err := enrichmentDependencyViolations(enrichmentProductionFiles(t), allowedModuleImports)
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Error(violation)
	}
}

func TestProductEnrichmentDeclaresNoRuntimeSemantics(t *testing.T) {
	t.Parallel()

	violations, err := runtimeSemanticViolations(enrichmentProductionFiles(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Error(violation)
	}
}

func enrichmentDependencyViolations(files []string, allowedModuleImports map[string]struct{}) ([]string, error) {
	violations := []string{}
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			return nil, fmt.Errorf("parse imports from %s: %w", file, err)
		}
		for _, imported := range parsed.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return nil, fmt.Errorf("unquote import %s: %w", imported.Path.Value, err)
			}
			if !strings.HasPrefix(path, "task-processor/") {
				if isBannedExternalImport(path) {
					violations = append(violations, fmt.Sprintf("product enrichment file %s imports banned runtime/provider dependency %q", file, path))
				}
				continue
			}
			if _, ok := allowedModuleImports[path]; !ok {
				violations = append(violations, fmt.Sprintf("product enrichment file %s imports non-domain dependency %q", file, path))
			}
		}
	}
	sort.Strings(violations)
	return violations, nil
}

func runtimeSemanticViolations(files []string) ([]string, error) {
	violations := []string{}
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse declarations from %s: %w", file, err)
		}
		inspectRuntimeSemanticIdentifiers(parsed, func(identifier string) {
			if isForbiddenRuntimeIdentifier(identifier) {
				violations = append(violations, fmt.Sprintf("product enrichment file %s declares forbidden runtime identifier %s", file, identifier))
			}
		})
	}
	sort.Strings(violations)
	return violations, nil
}

func inspectRuntimeSemanticIdentifiers(file *ast.File, visit func(string)) {
	ast.Inspect(file, func(node ast.Node) bool {
		switch declaration := node.(type) {
		case *ast.FuncDecl:
			visit(declaration.Name.Name)
		case *ast.TypeSpec:
			visit(declaration.Name.Name)
		case *ast.ValueSpec:
			visitIdentifiers(declaration.Names, visit)
		case *ast.StructType:
			for _, field := range declaration.Fields.List {
				visitIdentifiers(field.Names, visit)
			}
		case *ast.InterfaceType:
			for _, method := range declaration.Methods.List {
				visitIdentifiers(method.Names, visit)
			}
		case *ast.AssignStmt:
			if declaration.Tok == token.DEFINE {
				for _, expression := range declaration.Lhs {
					if identifier, ok := expression.(*ast.Ident); ok {
						visit(identifier.Name)
					}
				}
			}
		case *ast.RangeStmt:
			if declaration.Tok == token.DEFINE {
				for _, expression := range []ast.Expr{declaration.Key, declaration.Value} {
					if identifier, ok := expression.(*ast.Ident); ok {
						visit(identifier.Name)
					}
				}
			}
		}
		return true
	})
}

func visitIdentifiers(identifiers []*ast.Ident, visit func(string)) {
	for _, identifier := range identifiers {
		visit(identifier.Name)
	}
}

func isForbiddenRuntimeIdentifier(identifier string) bool {
	lower := strings.ToLower(identifier)
	for _, forbidden := range []string{"task", "repository", "retry", "queue", "provider"} {
		if strings.Contains(lower, forbidden) {
			return true
		}
	}
	return false
}

func containsRuntimeSemanticViolation(violations []string, identifier string) bool {
	for _, violation := range violations {
		if strings.HasSuffix(violation, " "+identifier) {
			return true
		}
	}
	return false
}

func enrichmentProductionFiles(t *testing.T) []string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve product enrichment package directory")
	}
	directory := filepath.Dir(currentFile)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read product enrichment package: %v", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		files = append(files, filepath.Join(directory, entry.Name()))
	}
	return files
}

func isBannedExternalImport(path string) bool {
	for _, prefix := range []string{
		"gorm.io/",
		"go.temporal.io/",
		"github.com/gin-gonic/",
		"github.com/sirupsen/logrus",
		"github.com/sashabaranov/go-openai",
		"github.com/aws/aws-sdk-go-v2",
		"github.com/redis/",
		"github.com/rabbitmq/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(path), "/queue")
}
