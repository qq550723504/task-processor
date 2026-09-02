package tests

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
		filepath.Join("..", "internal", "amazonlisting", "httpapi"),
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

func TestPhase2MigratedLegacyPackagesStayRetired(t *testing.T) {
	present := presentRetiredPathsInTrackedFiles(phase2RetiredLegacyPaths(), trackedFiles(t, "internal"))
	for _, path := range present {
		t.Errorf("retired path still exists: %s", path)
	}
}

func TestPhase2RetirementGuardDetectsRecreatedPackage(t *testing.T) {
	root := t.TempDir()
	recreated := filepath.Join("internal", "infra", "storage")
	if err := os.MkdirAll(filepath.Join(root, recreated), 0o755); err != nil {
		t.Fatal(err)
	}

	present, err := presentRetiredPaths(root, []string{recreated})
	if err != nil {
		t.Fatal(err)
	}
	if len(present) != 1 || present[0] != recreated {
		t.Fatalf("present retired paths = %v, want [%s]", present, recreated)
	}
}

func TestTrackedRetiredPathsIgnoreUntrackedRuntimeArtifacts(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "internal", "infra", "clients", "openai", "tmp", "logs")
	if err := os.MkdirAll(artifactPath, 0o755); err != nil {
		t.Fatal(err)
	}
	artifactFile, err := os.Create(filepath.Join(artifactPath, "app.log"))
	if err != nil {
		t.Fatal(err)
	}
	if err := artifactFile.Close(); err != nil {
		t.Fatal(err)
	}
	filesystemPresent, err := presentRetiredPaths(root, []string{"internal/infra/clients/openai"})
	if err != nil {
		t.Fatal(err)
	}
	if len(filesystemPresent) != 1 {
		t.Fatalf("filesystem retired paths = %v, want runtime artifact to be visible to fixture scanner", filesystemPresent)
	}

	paths := phase2RetiredLegacyPaths()
	trackedFiles := []string{
		"internal/app/httpapi/runtime.go",
	}

	present := presentRetiredPathsInTrackedFiles(paths, trackedFiles)
	if len(present) != 0 {
		t.Fatalf("present retired paths = %v, want none when only an untracked runtime artifact exists", present)
	}
}

func TestTrackedRetiredPathsDetectTrackedFilesUnderRetiredPackage(t *testing.T) {
	paths := []string{"internal/infra/clients/openai"}
	trackedFiles := []string{"internal/infra/clients/openai/client.go"}

	present := presentRetiredPathsInTrackedFiles(paths, trackedFiles)
	if len(present) != 1 || present[0] != paths[0] {
		t.Fatalf("present retired paths = %v, want [%s]", present, paths[0])
	}
}

func phase2RetiredLegacyPaths() []string {
	return []string{
		"internal/core/lifecycle",
		"internal/infra/database",
		"internal/infra/redisclient",
		"internal/infra/lock",
		"internal/infra/rabbitmq",
		"internal/infra/worker",
		"internal/infra/clients/openai",
		"internal/infra/clients/geminiimage",
		"internal/infra/clients/grsai",
		"internal/infra/storage",
		"internal/infra/resilience",
		"internal/infra/metrics",
		"internal/infra/monitoring",
		"internal/pkg/safeimagehttp",
		"internal/pkg/hashx",
		"internal/pkg/mathx",
		"internal/pkg/ptr",
		"internal/pkg/strx",
		"internal/pkg/timex",
	}
}

func presentRetiredPaths(root string, paths []string) ([]string, error) {
	present := make([]string, 0)
	for _, path := range paths {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		switch {
		case err == nil:
			present = append(present, path)
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return nil, fmt.Errorf("inspect retired path %s: %w", path, err)
		}
	}
	return present, nil
}

func presentRetiredPathsInTrackedFiles(paths, files []string) []string {
	present := make([]string, 0)
	for _, retiredPath := range paths {
		retiredPath = filepath.ToSlash(filepath.Clean(retiredPath))
		prefix := retiredPath + "/"
		for _, file := range files {
			file = filepath.ToSlash(filepath.Clean(file))
			if file == retiredPath || strings.HasPrefix(file, prefix) {
				present = append(present, retiredPath)
				break
			}
		}
	}
	return present
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

type legacyRootCeiling struct {
	name string
	max  int
}

func phase2LegacyRootCeilings() []legacyRootCeiling {
	return []legacyRootCeiling{
		{name: "core", max: 48},
		{name: "infra", max: 16},
		{name: "crawler", max: 134},
	}
}

type importerCeiling struct {
	path string
	max  int
}

func phase2ConcreteImporterCeilings() []importerCeiling {
	return []importerCeiling{
		{path: "task-processor/internal/core", max: 136},
		{path: "task-processor/internal/infra", max: 4},
		{path: "task-processor/internal/core/logger", max: 82},
		{path: "task-processor/internal/platform/logging", max: 9},
		{path: "task-processor/internal/platform/database", max: 21},
		{path: "task-processor/internal/platform/redis", max: 8},
		{path: "task-processor/internal/platform/queue/rabbitmq", max: 18},
		{path: "task-processor/internal/platform/workerpool", max: 23},
		{path: "task-processor/internal/integration/openai", max: 28},
		{path: "task-processor/internal/integration/geminiimage", max: 1},
		{path: "task-processor/internal/integration/grsai", max: 2},
		{path: "task-processor/internal/integration/s3", max: 4},
		{path: "task-processor/internal/integration/httpimage", max: 8},
	}
}

func TestPhase2LegacyRootsDoNotGrow(t *testing.T) {
	root := filepath.Join("..", "internal")
	for _, tc := range phase2LegacyRootCeilings() {
		if got := productionGoFileCount(t, filepath.Join(root, tc.name)); got > tc.max {
			t.Errorf("internal/%s production files = %d, baseline max = %d", tc.name, got, tc.max)
		}
	}
	for _, tc := range phase2ConcreteImporterCeilings() {
		if got := internalImporterPackageCount(t, tc.path); got > tc.max {
			t.Errorf("%s importer packages = %d, baseline max = %d", tc.path, got, tc.max)
		}
	}
}

func TestPhase2ClosureCeilingsRecordFreshInventory(t *testing.T) {
	if got := phase2LegacyRootCeilings(); !reflect.DeepEqual(got, []legacyRootCeiling{
		{name: "core", max: 48},
		{name: "infra", max: 16},
		{name: "crawler", max: 134},
	}) {
		t.Fatalf("legacy root ceilings = %#v", got)
	}
	if got := phase2ConcreteImporterCeilings(); !reflect.DeepEqual(got, []importerCeiling{
		{path: "task-processor/internal/core", max: 136},
		{path: "task-processor/internal/infra", max: 4},
		{path: "task-processor/internal/core/logger", max: 82},
		{path: "task-processor/internal/platform/logging", max: 9},
		{path: "task-processor/internal/platform/database", max: 21},
		{path: "task-processor/internal/platform/redis", max: 8},
		{path: "task-processor/internal/platform/queue/rabbitmq", max: 18},
		{path: "task-processor/internal/platform/workerpool", max: 23},
		{path: "task-processor/internal/integration/openai", max: 28},
		{path: "task-processor/internal/integration/geminiimage", max: 1},
		{path: "task-processor/internal/integration/grsai", max: 2},
		{path: "task-processor/internal/integration/s3", max: 4},
		{path: "task-processor/internal/integration/httpimage", max: 8},
	}) {
		t.Fatalf("concrete importer ceilings = %#v", got)
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

func integrationProviderAdapterImportViolations(root string) ([]string, error) {
	providerRoots := map[string]bool{
		"openai": true, "geminiimage": true, "grsai": true,
	}
	var violations []string
	for _, name := range []string{"openai", "geminiimage", "grsai", "httpimage"} {
		packageRoot := filepath.Join(root, name)
		info, err := os.Stat(packageRoot)
		if err != nil {
			violations = append(violations, fmt.Sprintf("internal/integration/%s must exist: %v", name, err))
			continue
		}
		if !info.IsDir() {
			violations = append(violations, fmt.Sprintf("internal/integration/%s is not a directory", name))
			continue
		}
		index, err := loadGoFileIndex(packageRoot, "")
		if err != nil {
			return nil, err
		}
		for path, facts := range index.files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			for importLiteral := range facts.imports {
				importPath, err := decodeGoImportPath(importLiteral)
				if err != nil {
					return nil, fmt.Errorf("decode import in %s: %w", path, err)
				}
				if !importMatchesPrefix(importPath, "task-processor/internal") {
					continue
				}
				allowed := false
				if providerRoots[name] {
					allowed = importPath == "task-processor/internal/ai" ||
						importMatchesPrefix(importPath, "task-processor/internal/shared") ||
						importPath == "task-processor/internal/integration/httpimage"
				}
				if !allowed {
					violations = append(violations, fmt.Sprintf("%s imports forbidden internal package %s", filepath.ToSlash(path), importPath))
				}
			}
		}
	}
	return violations, nil
}

func TestIntegrationProviderAdaptersUseOnlyContracts(t *testing.T) {
	violations, err := integrationProviderAdapterImportViolations(filepath.Join("..", "internal", "integration"))
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Error(violation)
	}
}

func TestIntegrationProviderBoundaryDecodesGoImportLiterals(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"openai", "geminiimage", "grsai", "httpimage"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	source := "package openai\nimport (\n" +
		"\t`task-processor/internal/app`\n" +
		"\t\"task-processor/internal/\\x63ore\"\n" +
		")\n"
	if err := os.WriteFile(filepath.Join(root, "openai", "fixture.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	violations, err := integrationProviderAdapterImportViolations(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 2 {
		t.Fatalf("decoded forbidden import violations = %v, want 2", violations)
	}
}

func TestLegacyMonitoringPathIsRetired(t *testing.T) {
	if present := presentRetiredPathsInTrackedFiles([]string{"internal/infra/monitoring"}, trackedFiles(t, "internal")); len(present) != 0 {
		t.Errorf("legacy monitoring path must stay retired: %v", present)
	}
	targetRoot := filepath.Join("..", "internal", "app", "monitoring")
	targetIndex, err := loadGoFileIndex(targetRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(targetIndex.files) == 0 {
		t.Fatal("internal/app/monitoring must contain tracked Go files")
	}

	index, err := loadGoFileIndex(filepath.Join("..", "internal"), "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		for importLiteral := range facts.imports {
			importPath, err := decodeGoImportPath(importLiteral)
			if err != nil {
				t.Fatalf("decode import in %s: %v", filepath.ToSlash(path), err)
			}
			if importMatchesPrefix(importPath, "task-processor/internal/infra/monitoring") {
				t.Errorf("%s imports retired monitoring path %s", filepath.ToSlash(path), importPath)
			}
		}
	}
}

func TestPlatformObservabilityDoesNotImportCoreOrApp(t *testing.T) {
	index, err := loadGoFileIndex(filepath.Join("..", "internal", "platform", "observability"), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(index.files) == 0 {
		t.Fatal("internal/platform/observability must contain tracked Go files")
	}
	for path, facts := range index.files {
		for importLiteral := range facts.imports {
			importPath, err := decodeGoImportPath(importLiteral)
			if err != nil {
				t.Fatalf("decode import in %s: %v", filepath.ToSlash(path), err)
			}
			if importMatchesPrefix(importPath, "task-processor/internal/core") ||
				importMatchesPrefix(importPath, "task-processor/internal/app") {
				t.Errorf("%s imports forbidden package %s", filepath.ToSlash(path), importPath)
			}
		}
	}
}
