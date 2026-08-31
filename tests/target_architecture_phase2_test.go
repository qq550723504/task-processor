package tests

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const temporalSDKClientImport = "go.temporal.io/sdk/client"

func temporalSDKDialViolations(root, ownerRoot string) ([]string, error) {
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		aliases := make(map[string]struct{})
		for _, spec := range file.Imports {
			importPath, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("decode import in %s: %w", path, err)
			}
			if importPath != temporalSDKClientImport {
				continue
			}
			if spec.Name != nil && spec.Name.Name == "." {
				violations = append(violations, fmt.Sprintf("%s dot-imports %s", filepath.ToSlash(path), temporalSDKClientImport))
				continue
			}
			alias := "client"
			if spec.Name != nil {
				alias = spec.Name.Name
			}
			if alias != "_" {
				aliases[alias] = struct{}{}
			}
		}
		if pathIsWithin(path, ownerRoot) {
			return nil
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || (selector.Sel.Name != "Dial" && selector.Sel.Name != "DialContext") {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, ok := aliases[qualifier.Name]; ok {
				violations = append(violations, fmt.Sprintf("%s references %s.%s outside internal/platform/temporal", filepath.ToSlash(path), qualifier.Name, selector.Sel.Name))
			}
			return true
		})
		return nil
	})
	return violations, err
}

func pathIsWithin(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func TestProductListingRepositoryBuildersDoNotCallDirectAutoMigrate(t *testing.T) {
	for _, root := range []string{
		filepath.Join("..", "internal", "app", "httpapi"),
		filepath.Join("..", "internal", "app", "bootstrap", "resources"),
		filepath.Join("..", "internal", "productimage", "httpapi"),
		filepath.Join("..", "internal", "productenrich", "httpapi"),
	} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch function := call.Fun.(type) {
				case *ast.SelectorExpr:
					if strings.HasPrefix(function.Sel.Name, "AutoMigrate") {
						t.Errorf("%s contains direct schema mutation call %s", path, function.Sel.Name)
					}
				case *ast.Ident:
					if strings.HasPrefix(function.Name, "AutoMigrate") {
						t.Errorf("%s contains direct schema mutation call %s", path, function.Name)
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", root, err)
		}
	}
}

func TestTemporalSDKDialOwnershipScannerRejectsAliasesAndDotImports(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{
			name: "aliased Dial",
			source: `package fixture
import temporalclient "go.temporal.io/sdk/client"
func connect() { _, _ = temporalclient.Dial(temporalclient.Options{}) }
`,
		},
		{
			name: "aliased DialContext",
			source: `package fixture
import (
	"context"
	temporalclient "go.temporal.io/sdk/client"
)
func connect(ctx context.Context) { _, _ = temporalclient.DialContext(ctx, temporalclient.Options{}) }
`,
		},
		{
			name: "indirect DialContext alias",
			source: `package fixture
import (
	"context"
	temporalclient ` + "`go.temporal.io/sdk/client`" + `
)
func connect(ctx context.Context) {
	dial := temporalclient.DialContext
	_, _ = dial(ctx, temporalclient.Options{})
}
`,
		},
		{
			name: "escaped SDK import",
			source: `package fixture
import temporalclient "go.temporal.io/sdk/\x63lient"
func connect() { _, _ = temporalclient.Dial(temporalclient.Options{}) }
`,
		},
		{
			name: "dot import",
			source: `package fixture
import . "go.temporal.io/sdk/client"
func connect() { _, _ = Dial(Options{}) }
`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "app", "runtime", "fixture.go")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.source), 0o600); err != nil {
				t.Fatal(err)
			}
			violations, err := temporalSDKDialViolations(root, filepath.Join(root, "platform", "temporal"))
			if err != nil {
				t.Fatal(err)
			}
			if len(violations) == 0 {
				t.Fatal("scanner accepted Temporal SDK dial construction outside platform/temporal")
			}
		})
	}
}

func TestTemporalSDKDialOwnershipScannerAllowsPlatformOwner(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "platform", "temporal", "client.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package temporal
import (
	"context"
	sdkclient "go.temporal.io/sdk/client"
)
func connect(ctx context.Context) { _, _ = sdkclient.DialContext(ctx, sdkclient.Options{}) }
`
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	violations, err := temporalSDKDialViolations(root, filepath.Join(root, "platform", "temporal"))
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("allowed platform Temporal dial violations = %v", violations)
	}
}

func TestTemporalSDKDialOwnershipScannerRejectsDotImportInsidePlatformOwner(t *testing.T) {
	root := t.TempDir()
	ownerRoot := filepath.Join(root, "platform", "temporal")
	path := filepath.Join(ownerRoot, "client.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package temporal
import . "go.temporal.io/sdk/client"
func connect() { _, _ = Dial(Options{}) }
`
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	violations, err := temporalSDKDialViolations(root, ownerRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) == 0 {
		t.Fatal("scanner accepted a dot import in the platform Temporal owner")
	}
}

func TestTemporalSDKDialConstructionOwnedByPlatform(t *testing.T) {
	internalRoot := filepath.Join("..", "internal")
	violations, err := temporalSDKDialViolations(internalRoot, filepath.Join(internalRoot, "platform", "temporal"))
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Error(violation)
	}
}

func TestPhase2TemporalPlatformSDKClientAllowlistIsFileScoped(t *testing.T) {
	allowedFiles := temporalPlatformSDKClientAllowedFiles()
	for _, allowed := range []string{"client.go", "client_test.go"} {
		path := filepath.Join("..", "internal", "platform", "temporal", allowed)
		if !pathAllowed(path, allowedFiles) {
			t.Errorf("Temporal SDK client allowlist rejected %s", path)
		}
	}
	for _, rejected := range []string{"worker.go", "workflow.go", "activity.go", "nested/client.go"} {
		path := filepath.Join("..", "internal", "platform", "temporal", filepath.FromSlash(rejected))
		if pathAllowed(path, allowedFiles) {
			t.Errorf("Temporal SDK client allowlist accepted unrelated platform file %s", path)
		}
	}
}

func TestPhase2TargetReadmesMatchApprovedOwnership(t *testing.T) {
	platform := readRepositoryText(t, filepath.Join("..", "internal", "platform", "README.md"))
	for _, forbidden := range []string{"authz", "objectstore"} {
		if strings.Contains(platform, forbidden) {
			t.Errorf("platform README still claims %s ownership", forbidden)
		}
	}
	integration := readRepositoryText(t, filepath.Join("..", "internal", "integration", "README.md"))
	for _, required := range []string{"S3", "ZITADEL", "Casbin", "persistence adapters"} {
		if !strings.Contains(integration, required) {
			t.Errorf("integration README must name %s", required)
		}
	}
}

func productionGoFileCount(t *testing.T, root string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func internalImporterPackageCount(t *testing.T, target string) int {
	t.Helper()
	cmd := exec.Command("go", "list", "-f", `{{.ImportPath}}|{{join .Imports ","}}`, "./internal/...")
	cmd.Dir = ".."
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		_, imports, ok := strings.Cut(strings.TrimSpace(line), "|")
		if !ok {
			continue
		}
		for _, imp := range strings.Split(imports, ",") {
			if importMatchesPrefix(imp, target) {
				count++
				break
			}
		}
	}
	return count
}

func TestPhase2LegacyRootsDoNotGrow(t *testing.T) {
	root := filepath.Join("..", "internal")
	for _, tc := range []struct {
		name string
		max  int
	}{{"core", 58}, {"infra", 68}, {"crawler", 134}} {
		if got := productionGoFileCount(t, filepath.Join(root, tc.name)); got > tc.max {
			t.Errorf("internal/%s production files = %d, baseline max = %d", tc.name, got, tc.max)
		}
	}
	for _, tc := range []struct {
		path string
		max  int
	}{
		{"task-processor/internal/core", 145},
		{"task-processor/internal/infra", 75},
		{"task-processor/internal/core/logger", 84},
	} {
		if got := internalImporterPackageCount(t, tc.path); got > tc.max {
			t.Errorf("%s importer packages = %d, baseline max = %d", tc.path, got, tc.max)
		}
	}
}

func TestTargetDomainsDoNotImportConcreteInfrastructure(t *testing.T) {
	index, err := loadGoFileIndex(filepath.Join("..", "internal"), "")
	if err != nil {
		t.Fatal(err)
	}
	domains := map[string]struct{}{
		"listing": {}, "product": {}, "marketplace": {}, "agent": {}, "knowledge": {},
		"resourcecatalog": {}, "commercial": {}, "ledger": {}, "organization": {},
	}
	for path, facts := range index.files {
		rel, err := filepath.Rel(filepath.Join("..", "internal"), path)
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) == 0 {
			continue
		}
		if _, ok := domains[parts[0]]; !ok {
			continue
		}
		for imp := range facts.imports {
			clean := strings.Trim(imp, `"`)
			if importMatchesPrefix(clean, "task-processor/internal/platform") ||
				importMatchesPrefix(clean, "task-processor/internal/integration") ||
				importMatchesPrefix(clean, "task-processor/internal/infra") ||
				importMatchesPrefix(clean, "task-processor/internal/app") {
				t.Errorf("%s imports concrete infrastructure %s", filepath.ToSlash(rel), imp)
			}
		}
	}
}

func TestPlatformQueueRabbitMQDoesNotImportApplicationOrLegacyInfrastructure(t *testing.T) {
	index, err := loadGoFileIndex(filepath.Join("..", "internal", "platform", "queue", "rabbitmq"), "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		for imp := range facts.imports {
			clean, err := decodeGoImportPath(imp)
			if err != nil {
				t.Fatalf("%s has invalid import %s: %v", filepath.ToSlash(path), imp, err)
			}
			for _, forbidden := range []string{
				"task-processor/internal/app",
				"task-processor/internal/core",
				"task-processor/internal/infra",
			} {
				if importMatchesPrefix(clean, forbidden) {
					t.Errorf("%s imports forbidden package %s", filepath.ToSlash(path), clean)
				}
			}
		}
	}
}

func TestSharedImportPathDecodesGoStringLiterals(t *testing.T) {
	root := t.TempDir()
	source := "package fixture\nimport (\n" +
		"\t\"task-processor/internal/app\"\n" +
		"\t`task-processor/internal/app`\n" +
		"\t\"task-processor/internal/\\x61pp\"\n" +
		")\n"
	if err := os.WriteFile(filepath.Join(root, "fixture.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(index.files) != 1 {
		t.Fatalf("parsed Go files = %d, want 1", len(index.files))
	}

	want := "task-processor/internal/app"
	for _, importLiteral := range []string{
		`"task-processor/internal/app"`,
		"`task-processor/internal/app`",
		`"task-processor/internal/\x61pp"`,
	} {
		for _, facts := range index.files {
			if _, ok := facts.imports[importLiteral]; !ok {
				t.Errorf("parsed imports do not contain literal %q", importLiteral)
				continue
			}
		}
		got, err := decodeGoImportPath(importLiteral)
		if err != nil {
			t.Errorf("decodeGoImportPath(%q): %v", importLiteral, err)
			continue
		}
		if got != want {
			t.Errorf("decodeGoImportPath(%q) = %q, want %q", importLiteral, got, want)
		}
		if !importMatchesPrefix(got, want) {
			t.Errorf("decoded import %q does not match forbidden prefix %q", got, want)
		}
	}
	if _, err := decodeGoImportPath("not-a-go-string-literal"); err == nil {
		t.Error("decodeGoImportPath accepted an invalid Go import literal")
	}
}

func TestSharedPackagesDoNotImportAppDomainPlatformOrIntegration(t *testing.T) {
	index, err := loadGoFileIndex(filepath.Join("..", "internal", "shared"), "")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"task-processor/internal/app",
		"task-processor/internal/listing",
		"task-processor/internal/product",
		"task-processor/internal/marketplace",
		"task-processor/internal/agent",
		"task-processor/internal/knowledge",
		"task-processor/internal/resourcecatalog",
		"task-processor/internal/commercial",
		"task-processor/internal/ledger",
		"task-processor/internal/organization",
		"task-processor/internal/platform",
		"task-processor/internal/integration",
	}
	for path, facts := range index.files {
		rel, err := filepath.Rel(filepath.Join("..", "internal", "shared"), path)
		if err != nil {
			t.Fatal(err)
		}
		for imp := range facts.imports {
			clean, err := decodeGoImportPath(imp)
			if err != nil {
				t.Errorf("%s has invalid Go import literal %q: %v", filepath.ToSlash(rel), imp, err)
				continue
			}
			for _, prefix := range forbidden {
				if importMatchesPrefix(clean, prefix) {
					t.Errorf("%s imports forbidden package %s", filepath.ToSlash(rel), imp)
				}
			}
		}
	}
}

func TestSharedResilienceDoesNotImportInternalPackages(t *testing.T) {
	index, err := loadGoFileIndex(filepath.Join("..", "internal", "shared", "resilience"), "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		for imp := range facts.imports {
			clean, err := decodeGoImportPath(imp)
			if err != nil {
				t.Errorf("%s has invalid Go import literal %q: %v", filepath.ToSlash(path), imp, err)
				continue
			}
			if importMatchesPrefix(clean, "task-processor/internal") {
				t.Errorf("%s imports forbidden internal package %s", filepath.ToSlash(path), clean)
			}
		}
	}
}

func TestSharedPackageTargetsExist(t *testing.T) {
	for _, name := range []string{"hashx", "mathx", "ptr", "resilience", "strx", "timex"} {
		info, err := os.Stat(filepath.Join("..", "internal", "shared", name))
		if err != nil || !info.IsDir() {
			t.Errorf("internal/shared/%s must exist as a directory: %v", name, err)
		}
	}
}
