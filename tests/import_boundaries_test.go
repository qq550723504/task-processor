package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListingKitDoesNotImportLegacySheinRuntime(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "listingkit"), []string{
		`"task-processor/internal/shein/pipeline"`,
		`"task-processor/internal/shein/publish"`,
		`"task-processor/internal/shein/product/build"`,
	}, nil)
}

func TestListingKitDoesNotImportSheinAPIRoot(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "listingkit"), []string{
		`"task-processor/internal/shein/api"`,
	}, nil)
}

func TestListingKitNonAPISheinImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	allowedImports := map[string]map[string]struct{}{
		`"task-processor/internal/shein/client"`: {
			filepath.Clean(filepath.Join(root, "service_shein_categories.go")):     {},
			filepath.Clean(filepath.Join(root, "service_submit_store_context.go")): {},
		},
		`"task-processor/internal/shein/store"`: {
			filepath.Clean(filepath.Join(root, "shein_submit_payload.go")): {},
		},
		`"task-processor/internal/shein/submitprep"`: {
			filepath.Clean(filepath.Join(root, "shein_submit_test_helpers_test.go")): {},
		},
	}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)
		cleanPath := filepath.Clean(path)
		for bannedImport, allowedFiles := range allowedImports {
			if !strings.Contains(text, bannedImport) {
				continue
			}
			if _, ok := allowedFiles[cleanPath]; !ok {
				t.Errorf("%s imports %s; keep non-API SHEIN dependencies isolated to the current allowlisted adapter files", path, bannedImport)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListingKitAmazonListingImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "assembler_test.go")): {},
		filepath.Clean(filepath.Join(root, "export_model.go")):   {},
		filepath.Clean(filepath.Join(root, "interfaces.go")):     {},
		filepath.Clean(filepath.Join(root, "model_result.go")):   {},
		filepath.Clean(filepath.Join(root, "preview_model.go")):  {},
		filepath.Clean(filepath.Join(root, "service.go")):        {},
	}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(content), `"task-processor/internal/amazonlisting"`) {
			return nil
		}
		if _, ok := allowedFiles[filepath.Clean(path)]; !ok {
			t.Errorf("%s imports task-processor/internal/amazonlisting; keep amazonlisting dependencies isolated to current facade/result bridge files", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit(t *testing.T) {
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join("..", "internal", "publishing", "shein", "submit_validation.go")): {},
	}
	assertNoBannedImports(t, filepath.Join("..", "internal", "publishing", "shein"), []string{
		`"task-processor/internal/listingkit"`,
		`"task-processor/internal/productenrich"`,
		`"task-processor/internal/shein/pipeline"`,
		`"task-processor/internal/shein/publish"`,
		`"task-processor/internal/shein/product/build"`,
	}, allowedFiles)
}

func TestPublishingCommonUsesCanonicalPackage(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "publishing", "common"), []string{
		`"task-processor/internal/productenrich"`,
	}, nil)
}

func TestCatalogDoesNotDependOnProductEnrichAliases(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "catalog"), []string{
		`"task-processor/internal/productenrich"`,
	}, nil)
}

func TestListingKitSubdomainsDoNotImportRootFacade(t *testing.T) {
	for _, subdomain := range []string{"generation", "submission", "workflow", "workspace"} {
		t.Run(subdomain, func(t *testing.T) {
			assertNoBannedImports(t, filepath.Join("..", "internal", "listingkit", subdomain), []string{
				`"task-processor/internal/listingkit"`,
			}, nil)
		})
	}
}

func TestListingKitRootSheinHelpersStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	allowed := map[string]struct{}{
		"shein_build_validation.go":             {},
		"shein_color_block_image.go":            {},
		"shein_final_draft.go":                  {},
		"shein_image_regeneration.go":           {},
		"shein_image_regeneration_model.go":     {},
		"shein_image_strategy.go":               {},
		"shein_pricing.go":                      {},
		"shein_repair_center.go":                {},
		"shein_repair_support.go":               {},
		"shein_resolution_cache.go":             {},
		"shein_review_state.go":                 {},
		"shein_size_reference_images.go":        {},
		"shein_settings.go":                     {},
		"shein_studio_ai_product_images.go":     {},
		"shein_studio_image_helpers.go":         {},
		"shein_studio_images.go":                {},
		"shein_studio_size_reference_images.go": {},
		"shein_studio_variant_coverage.go":      {},
		"shein_studio_variant_images.go":        {},
		"shein_submission_events.go":            {},
		"shein_submit_debug.go":                 {},
		"shein_submit_images.go":                {},
		"shein_submit_payload.go":               {},
		"shein_submit_readiness.go":             {},
		"shein_submit_readiness_state.go":       {},
		"shein_submit_retry.go":                 {},
		"shein_submit_sku_normalization.go":     {},
		"shein_submit_state.go":                 {},
		"shein_template_matcher.go":             {},
		"shein_workspace_editor_bridge.go":      {},
		"shein_workspace_inspection_bridge.go":  {},
		"shein_workspace_readiness_support.go":  {},
		"shein_workspace_repair_bridge.go":      {},
		"shein_workspace_revision_bridge.go":    {},
		"shein_workspace_submit_bridge.go":      {},
		"shein_workspace_types_bridge.go":       {},
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "shein_") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, ok := allowed[name]; !ok {
			t.Errorf("%s is a new root-level SHEIN helper; move new domain logic into publishing/shein, workspace/shein, or a listingkit subpackage instead", filepath.Join(root, name))
		}
	}
}

func TestCanonicalTypesDoNotUseProductEnrichCompatibilityAliases(t *testing.T) {
	assertNoBannedSelectorsOutside(t, filepath.Join("..", "internal"), filepath.Join("..", "internal", "productenrich"), map[string]struct{}{
		"CanonicalProduct":           {},
		"CanonicalVariant":           {},
		"CanonicalImage":             {},
		"CanonicalAttribute":         {},
		"CanonicalSource":            {},
		"CanonicalSourceType":        {},
		"FieldTrace":                 {},
		"ProductSpecs":               {},
		"Dimensions":                 {},
		"Weight":                     {},
		"PackageInfo":                {},
		"PriceInfo":                  {},
		"ScrapedVariantDimension":    {},
		"CanonicalSourceUserText":    {},
		"CanonicalSourceUserImage":   {},
		"CanonicalSourceProductURL":  {},
		"CanonicalSourceScrapedData": {},
		"CanonicalSourceLLM":         {},
		"CanonicalSourceDerived":     {},
	})
}

func TestInternalPackagesDoNotImportAppProcessorCompatibilityLayer(t *testing.T) {
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join("..", "internal", "app", "processor", "compat.go")): {},
	}
	assertNoBannedImports(t, filepath.Join("..", "internal"), []string{
		`"task-processor/internal/app/processor"`,
	}, allowedFiles)
}

func TestInternalPackagesDoNotImportAppStateCompatibilityLayer(t *testing.T) {
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join("..", "internal", "app", "state", "compat.go")): {},
	}
	assertNoBannedImports(t, filepath.Join("..", "internal"), []string{
		`"task-processor/internal/app/state"`,
	}, allowedFiles)
}

func TestCmdPackagesDoNotImportAppCompatibilityLayers(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "cmd"), []string{
		`"task-processor/internal/app/processor"`,
		`"task-processor/internal/app/state"`,
	}, nil)
}

func TestDomainHTTPPackagesDoNotImportAppHTTPAPI(t *testing.T) {
	for _, domainRoot := range []string{
		filepath.Join("..", "internal", "productenrich", "httpapi"),
		filepath.Join("..", "internal", "productimage", "httpapi"),
		filepath.Join("..", "internal", "amazonlisting", "httpapi"),
		filepath.Join("..", "internal", "listingkit", "httpapi"),
	} {
		t.Run(filepath.Base(domainRoot), func(t *testing.T) {
			assertNoBannedImports(t, domainRoot, []string{
				`"task-processor/internal/app/httpapi"`,
			}, nil)
		})
	}
}

func TestAppHTTPAPIRootListingKitHelpersStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "httpapi")
	allowed := map[string]struct{}{
		"listingkit_support.go":         {},
		"listingkit_temporal_worker.go": {},
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "listingkit_") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, ok := allowed[name]; !ok {
			t.Errorf("%s is a new app/httpapi ListingKit helper; move ListingKit-specific logic into internal/listingkit/httpapi instead", filepath.Join(root, name))
		}
	}
}

func TestAppHTTPAPIModuleBuildersStayAllowlisted(t *testing.T) {
	filePath := filepath.Join("..", "internal", "app", "httpapi", "modules.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	allowed := map[string]struct{}{
		"buildProductModule":       {},
		"buildImageModule":         {},
		"buildAmazonListingModule": {},
		"buildSheinLoginModule":    {},
		"buildSDSLoginModule":      {},
		"buildListingKitModule":    {},
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		if !strings.HasPrefix(fn.Name.Name, "build") || !strings.HasSuffix(fn.Name.Name, "Module") {
			continue
		}
		if _, ok := allowed[fn.Name.Name]; !ok {
			t.Errorf("%s declares new centralized module builder %s; prefer adding module/bootstrap in the owning domain package and keep app/httpapi as thin assembly", filePath, fn.Name.Name)
		}
	}
}

func TestAppHTTPAPIListingKitSupportImportsStayAllowlisted(t *testing.T) {
	filePath := filepath.Join("..", "internal", "app", "httpapi", "listingkit_support.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}

	allowed := map[string]struct{}{
		`"fmt"`:                                 {},
		`"path/filepath"`:                       {},
		`"strings"`:                             {},
		`"github.com/sirupsen/logrus"`:          {},
		`"gorm.io/gorm"`:                        {},
		`"task-processor/internal/app/runtime"`: {},
		`"task-processor/internal/asset/repository"`:       {},
		`"task-processor/internal/core/config"`:            {},
		`"task-processor/internal/infra/database"`:         {},
		`"task-processor/internal/listingadmin"`:           {},
		`"task-processor/internal/listingkit"`:             {},
		`"task-processor/internal/listingkit/httpapi"`:     {},
		`"task-processor/internal/listingkit/reviewstore"`: {},
		`"task-processor/internal/listingkit/store"`:       {},
		`"task-processor/internal/listingsubscription"`:    {},
		`"task-processor/internal/publishing/shein"`:       {},
		`"task-processor/internal/sds/usecase"`:            {},
		`"task-processor/internal/tenantbridge"`:           {},
	}

	for _, imp := range file.Imports {
		path := imp.Path.Value
		if _, ok := allowed[path]; !ok {
			t.Errorf("%s imports %s; keep app/httpapi/listingkit_support.go limited to assembly and repository wiring, and move new ListingKit-specific logic into internal/listingkit/httpapi or domain packages", filePath, path)
		}
	}
}

func assertNoBannedImports(t *testing.T, root string, bannedImports []string, allowedFiles map[string]struct{}) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		if _, allowed := allowedFiles[filepath.Clean(path)]; allowed {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)
		for _, banned := range bannedImports {
			if strings.Contains(text, banned) {
				t.Errorf("%s imports banned boundary package %s", path, banned)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertNoBannedSelectorsOutside(t *testing.T, root, allowedRoot string, bannedSelectors map[string]struct{}) {
	t.Helper()

	root = filepath.Clean(root)
	allowedRoot = filepath.Clean(allowedRoot)
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		path = filepath.Clean(path)
		if entry.IsDir() {
			if path == allowedRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok || ident.Name != "productenrich" {
				return true
			}
			if _, banned := bannedSelectors[selector.Sel.Name]; banned {
				t.Errorf("%s uses productenrich.%s compatibility alias; import internal/catalog/canonical directly", fset.Position(selector.Pos()), selector.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
