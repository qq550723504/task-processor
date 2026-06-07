package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
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
			filepath.Clean(filepath.Join(root, "service_shein_categories.go")):        {},
			filepath.Clean(filepath.Join(root, "service_submit_store_context.go")):    {},
			filepath.Clean(filepath.Join(root, "shein_runtime.go")):                   {},
			filepath.Clean(filepath.Join(root, "service_preview_test.go")):            {},
			filepath.Clean(filepath.Join(root, "service_revision_test.go")):           {},
			filepath.Clean(filepath.Join(root, "service_shein_store_client.go")):      {},
			filepath.Clean(filepath.Join(root, "service_shein_store_client_test.go")): {},
			filepath.Clean(filepath.Join(root, "shein_admin_service.go")):             {},
			filepath.Clean(filepath.Join(root, "httpapi", "bootstrap_test.go")):       {},
		},
		`"task-processor/internal/shein/store"`: {
			filepath.Clean(filepath.Join(root, "shein_submit_payload.go")): {},
		},
		`"task-processor/internal/shein/submitprep"`: {
			filepath.Clean(filepath.Join(root, "shein_submit_test_helpers_test.go")): {},
		},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		for bannedImport, allowedFiles := range allowedImports {
			if _, ok := facts.imports[bannedImport]; !ok {
				continue
			}
			if _, ok := allowedFiles[path]; !ok {
				t.Errorf("%s imports %s; keep non-API SHEIN dependencies isolated to the current allowlisted adapter files", path, bannedImport)
			}
		}
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

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/amazonlisting"`]; !ok {
			continue
		}
		if _, ok := allowedFiles[path]; !ok {
			t.Errorf("%s imports task-processor/internal/amazonlisting; keep amazonlisting dependencies isolated to current facade/result bridge files", path)
		}
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

func TestPublishingSheinNonAPISheinImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "publishing", "shein")
	allowedImports := map[string]map[string]struct{}{
		`"task-processor/internal/shein/client"`: {
			filepath.Clean(filepath.Join(root, "managed_api_factory.go")): {},
			filepath.Clean(filepath.Join(root, "runtime_api_factory.go")): {},
		},
		`"task-processor/internal/shein/content"`: {
			filepath.Clean(filepath.Join(root, "review_content.go")):               {},
			filepath.Clean(filepath.Join(root, "sale_attribute_custom_values.go")): {},
			filepath.Clean(filepath.Join(root, "submit_prep.go")):                  {},
		},
		`"task-processor/internal/shein/category"`: {
			filepath.Clean(filepath.Join(root, "category_query.go")):            {},
			filepath.Clean(filepath.Join(root, "category_resolver_test.go")):    {},
			filepath.Clean(filepath.Join(root, "category_suggest_fallback.go")): {},
			filepath.Clean(filepath.Join(root, "category_tree_fallback.go")):    {},
		},
		`"task-processor/internal/shein/submitprep"`: {
			filepath.Clean(filepath.Join(root, "submit_prep.go")):      {},
			filepath.Clean(filepath.Join(root, "submit_prep_test.go")): {},
		},
		`"task-processor/internal/shein/publish"`: {
			filepath.Clean(filepath.Join(root, "submit_validation.go")): {},
		},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		for bannedImport, allowedFiles := range allowedImports {
			if _, ok := facts.imports[bannedImport]; !ok {
				continue
			}
			if _, ok := allowedFiles[path]; !ok {
				t.Errorf("%s imports %s; keep publishing/shein direct dependencies on legacy shein implementation packages isolated to current allowlisted adapter files", path, bannedImport)
			}
		}
	}
}

func TestPublishingCommonUsesCanonicalPackage(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "publishing", "common"), []string{
		`"task-processor/internal/productenrich"`,
	}, nil)
}

func TestPublishingCommonDoesNotImportPlatformImplementations(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "publishing", "common"), []string{
		`"task-processor/internal/shein"`,
		`"task-processor/internal/temu"`,
		`"task-processor/internal/amazon"`,
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
		"shein_admin_service.go":                {},
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
		"shein_runtime.go":                      {},
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

func TestListingKitRootServiceSubmitFilesStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	allowed := map[string]struct{}{
		"service_submit.go":                     {},
		"service_submit_context_resolver.go":    {},
		"service_submit_default_action.go":      {},
		"service_submit_direct.go":              {},
		"service_submit_runtime_context.go":     {},
		"service_submit_settings_resolution.go": {},
		"service_submit_store_context.go":       {},
		"service_submit_recovery.go":            {},
		"service_submit_temporal_adapter.go":    {},
		"service_submit_wiring.go":              {},
		"service_submit_workflow.go":            {},
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
		if !strings.HasPrefix(name, "service_submit") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, ok := allowed[name]; !ok {
			t.Errorf("%s is a new root-level submit service file; move new submit logic into internal/listingkit/submission or an existing thin facade file instead", filepath.Join(root, name))
		}
	}
}

func TestListingKitRootTaskSubmissionFilesStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	allowed := map[string]struct{}{
		"task_submission_service.go":           {},
		"task_submission_execution_service.go": {},
		"task_submission_recovery_service.go":  {},
		"task_submission_state_service.go":     {},
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
		if !strings.HasPrefix(name, "task_submission") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, ok := allowed[name]; !ok {
			t.Errorf("%s is a new root-level task submission file; move new submission task logic into internal/listingkit/submission or an existing task submission facade file instead", filepath.Join(root, name))
		}
	}
}

func TestListingKitRootServiceGenerationFilesStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	allowed := map[string]struct{}{
		"service_generation.go": {},
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
		if !strings.HasPrefix(name, "service_generation") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, ok := allowed[name]; !ok {
			t.Errorf("%s is a new root-level generation service file; move new generation logic into internal/listingkit/generation or an existing thin facade file instead", filepath.Join(root, name))
		}
	}
}

func TestListingKitRootGenerationFilesStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	allowed := map[string]struct{}{
		"generation_action_keys.go":                   {},
		"generation_action_render_previews.go":        {},
		"generation_conditional_state.go":             {},
		"generation_navigation_dispatch_merge.go":     {},
		"generation_navigation_dispatch_rules.go":     {},
		"generation_navigation_target_conditional.go": {},
		"generation_navigation_target_identity.go":    {},
		"generation_overview.go":                      {},
		"generation_platform_cards.go":                {},
		"generation_queue.go":                         {},
		"generation_queue_list.go":                    {},
		"generation_queue_tasks.go":                   {},
		"generation_recovery_summary.go":              {},
		"generation_render_preview_capabilities.go":   {},
		"generation_resolved_action_summary.go":       {},
		"generation_review_delta.go":                  {},
		"generation_review_focus.go":                  {},
		"generation_review_navigation_target.go":      {},
		"generation_review_patch.go":                  {},
		"generation_review_persistence.go":            {},
		"generation_review_section_config.go":         {},
		"generation_review_session_sections.go":       {},
		"generation_review_state.go":                  {},
		"generation_review_toolbar.go":                {},
		"generation_scene_preset_summary.go":          {},
		"generation_task_list.go":                     {},
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
		if !strings.HasPrefix(name, "generation_") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, ok := allowed[name]; !ok {
			t.Errorf("%s is a new root-level generation file; move new generation logic into internal/listingkit/generation or another narrower subpackage instead", filepath.Join(root, name))
		}
	}
}

func TestListingKitRootSheinWorkspaceBridgesDoNotImportWorkspaceDomainDirectly(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	targetFiles := map[string]struct{}{
		"preview_builder_shein.go":             {},
		"preview_model.go":                     {},
		"final_review_support.go":              {},
		"model_task.go":                        {},
		"revision_apply_result.go":             {},
		"revision_applied_changes.go":          {},
		"revision_diff_preview.go":             {},
		"revision_diff_compare.go":             {},
		"revision_history_compare.go":          {},
		"revision_history_detail.go":           {},
		"revision_history_restore_draft.go":    {},
		"revision_interaction_presentation.go": {},
		"revision_restore_result.go":           {},
		"revision_restore_request.go":          {},
		"revision_success_shared.go":           {},
		"revision_success_payload.go":          {},
		"revision_validation.go":               {},
		"revision_validate_model.go":           {},
		"revision_workspace_bridge.go":         {},
		"shein_submit_readiness.go":            {},
		"shein_workspace_editor_bridge.go":     {},
		"shein_workspace_inspection_bridge.go": {},
		"shein_repair_support.go":              {},
		"shein_repair_center.go":               {},
		"shein_review_state.go":                {},
		"shein_workspace_readiness_support.go": {},
		"shein_build_validation.go":            {},
		"shein_workspace_repair_bridge.go":     {},
		"shein_workspace_revision_bridge.go":   {},
		"shein_workspace_submit_bridge.go":     {},
		"shein_workspace_types_bridge.go":      {},
		"submit_readiness_guidance_shein.go":   {},
		"submit_freshness_shein.go":            {},
		"submit_readiness_projection_shein.go": {},
		"submit_readiness_summary_shein.go":    {},
		"task_state_support.go":                {},
	}
	allowedImports := map[string]map[string]struct{}{
		`"task-processor/internal/workspace/shein"`: {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		name := filepath.Base(path)
		if _, ok := targetFiles[name]; !ok {
			continue
		}
		for bannedImport, allowedFiles := range allowedImports {
			if _, ok := facts.imports[bannedImport]; !ok {
				continue
			}
			if _, ok := allowedFiles[path]; !ok {
				t.Errorf("%s imports %s; keep root shein workspace bridges as thin compatibility wrappers and move direct workspace domain wiring into internal/listingkit/workspace/shein", path, bannedImport)
			}
		}
	}
}

func TestListingKitRootNonTestFilesDoNotImportWorkspaceDomainDirectly(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if filepath.Dir(path) != filepath.Clean(root) {
			continue
		}
		name := filepath.Base(path)
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, ok := facts.imports[`"task-processor/internal/workspace/shein"`]; ok {
			t.Errorf("%s imports task-processor/internal/workspace/shein; keep root ListingKit files on the internal/listingkit/workspace/shein compatibility layer instead", path)
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
		"listingkit_shein_support.go":   {},
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
		`"task-processor/internal/infra/clients/openai"`:   {},
		`"task-processor/internal/listingadmin"`:           {},
		`"task-processor/internal/listingkit"`:             {},
		`"task-processor/internal/listingkit/httpapi"`:     {},
		`"task-processor/internal/listingkit/reviewstore"`: {},
		`"task-processor/internal/listingkit/store"`:       {},
		`"task-processor/internal/listingsubscription"`:    {},
		`"task-processor/internal/publishing/shein"`:       {},
		`"task-processor/internal/sds/usecase"`:            {},
		`"task-processor/internal/sheinlogin"`:             {},
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

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, allowed := allowedFiles[path]; allowed {
			continue
		}
		for _, banned := range bannedImports {
			if _, ok := facts.imports[banned]; ok {
				t.Errorf("%s imports banned boundary package %s", path, banned)
			}
		}
	}
}

func assertNoBannedSelectorsOutside(t *testing.T, root, allowedRoot string, bannedSelectors map[string]struct{}) {
	t.Helper()

	index, err := loadGoFileIndex(root, allowedRoot)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for path, facts := range index.files {
		if !strings.Contains(string(facts.source), "productenrich.") {
			continue
		}
		file, err := parser.ParseFile(fset, path, facts.source, 0)
		if err != nil {
			t.Fatal(err)
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
				t.Errorf("%s uses productenrich.%s compatibility alias; import internal/catalog/canonical directly", path, selector.Sel.Name)
			}
			return true
		})
	}
}
