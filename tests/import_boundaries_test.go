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
)

func TestListingKitDoesNotImportLegacySheinRuntime(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "listingkit"), []string{
		`"task-processor/internal/shein/pipeline"`,
		`"task-processor/internal/shein/publish"`,
		`"task-processor/internal/shein/product/build"`,
	}, nil)
}

func TestPortsManagementAPIPackageIsRetired(t *testing.T) {
	index, err := loadGoFileIndex(filepath.Join("..", "internal"), "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/ports/managementapi"`]; ok {
			t.Fatalf("%s imports retired managementapi compatibility package; use listingadmin/local runtime types instead", path)
		}
	}
}

func TestListingAdminCompatibilityDoesNotExposeTaskStatusAdapters(t *testing.T) {
	path := filepath.Join("..", "internal", "listingadmin", "management_api_types.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(content), "TaskStatusSnapshotFromDTO") {
		t.Fatalf("%s exposes TaskStatusSnapshotFromDTO; keep task status DTO conversion in runtime adapters", path)
	}
	for _, dto := range []string{"TaskStatusRespDTO", "TaskActionRespDTO"} {
		if strings.Contains(string(content), dto) {
			t.Fatalf("%s exposes %s; keep task RPC DTOs in management/taskrpc API adapters", path, dto)
		}
	}
}

func TestListingAdminCompatibilityDoesNotExposeImportTaskUpdateDTO(t *testing.T) {
	path := filepath.Join("..", "internal", "listingadmin", "management_api_types.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(content), "ProductImportTaskUpdateReqDTO") {
		t.Fatalf("%s exposes ProductImportTaskUpdateReqDTO; keep import task status updates on local listingadmin commands", path)
	}
}

func TestListingAdminCompatibilityDoesNotExposeImportTaskResponseDTO(t *testing.T) {
	path := filepath.Join("..", "internal", "listingadmin", "management_api_types.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(content), "ProductImportTaskRespDTO") {
		t.Fatalf("%s exposes ProductImportTaskRespDTO; keep import task reads on local listingadmin models", path)
	}
}

func TestListingRuntimeDebugTaskLoaderUsesLocalTaskModel(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "runtime", "listing", "debug_task_runner.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(content), "ProductImportTaskRespDTO") {
		t.Fatalf("%s uses ProductImportTaskRespDTO; keep debug task loading on local listingadmin/model types", path)
	}
}

func TestAppTaskStatusDTOAdapterIsRetired(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "taskstatusdto")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s still exists; keep management task status DTO conversion in the management adapter", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
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
			filepath.Clean(filepath.Join(root, "service_shein_categories.go")):              {},
			filepath.Clean(filepath.Join(root, "service_shein_category_api_helpers.go")):    {},
			filepath.Clean(filepath.Join(root, "service_submit_remote_context_helpers.go")): {},
			filepath.Clean(filepath.Join(root, "service_submit_store_context.go")):          {},
			filepath.Clean(filepath.Join(root, "shein_runtime.go")):                         {},
			filepath.Clean(filepath.Join(root, "service_preview_test.go")):                  {},
			filepath.Clean(filepath.Join(root, "service_revision_test.go")):                 {},
			filepath.Clean(filepath.Join(root, "service_shein_store_client.go")):            {},
			filepath.Clean(filepath.Join(root, "service_shein_store_client_test.go")):       {},
			filepath.Clean(filepath.Join(root, "shein_admin_service.go")):                   {},
			filepath.Clean(filepath.Join(root, "shein_admin_service_support.go")):           {},
			filepath.Clean(filepath.Join(root, "service_admin_wiring_support.go")):          {},
			filepath.Clean(filepath.Join(root, "httpapi", "bootstrap_test.go")):             {},
			filepath.Clean(filepath.Join(root, "service_submit_context_resolver.go")):       {},
		},
		`"task-processor/internal/shein/store"`: {
			filepath.Clean(filepath.Join(root, "shein_submit_payload.go")):              {},
			filepath.Clean(filepath.Join(root, "shein_submit_payload_site_support.go")): {},
		},
		`"task-processor/internal/shein/submitprep"`: {
			filepath.Clean(filepath.Join(root, "shein_submit_test_helpers_test.go")): {},
		},
		`"task-processor/internal/shein/category"`: {
			filepath.Clean(filepath.Join(root, "httpapi", "shein_category_selector_adapter.go")): {},
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

func TestListingKitRootDoesNotImportManagementAPI(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") || filepath.Dir(path) != filepath.Clean(root) {
			continue
		}
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Errorf("%s imports management/api; keep ListingKit root facade free of concrete management DTO contracts", path)
		}
	}
}

func TestListingKitProductionDoesNotImportMarketplaceSheinPublishing(t *testing.T) {
	assertNoProductionBannedImports(t, filepath.Join("..", "internal", "listingkit"), []string{
		`"task-processor/internal/marketplace/shein/publishing"`,
	}, nil)
}

func TestListingKitSheinSyncLegacyPromotionImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit", "sheinsync")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "promotion_bridge_legacy_adapter.go")): {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") || pathAllowed(path, allowedFiles) {
			continue
		}
		for _, bannedImport := range []string{
			`"task-processor/internal/infra/clients/management/api"`,
			`"task-processor/internal/shein/activity"`,
		} {
			if _, ok := facts.imports[bannedImport]; ok {
				t.Errorf("%s imports %s; keep SHEIN sync legacy promotion DTO/bridge dependencies isolated to promotion_bridge_legacy_adapter.go", path, bannedImport)
			}
		}
	}
}

func TestListingKitAmazonListingImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "assembler_test.go")):                         {},
		filepath.Clean(filepath.Join(root, "export_model.go")):                           {},
		filepath.Clean(filepath.Join(root, "interfaces.go")):                             {},
		filepath.Clean(filepath.Join(root, "interfaces_dependencies.go")):                {},
		filepath.Clean(filepath.Join(root, "model_result.go")):                           {},
		filepath.Clean(filepath.Join(root, "model_result_support.go")):                   {},
		filepath.Clean(filepath.Join(root, "preview_model.go")):                          {},
		filepath.Clean(filepath.Join(root, "platform_payload_input_models.go")):          {},
		filepath.Clean(filepath.Join(root, "platform_payload_models_export.go")):         {},
		filepath.Clean(filepath.Join(root, "platform_payload_models_preview_amazon.go")): {},
		filepath.Clean(filepath.Join(root, "preview_platform_payload_test.go")):          {},
		filepath.Clean(filepath.Join(root, "service.go")):                                {},
		filepath.Clean(filepath.Join(root, "service_defaults.go")):                       {},
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

func TestListingPreviewPackageStaysPlatformNeutral(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "listing", "preview"), []string{
		`"task-processor/internal/listingkit"`,
		`"task-processor/internal/marketplace/amazon"`,
		`"task-processor/internal/marketplace/shein"`,
		`"task-processor/internal/publishing/shein"`,
		`"task-processor/internal/workspace/shein"`,
		`"task-processor/internal/shein"`,
		`"task-processor/internal/temu"`,
		`"task-processor/internal/amazon"`,
		`"task-processor/internal/amazonlisting"`,
	}, nil)
}

func TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "publishing", "shein"), []string{
		`"task-processor/internal/listingkit"`,
		`"task-processor/internal/listingkit/tenantctx"`,
		`"task-processor/internal/productenrich"`,
		`"task-processor/internal/shein/pipeline"`,
		`"task-processor/internal/shein/publish"`,
		`"task-processor/internal/shein/product/build"`,
	}, nil)
}

func TestSheinPipelineDoesNotImportListingKitFacade(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "shein", "pipeline"), []string{
		`"task-processor/internal/listingkit"`,
		`"task-processor/internal/listingkit/tenantctx"`,
	}, nil)
}

func TestSheinSubmitPrepDoesNotImportListingKitTenantContext(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "shein", "submitprep"), []string{
		`"task-processor/internal/listingkit/tenantctx"`,
	}, nil)
}

func TestPublishingSheinNonAPISheinImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "publishing", "shein")
	allowedImports := map[string]map[string]struct{}{
		`"task-processor/internal/shein/client"`: {
			filepath.Clean(filepath.Join(root, "managed_api_factory.go")):        {},
			filepath.Clean(filepath.Join(root, "runtime_base_client_legacy.go")): {},
		},
		`"task-processor/internal/shein/content"`: {},
		`"task-processor/internal/shein/category"`: {
			filepath.Clean(filepath.Join(root, "category_legacy_selector_adapter.go")): {},
		},
		`"task-processor/internal/shein/submitprep"`: {
			filepath.Clean(filepath.Join(root, "submit_prep_sensitive_adapter.go")): {},
		},
		`"task-processor/internal/shein/publish"`: {},
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

	managedRoot := filepath.Join("..", "internal", "publishing", "sheinmanaged")
	managedAllowedImports := map[string]map[string]struct{}{
		`"task-processor/internal/shein/category"`: {
			filepath.Clean(filepath.Join(managedRoot, "category_selector_adapter.go")): {},
		},
		`"task-processor/internal/shein/client"`: {
			filepath.Clean(filepath.Join(managedRoot, "api_factory.go")): {},
		},
		`"task-processor/internal/shein/managedclient"`: {
			filepath.Clean(filepath.Join(managedRoot, "api_factory.go")): {},
		},
	}
	managedIndex, err := loadGoFileIndex(managedRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range managedIndex.files {
		for bannedImport, allowedFiles := range managedAllowedImports {
			if _, ok := facts.imports[bannedImport]; !ok {
				continue
			}
			if _, ok := allowedFiles[path]; !ok {
				t.Errorf("%s imports %s; keep publishing/sheinmanaged direct dependencies on legacy shein category implementation isolated to adapter files", path, bannedImport)
			}
		}
	}
}

func TestPublishingSheinManagedAPIImportsStayAllowlisted(t *testing.T) {
	managedRoot := filepath.Join("..", "internal", "publishing", "sheinmanaged")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(managedRoot, "api_builders.go")):          {},
		filepath.Clean(filepath.Join(managedRoot, "attribute_api_factory.go")): {},
		filepath.Clean(filepath.Join(managedRoot, "category_api_factory.go")):  {},
	}

	index, err := loadGoFileIndex(managedRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") || pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/shein/api") {
				t.Errorf("%s imports %s; keep publishing/sheinmanaged concrete SHEIN API clients isolated to current builder and API factory files", path, importPath)
			}
		}
	}
}

func TestPublishingSheinManagedRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "publishing", "sheinmanaged")
	allowedFiles := map[string]struct{}{}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep publishing/sheinmanaged management dependencies limited to current retirement seams and prefer in-repository database/repository access for new business data", path, importPath)
			}
		}
	}
}

func TestPublishingSheinOpenAIImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "publishing", "shein")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "attribute_resolver_test.go")):                  {},
		filepath.Clean(filepath.Join(root, "category_semantic_verifier_test.go")):          {},
		filepath.Clean(filepath.Join(root, "display_attribute_resolution_flow_test.go")):   {},
		filepath.Clean(filepath.Join(root, "generation_topic_policy_integration_test.go")): {},
		filepath.Clean(filepath.Join(root, "listing_copy_test.go")):                        {},
		filepath.Clean(filepath.Join(root, "review_content_test.go")):                      {},
		filepath.Clean(filepath.Join(root, "sale_attribute_resolver_test.go")):             {},
		filepath.Clean(filepath.Join(root, "sale_attribute_value_matcher_test.go")):        {},
		filepath.Clean(filepath.Join(root, "submit_prep_test.go")):                         {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/openai"`]; ok {
			t.Errorf("%s imports OpenAI adapter directly; keep publishing/shein concrete OpenAI dependencies limited to current inference and runtime resolver seams", path)
		}
	}
}

func TestPublishingSheinSubmitPrepUsesOnlySensitiveWordAdapter(t *testing.T) {
	filePath := filepath.Join("..", "internal", "publishing", "shein", "submit_prep.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	allowedSelectors := map[string]struct{}{
		"CleanSensitiveWordsWithContext":       {},
		"RetrySensitiveWordCleanupWithContext": {},
		"NewSensitiveWordServiceForContext":    {},
		"SetSensitiveWordRepository":           {},
		"SetGenerationTopicPolicyRepository":   {},
		"SetGenerationTopicOverrideRepository": {},
	}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok || ident.Name != "submitprep" {
			return true
		}
		if _, ok := allowedSelectors[selector.Sel.Name]; !ok {
			t.Errorf("%s uses submitprep.%s; keep submitprep dependency limited to sensitive-word repository adapters and move pure submit content helpers into publishing/shein", filePath, selector.Sel.Name)
		}
		return true
	})
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
			dir := filepath.Join("..", "internal", "listingkit", subdomain)
			if _, err := os.Stat(dir); err != nil {
				if os.IsNotExist(err) {
					t.Skipf("listingkit subdomain %s is retired", subdomain)
				}
				t.Fatalf("stat %s: %v", dir, err)
			}
			assertNoBannedImports(t, dir, []string{
				`"task-processor/internal/listingkit"`,
			}, nil)
		})
	}
}

func TestListingKitRootSheinHelpersStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	allowed := map[string]struct{}{
		"shein_build_validation.go":                           {},
		"shein_admin_service.go":                              {},
		"shein_admin_service_support.go":                      {},
		"shein_color_block_image.go":                          {},
		"shein_default_store_resolution.go":                   {},
		"shein_final_draft.go":                                {},
		"shein_final_draft_support.go":                        {},
		"shein_image_regeneration.go":                         {},
		"shein_image_regeneration_model.go":                   {},
		"shein_image_strategy.go":                             {},
		"shein_pricing.go":                                    {},
		"shein_repair_center.go":                              {},
		"shein_repair_clone_support.go":                       {},
		"shein_repair_revision_support.go":                    {},
		"shein_repair_support.go":                             {},
		"shein_repair_validation_support.go":                  {},
		"shein_resolution_cache.go":                           {},
		"shein_review_state.go":                               {},
		"shein_runtime.go":                                    {},
		"shein_sale_attribute_policy.go":                      {},
		"shein_size_reference_images.go":                      {},
		"shein_settings.go":                                   {},
		"shein_store_resolution_presentation.go":              {},
		"shein_studio_ai_product_images.go":                   {},
		"shein_studio_image_helpers.go":                       {},
		"shein_studio_images.go":                              {},
		"shein_studio_size_reference_images.go":               {},
		"shein_studio_variant_coverage.go":                    {},
		"shein_studio_variant_images.go":                      {},
		"shein_submission_events.go":                          {},
		"shein_submit_debug.go":                               {},
		"shein_submit_images.go":                              {},
		"shein_submit_payload.go":                             {},
		"shein_submit_image_upload_support.go":                {},
		"shein_submit_payload_image_support.go":               {},
		"shein_submit_payload_site_support.go":                {},
		"shein_submit_payload_supplier_validation_support.go": {},
		"shein_submit_readiness.go":                           {},
		"shein_submit_readiness_checks_support.go":            {},
		"shein_submit_readiness_guidance_support.go":          {},
		"shein_submit_readiness_status_support.go":            {},
		"shein_submit_readiness_types.go":                     {},
		"shein_submit_readiness_state.go":                     {},
		"shein_submit_retry.go":                               {},
		"shein_submit_sku_normalization.go":                   {},
		"shein_submit_sku_pricing_support.go":                 {},
		"shein_submit_sku_style_support.go":                   {},
		"shein_submit_sku_variant_support.go":                 {},
		"shein_submit_state.go":                               {},
		"shein_template_matcher.go":                           {},
		"shein_workspace_editor_bridge.go":                    {},
		"shein_workspace_inspection_bridge.go":                {},
		"shein_workspace_readiness_support.go":                {},
		"shein_workspace_repair_bridge.go":                    {},
		"shein_workspace_revision_bridge.go":                  {},
		"shein_workspace_submit_bridge.go":                    {},
		"shein_workspace_types_bridge.go":                     {},
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
		"service_submit.go":                                {},
		"service_submit_action_normalization_helper.go":    {},
		"service_submit_collaborators.go":                  {},
		"service_submit_contracts.go":                      {},
		"service_submit_context_resolver.go":               {},
		"service_submit_bindings.go":                       {},
		"service_submit_collaborator_stages.go":            {},
		"service_submit_default_action.go":                 {},
		"service_submit_default_action_resolver_helper.go": {},
		"service_submit_direct.go":                         {},
		"service_submit_entrypoint.go":                     {},
		"service_submit_lease_helper.go":                   {},
		"service_submit_remote_context_helpers.go":         {},
		"service_submit_routing.go":                        {},
		"service_submit_runtime_context.go":                {},
		"service_submit_runtime_context_resolver.go":       {},
		"service_submit_managed_wiring_support.go":         {},
		"service_submit_settings_resolution.go":            {},
		"service_submit_settings_resolution_helpers.go":    {},
		"service_submit_shared.go":                         {},
		"service_submit_store_context.go":                  {},
		"service_submit_task_identity_helper.go":           {},
		"service_submit_recovery.go":                       {},
		"service_submit_temporal_adapter.go":               {},
		"service_submit_temporal_wiring_support.go":        {},
		"service_submit_temporal_task_loader_helper.go":    {},
		"service_submit_warehouse_selection_helper.go":     {},
		"service_submit_wiring.go":                         {},
		"service_submit_wiring_resolution_support.go":      {},
		"service_submit_wiring_support.go":                 {},
		"service_submit_workflow.go":                       {},
		"service_submit_workflow_entry_helpers.go":         {},
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
		"task_submission_service.go":                         {},
		"task_submission_execution_images.go":                {},
		"task_submission_execution_normalize.go":             {},
		"task_submission_execution_product.go":               {},
		"task_submission_execution_remote.go":                {},
		"task_submission_execution_service.go":               {},
		"task_submission_failure_persistence_support.go":     {},
		"task_submission_payload_stage_support.go":           {},
		"task_submission_recovery_lease.go":                  {},
		"task_submission_recovery_remote.go":                 {},
		"task_submission_recovery_service.go":                {},
		"task_submission_recovery_service_remote_support.go": {},
		"task_submission_recovery_service_route_support.go":  {},
		"task_submission_refresh_mutation.go":                {},
		"task_submission_refresh_selection.go":               {},
		"task_submission_refresh_service.go":                 {},
		"task_submission_remote_completion_support.go":       {},
		"task_submission_remote_refresh_support.go":          {},
		"task_submission_remote_support.go":                  {},
		"task_submission_state_service.go":                   {},
		"task_submission_state_persistence_support.go":       {},
		"task_submission_success_persistence_support.go":     {},
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
		"generation_action_keys.go":                          {},
		"generation_action_render_previews.go":               {},
		"generation_conditional_descriptor_support.go":       {},
		"generation_conditional_panel_update_support.go":     {},
		"generation_conditional_state.go":                    {},
		"generation_navigation_dispatch_merge.go":            {},
		"generation_navigation_dispatch_rules.go":            {},
		"generation_navigation_target_conditional.go":        {},
		"generation_navigation_target_dispatch_support.go":   {},
		"generation_navigation_target_identity_support.go":   {},
		"generation_navigation_target_identity.go":           {},
		"generation_overview.go":                             {},
		"generation_overview_action_support.go":              {},
		"generation_overview_filter_support.go":              {},
		"generation_overview_quality_support.go":             {},
		"generation_platform_cards.go":                       {},
		"generation_queue.go":                                {},
		"generation_queue_bundle_support.go":                 {},
		"generation_queue_list.go":                           {},
		"generation_queue_summary_support.go":                {},
		"generation_queue_tasks.go":                          {},
		"generation_recovery_summary_support.go":             {},
		"generation_recovery_summary.go":                     {},
		"generation_render_preview_capabilities.go":          {},
		"generation_resolved_action_summary.go":              {},
		"generation_review_delta.go":                         {},
		"generation_review_focus.go":                         {},
		"generation_review_navigation_target_clone_shape.go": {},
		"generation_review_navigation_target.go":             {},
		"generation_review_patch_support.go":                 {},
		"generation_review_patch.go":                         {},
		"generation_review_persistence.go":                   {},
		"generation_review_section_config.go":                {},
		"generation_review_section_support.go":               {},
		"generation_review_session_sections.go":              {},
		"generation_review_session_slot_support.go":          {},
		"generation_review_state.go":                         {},
		"generation_review_toolbar.go":                       {},
		"generation_scene_preset_summary.go":                 {},
		"generation_task_list.go":                            {},
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

func TestListingKitSheinWorkspaceBridgeDoesNotImportLegacyWorkspaceDomain(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit", "workspace", "shein")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatal(err)
	}
	assertNoBannedImports(t, root, []string{
		`"task-processor/internal/workspace/shein"`,
	}, nil)
}

func TestListingKitHTTPAPIExternalClientImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit", "httpapi")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "ai_credential_store_adapter.go")):           {},
		filepath.Clean(filepath.Join(root, "ai_image_generator_adapter.go")):            {},
		filepath.Clean(filepath.Join(root, "ai_client_builders.go")):                    {},
		filepath.Clean(filepath.Join(root, "ai_client_fallback_helpers.go")):            {},
		filepath.Clean(filepath.Join(root, "ai_client_image_routing.go")):               {},
		filepath.Clean(filepath.Join(root, "ai_client_strict_chat.go")):                 {},
		filepath.Clean(filepath.Join(root, "ai_client_strict_image.go")):                {},
		filepath.Clean(filepath.Join(root, "ai_clients.go")):                            {},
		filepath.Clean(filepath.Join(root, "ai_clients_test.go")):                       {},
		filepath.Clean(filepath.Join(root, "bootstrap_contracts.go")):                   {},
		filepath.Clean(filepath.Join(root, "bootstrap_submit_module.go")):               {},
		filepath.Clean(filepath.Join(root, "bootstrap_test.go")):                        {},
		filepath.Clean(filepath.Join(root, "runtime_support_hooks.go")):                 {},
		filepath.Clean(filepath.Join(root, "shein_category_selector_adapter.go")):       {},
		filepath.Clean(filepath.Join(root, "runtime_support_shein.go")):                 {},
		filepath.Clean(filepath.Join(root, "runtime_support_shein_adapter_helpers.go")): {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for _, bannedImport := range []string{
			`"task-processor/internal/infra/clients/nanobanana"`,
			`"task-processor/internal/infra/clients/openai"`,
		} {
			if _, ok := facts.imports[bannedImport]; ok {
				t.Errorf("%s imports %s; keep listingkit/httpapi concrete external clients limited to current AI runtime and bootstrap seams", path, bannedImport)
			}
		}
	}
}

func TestListingKitHTTPAPIRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit", "httpapi")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "shein_sync_runtime_strategy_helpers.go")): {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep listingkit/httpapi management dependencies limited to current retirement seams and prefer in-repository database/repository access for new business data", path, importPath)
			}
		}
	}
}

func TestAppTaskRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "task")
	allowedFiles := map[string]struct{}{}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep app/task management dependencies limited to current task-source retirement seams and prefer in-repository database/repository access for new task data", path, importPath)
			}
		}
	}
}

func TestAppRunnerRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "runner")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "health_checks.go")):                  {},
		filepath.Clean(filepath.Join(root, "processor_modules_test.go")):         {},
		filepath.Clean(filepath.Join(root, "processor_service.go")):              {},
		filepath.Clean(filepath.Join(root, "processor_service_impl.go")):         {},
		filepath.Clean(filepath.Join(root, "scheduler_dependencies_builder.go")): {},
		filepath.Clean(filepath.Join(root, "scheduler_service.go")):              {},
		filepath.Clean(filepath.Join(root, "scheduler_task_starter.go")):         {},
		filepath.Clean(filepath.Join(root, "scheduler_task_starter_test.go")):    {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep app/runner management dependencies limited to current runtime assembly retirement seams and prefer in-repository database/repository access for new runtime data", path, importPath)
			}
		}
	}
}

func TestAppConsumerRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "consumer")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "auto_shard_coordinator.go")):      {},
		filepath.Clean(filepath.Join(root, "auto_shard_coordinator_test.go")): {},
		filepath.Clean(filepath.Join(root, "listing_runtime_support.go")):     {},
		filepath.Clean(filepath.Join(root, "platform_processor_registry.go")): {},
		filepath.Clean(filepath.Join(root, "processor_registry.go")):          {},
		filepath.Clean(filepath.Join(root, "product_fetcher_builder.go")):     {},
		filepath.Clean(filepath.Join(root, "rabbitmq_service.go")):            {},
		filepath.Clean(filepath.Join(root, "rabbitmq_service_test.go")):       {},
		filepath.Clean(filepath.Join(root, "service_component_state.go")):     {},
		filepath.Clean(filepath.Join(root, "service_manager.go")):             {},
		filepath.Clean(filepath.Join(root, "shared_resources.go")):            {},
		filepath.Clean(filepath.Join(root, "task_handler.go")):                {},
		filepath.Clean(filepath.Join(root, "task_handler_test.go")):           {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep app/consumer management dependencies limited to current consumer runtime retirement seams and prefer in-repository database/repository access for new consumer data", path, importPath)
			}
		}
	}
}

func TestPlatformProcessorRegistryDoesNotExposeRetiredManagementService(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "consumer", "platform_processor_registry.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name == nil || fn.Name.Name != "GetManagementClient" {
			continue
		}
		if len(fn.Recv.List) == 0 {
			continue
		}
		star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		ident, ok := star.X.(*ast.Ident)
		if ok && ident.Name == "PlatformProcessorRegistry" {
			t.Fatalf("%s exposes PlatformProcessorRegistry.GetManagementClient; expose narrower runtime-owned ports instead", path)
		}
	}
}

func TestAppConsumerRuntimeBoundariesDoNotCarryRetiredManagementService(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "consumer", "shared_resources.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name == nil {
				continue
			}
			if typeSpec.Name.Name != "SharedResources" && typeSpec.Name.Name != "PlatformRuntimeContext" {
				continue
			}
			st, ok := typeSpec.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				continue
			}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.Name == "ManagementClient" {
						t.Fatalf("%s exposes %s.ManagementClient; pass narrower runtime-owned ports instead", path, typeSpec.Name.Name)
					}
				}
			}
		}
	}
}

func TestAppConsumerSharedResourcesDoNotCarryListingRuntimeHealthValidator(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "consumer", "shared_resources.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name == nil {
				continue
			}
			if typeSpec.Name.Name == "ListingRuntimeHealthValidator" {
				t.Fatalf("%s defines ListingRuntimeHealthValidator; keep listing runtime health checks owned by app/runtime/listing", path)
			}
			if typeSpec.Name.Name != "SharedResources" {
				continue
			}
			st, ok := typeSpec.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				continue
			}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.Name == "ListingRuntimeHealthValidator" {
						t.Fatalf("%s exposes SharedResources.ListingRuntimeHealthValidator; keep listing runtime health checks out of consumer assembly resources", path)
					}
				}
			}
		}
	}
}

func TestPlatformProcessorRegistryDoesNotStoreRetiredManagementService(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "consumer", "platform_processor_registry.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name == nil || typeSpec.Name.Name != "PlatformProcessorRegistry" {
				continue
			}
			st, ok := typeSpec.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				continue
			}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.Name == "managementClient" {
						t.Fatalf("%s stores PlatformProcessorRegistry.managementClient; store narrower runtime-owned ports instead", path)
					}
					if name.Name == "listingRuntimeHealthValidator" {
						t.Fatalf("%s stores PlatformProcessorRegistry.listingRuntimeHealthValidator; let listing runtime consume the initialized health validator directly", path)
					}
				}
			}
		}
	}
}

func TestPlatformProcessorRegistryDoesNotFanOutSharedResources(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "consumer", "platform_processor_registry.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	bannedFields := map[string]struct{}{
		"listingRuntimeImportTaskRepository": {},
		"rawJSONDataClient":                  {},
		"storeAPI":                           {},
		"schedulerRuntime":                   {},
		"schedulerFactoryRuntime":            {},
		"processorRuntime":                   {},
		"sharedCrawlSource":                  {},
		"sharedProductFetcher":               {},
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name == nil || typeSpec.Name.Name != "PlatformProcessorRegistry" {
				continue
			}
			st, ok := typeSpec.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				continue
			}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if _, banned := bannedFields[name.Name]; banned {
						t.Fatalf("%s stores PlatformProcessorRegistry.%s; keep initialized runtime resources bundled as SharedResources", path, name.Name)
					}
				}
			}
		}
	}
}

func TestPlatformProcessorRegistryDoesNotStoreSharedResources(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "consumer", "platform_processor_registry.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, phrase := range []string{
		"sharedResources  *SharedResources",
		"r.sharedResources",
		"runtimeResources",
		"useSharedResources",
		"GetSharedCrawlSource",
		"GetSharedProductFetcher",
	} {
		if strings.Contains(string(content), phrase) {
			t.Fatalf("%s mentions %q; pass SharedResources explicitly instead of storing runtime resources on the registry", path, phrase)
		}
	}
}

func TestPlatformProcessorRegistryDoesNotOwnSharedResourceProvider(t *testing.T) {
	paths := []string{
		filepath.Join("..", "internal", "app", "consumer", "dependencies.go"),
		filepath.Join("..", "internal", "app", "consumer", "platform_processor_registry.go"),
		filepath.Join("..", "internal", "app", "consumer", "shared_resources.go"),
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, phrase := range []string{
			"SharedResourceProvider",
			"sharedResourceProvider",
			"initializeSharedResources",
		} {
			if strings.Contains(string(content), phrase) {
				t.Fatalf("%s mentions %q; build shared resources in bootstrap/runtime assembly and inject the resource bundle into the registry", path, phrase)
			}
		}
	}
}

func TestPlatformProcessorRegistryDoesNotExposeListingRuntimeHealthValidator(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "consumer", "platform_processor_registry.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name == nil || fn.Name.Name != "GetListingRuntimeHealthValidator" {
			continue
		}
		if len(fn.Recv.List) == 0 {
			continue
		}
		star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		ident, ok := star.X.(*ast.Ident)
		if ok && ident.Name == "PlatformProcessorRegistry" {
			t.Fatalf("%s exposes PlatformProcessorRegistry.GetListingRuntimeHealthValidator; listing runtime should consume the initialized health validator without a registry accessor", path)
		}
	}
}

func TestAppConsumerTaskStatusRuntimeProviderIsNotNamedRetiredManagementService(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "consumer")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		file, err := parser.ParseFile(token.NewFileSet(), path, facts.source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if ok && typeSpec.Name.Name == "managementClientProvider" {
					t.Fatalf("%s defines managementClientProvider; name task status runtime ports after the capability they expose", path)
				}
			}
		}
	}
}

func TestAppConsumerDoesNotUseManagementNamedTaskStatusAdapter(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "consumer")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		file, err := parser.ParseFile(token.NewFileSet(), path, facts.source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel == nil || selector.Sel.Name != "NewManagementClientAdapter" {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if ok && ident.Name == "taskstatus" {
				t.Fatalf("%s calls taskstatus.NewManagementClientAdapter; use a runtime task status adapter in app/consumer", path)
			}
			return true
		})
	}
}

func TestAppTaskPollingSourceUsesCapabilityNames(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("..", "internal", "app", "task", "task_source.go"): {
			"management client is not initialized",
			"s.fetcher.managementClient",
		},
		filepath.Join("..", "internal", "app", "task", "dispatcher.go"): {
			"管理客户端为空，跳过任务获取",
		},
		filepath.Join("..", "internal", "app", "task", "fetcher_utils.go"): {
			"fetches candidate tasks from management",
			"return f.managementClient",
		},
	}

	for path, phrases := range checks {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, phrase := range phrases {
			if strings.Contains(string(content), phrase) {
				t.Fatalf("%s mentions %q; use pending task source names for legacy polling dependencies", path, phrase)
			}
		}
	}
}

func TestAppTaskFetcherDoesNotStoreRetiredManagementService(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "task", "fetcher.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name == nil || typeSpec.Name.Name != "TaskFetcher" {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok || structType.Fields == nil {
				continue
			}
			for _, field := range structType.Fields.List {
				for _, name := range field.Names {
					if name.Name == "managementClient" {
						t.Fatalf("%s stores managementClient on TaskFetcher; store narrower task runtime capabilities instead", path)
					}
				}
			}
		}
	}
}

func TestAppTaskInterfacesDoNotExposeLegacyClientProviders(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "task", "interfaces.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	legacyTypes := map[string]struct{}{
		"ManagementClientProvider": {},
		"ImportTaskClient":         {},
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name == nil {
				continue
			}
			if _, ok := legacyTypes[typeSpec.Name.Name]; ok {
				t.Fatalf("%s exposes %s; inject narrower task runtime capabilities instead", path, typeSpec.Name.Name)
			}
		}
	}
}

func TestAppTaskDispatchGuardUsesCapabilityNames(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "task", "task_dispatch_guard.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, phrase := range []string{
		"fetcher.managementClient",
		"GetDailyListingCountClient",
	} {
		if strings.Contains(string(content), phrase) {
			t.Fatalf("%s mentions %q; inject dispatch guard runtime capabilities instead", path, phrase)
		}
	}
}

func TestAppTaskDispatcherUsesCapabilityNames(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "task", "dispatcher.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(content), "managementClient.GetRuntimeStoreService") {
		t.Fatalf("%s reaches into managementClient for store dispatch; inject store dispatch runtime capability instead", path)
	}
}

func TestAppTaskStatusUpdatesUseCapabilityNames(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "task", "fetcher_utils.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, phrase := range []string{
		"f == nil || f.managementClient == nil",
		"f.managementClient.UpdateRuntimeTaskStatus",
	} {
		if strings.Contains(string(content), phrase) {
			t.Fatalf("%s mentions %q; use runtime task status updater capability instead", path, phrase)
		}
	}
}

func TestTaskStatusAdapterCallersUseRuntimeNamedConstructor(t *testing.T) {
	root := filepath.Join("..", "internal")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "taskstatus", "service.go")):        {},
		filepath.Clean(filepath.Join(root, "app", "taskstatus", "service.go")): {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, facts.source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel == nil || selector.Sel.Name != "NewManagementClientAdapter" {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if ok && ident.Name == "taskstatus" {
				t.Fatalf("%s calls taskstatus.NewManagementClientAdapter; use NewRuntimeTaskStatusAdapter outside compatibility wrappers", path)
			}
			return true
		})
	}
}

func TestTaskStatusPackageDoesNotExposeManagementNamedAdapter(t *testing.T) {
	path := filepath.Join("..", "internal", "taskstatus", "service.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name != nil && fn.Name.Name == "NewManagementClientAdapter" {
			t.Fatalf("%s exposes NewManagementClientAdapter; expose runtime-named task status adapters instead", path)
		}
	}
}

func TestTaskStatusPackageDoesNotExposeBroadManagementRuntimeConstructor(t *testing.T) {
	path := filepath.Join("..", "internal", "taskstatus", "service.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name != nil && fn.Name.Name == "NewManagementRuntime" {
			t.Fatalf("%s exposes NewManagementRuntime; use task-status-specific runtime constructor names", path)
		}
	}
}

func TestTaskStatusRuntimeErrorsUseCapabilityNames(t *testing.T) {
	path := filepath.Join("..", "internal", "taskstatus", "service.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(content), "management client is not initialized") {
		t.Fatalf("%s mentions management client initialization; use task status runtime names for task status dependencies", path)
	}
}

func TestTaskStatusPackageDoesNotImportRetiredManagementPackage(t *testing.T) {
	root := filepath.Join("..", "internal", "taskstatus")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Fatalf("%s imports %s; keep taskstatus package on task-status runtime ports", path, importPath)
			}
		}
	}
}

func TestBaseProcessorDoesNotExposeRetiredManagementService(t *testing.T) {
	path := filepath.Join("..", "internal", "processor", "base_processor.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
			t.Fatalf("%s imports %s; BaseProcessor must only expose explicitly injected runtime ports after retired management service shutdown", path, importPath)
		}
	}

	for _, decl := range file.Decls {
		switch typedDecl := decl.(type) {
		case *ast.FuncDecl:
			if typedDecl.Recv != nil && typedDecl.Name != nil && typedDecl.Name.Name == "GetManagementClient" {
				t.Fatalf("%s exposes BaseProcessor.GetManagementClient; expose narrower runtime-owned ports instead", path)
			}
		case *ast.GenDecl:
			for _, spec := range typedDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name == nil {
					continue
				}
				if typeSpec.Name.Name != "BaseProcessor" && typeSpec.Name.Name != "BaseProcessorConfig" {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range structType.Fields.List {
					for _, name := range field.Names {
						if strings.EqualFold(name.Name, "managementClient") {
							t.Fatalf("%s %s exposes %s; inject StoreAPI/TaskStatusRuntime/DailyCountClientProvider ports instead", path, typeSpec.Name.Name, name.Name)
						}
					}
				}
			}
		}
	}

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(source), "management.New") {
		t.Fatalf("%s constructs management clients; BaseProcessor must not revive retired management service", path)
	}
}

func TestAppAssemblyUsesManagementAPIPortAliases(t *testing.T) {
	paths := []string{
		filepath.Join("..", "internal", "app", "runner", "processor_service.go"),
		filepath.Join("..", "internal", "app", "bootstrap", "resources", "shared_resources.go"),
		filepath.Join("..", "internal", "app", "consumer", "shared_resources.go"),
		filepath.Join("..", "internal", "app", "consumer", "platform_processor_registry.go"),
		filepath.Join("..", "internal", "app", "consumer", "auto_shard_coordinator.go"),
		filepath.Join("..", "internal", "app", "consumer", "listing_runtime_support.go"),
		filepath.Join("..", "internal", "app", "consumer", "processor_registry.go"),
		filepath.Join("..", "internal", "app", "consumer", "rabbitmq_service.go"),
		filepath.Join("..", "internal", "app", "consumer", "service_component_state.go"),
		filepath.Join("..", "internal", "app", "consumer", "service_manager.go"),
		filepath.Join("..", "internal", "app", "consumer", "task_handler.go"),
		filepath.Join("..", "internal", "app", "consumer", "auto_shard_coordinator_test.go"),
		filepath.Join("..", "internal", "app", "consumer", "rabbitmq_service_test.go"),
		filepath.Join("..", "internal", "app", "consumer", "task_handler_test.go"),
		filepath.Join("..", "internal", "app", "runtime", "listing", "debug_task_runner.go"),
		filepath.Join("..", "internal", "app", "runtime", "listing", "debug_task_runner_test.go"),
	}
	for _, path := range paths {
		path = filepath.Clean(path)
		index, err := loadGoFileIndex(filepath.Dir(path), "")
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		facts, ok := index.files[path]
		if !ok {
			t.Fatalf("load %s: file facts missing", path)
		}
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types in assembly interfaces", path)
		}
	}
}

func TestListingAdminUsesManagementAPIPortAliases(t *testing.T) {
	root := filepath.Join("..", "internal", "listingadmin")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "management_compat.go")): {},
	}
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types for compatibility DTOs", path)
		}
	}
}

func TestStateAndTaskRPCUseManagementAPIPortAliases(t *testing.T) {
	paths := []string{
		filepath.Join("..", "internal", "state", "daily_count_manager.go"),
		filepath.Join("..", "internal", "taskrpcapi", "handler.go"),
	}
	for _, path := range paths {
		path = filepath.Clean(path)
		index, err := loadGoFileIndex(filepath.Dir(path), "")
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		facts, ok := index.files[path]
		if !ok {
			t.Fatalf("load %s: file facts missing", path)
		}
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestAmazonTaskStatusUpdatesUseTaskStatusRuntime(t *testing.T) {
	path := filepath.Join("..", "internal", "amazon", "task_status.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != "updateTaskStatusSyncWithInput" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel == nil || selector.Sel.Name != "GetManagementClient" {
				return true
			}
			t.Fatalf("%s updateTaskStatusSyncWithInput calls GetManagementClient; use GetTaskStatusRuntime for task status updates", path)
			return true
		})
	}
}

func TestAmazonAuthPauseUsesStoreAPIPort(t *testing.T) {
	path := filepath.Join("..", "internal", "amazon", "task_status.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != "pauseStoreForAuthentication" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel == nil || selector.Sel.Name != "GetManagementClient" {
				return true
			}
			t.Fatalf("%s pauseStoreForAuthentication calls GetManagementClient; use a store API port for auth pause updates", path)
			return true
		})
	}
}

func TestAmazonServicesUseStoreAPIPort(t *testing.T) {
	path := filepath.Join("..", "internal", "amazon", "model", "context.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name == nil || typeSpec.Name.Name != "Services" {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok || structType.Fields == nil {
					continue
				}
				for _, field := range structType.Fields.List {
					for _, name := range field.Names {
						if name.Name == "ManagementClient" {
							t.Fatalf("%s Services exposes ManagementClient; expose StoreAPI for store reads and pause updates", path)
						}
					}
				}
			}
		case *ast.FuncDecl:
			if typed.Name != nil && typed.Name.Name == "SetManagementClient" {
				t.Fatalf("%s exposes SetManagementClient; use SetStoreAPI so Amazon services depend on the store port", path)
			}
		}
	}
}

func TestAmazonUsesManagementAPIPortAliases(t *testing.T) {
	root := filepath.Join("..", "internal", "amazon")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestTemuPricingRuntimeUsesCapabilityNames(t *testing.T) {
	path := filepath.Join("..", "internal", "temu", "pricing", "runtime_adapter.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name == nil {
					continue
				}
				if typeSpec.Name.Name == "ManagementRuntime" {
					t.Fatalf("%s exposes ManagementRuntime; use pricing runtime names for TEMU pricing ports", path)
				}
			}
		case *ast.FuncDecl:
			if typed.Name != nil && typed.Name.Name == "NewManagementRuntime" {
				t.Fatalf("%s exposes NewManagementRuntime; use NewPricingRuntime for TEMU pricing ports", path)
			}
		}
	}
}

func TestTemuSchedulerRuntimeUsesCapabilityNames(t *testing.T) {
	path := filepath.Join("..", "internal", "temu", "scheduler", "runtime_adapter.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name == nil {
					continue
				}
				if typeSpec.Name.Name == "ManagementRuntime" || typeSpec.Name.Name == "managementRuntime" {
					t.Fatalf("%s exposes ManagementRuntime; use scheduler runtime names for TEMU scheduler ports", path)
				}
			}
		case *ast.FuncDecl:
			if typed.Name != nil && typed.Name.Name == "NewManagementRuntime" {
				t.Fatalf("%s exposes NewManagementRuntime; use NewSchedulerRuntime for TEMU scheduler ports", path)
			}
		}
	}
}

func TestTemuSyncRuntimeUsesCapabilityNames(t *testing.T) {
	path := filepath.Join("..", "internal", "temu", "sync", "runtime_adapter.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name == nil {
				continue
			}
			if typeSpec.Name.Name == "managementRuntime" {
				t.Fatalf("%s defines managementRuntime; use service runtime names for TEMU sync ports", path)
			}
		}
	}
}

func TestTemuRuntimeErrorsUseCapabilityNames(t *testing.T) {
	paths := []string{
		filepath.Join("..", "internal", "temu", "api", "client", "auth_pause_handler.go"),
		filepath.Join("..", "internal", "temu", "api", "client", "client.go"),
		filepath.Join("..", "internal", "temu", "api", "client", "cookie_manager.go"),
		filepath.Join("..", "internal", "temu", "pricing", "pricing_data_service.go"),
		filepath.Join("..", "internal", "temu", "pricing", "store_config_service.go"),
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, phrase := range []string{"管理客户端未初始化", "管理客户端为空", "管理系统客户端为空"} {
			if strings.Contains(string(content), phrase) {
				t.Fatalf("%s mentions %q; use store/pricing/service runtime names for TEMU runtime dependencies", path, phrase)
			}
		}
	}
}

func TestTemuContextUsesManagementAPIPortAliases(t *testing.T) {
	root := filepath.Join("..", "internal", "temu", "context")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestTemuRootUsesManagementAPIPortAliases(t *testing.T) {
	paths := []string{
		filepath.Join("..", "internal", "temu", "processor.go"),
		filepath.Join("..", "internal", "temu", "pipeline_registry.go"),
	}
	for _, path := range paths {
		path = filepath.Clean(path)
		index, err := loadGoFileIndex(filepath.Dir(path), "")
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		facts, ok := index.files[path]
		if !ok {
			t.Fatalf("load %s: file facts missing", path)
		}
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestTemuFilterUsesManagementAPIPortAliases(t *testing.T) {
	root := filepath.Join("..", "internal", "temu", "filter")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestTemuRulesUsesManagementAPIPortAliases(t *testing.T) {
	root := filepath.Join("..", "internal", "temu", "rules")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestTemuHandlerBaseUsesManagementAPIPortAliases(t *testing.T) {
	root := filepath.Join("..", "internal", "temu", "handlerbase")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestTemuStoreUsesManagementAPIPortAliases(t *testing.T) {
	root := filepath.Join("..", "internal", "temu", "store")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestTemuSkuUsesManagementAPIPortAliases(t *testing.T) {
	root := filepath.Join("..", "internal", "temu", "sku")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestTemuProductUsesManagementAPIPortAliases(t *testing.T) {
	root := filepath.Join("..", "internal", "temu", "product")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestTemuPricingUsesManagementAPIPortAliases(t *testing.T) {
	root := filepath.Join("..", "internal", "temu", "pricing")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestTemuSyncUsesManagementAPIPortAliases(t *testing.T) {
	root := filepath.Join("..", "internal", "temu", "sync")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestTemuAPIClientUsesManagementAPIPortAliases(t *testing.T) {
	root := filepath.Join("..", "internal", "temu", "api", "client")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
			t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
		}
	}
}

func TestTemuPricingFallbackLogsUseCapabilityNames(t *testing.T) {
	paths := []string{
		filepath.Join("..", "internal", "temu", "pricing", "pricing_data_service.go"),
		filepath.Join("..", "internal", "temu", "pricing", "store_config_service.go"),
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(content), "management client") {
			t.Fatalf("%s mentions management client in fallback logs; use pricing/store capability names", path)
		}
	}
}

func TestTemuSyncFallbackLogsUseCapabilityNames(t *testing.T) {
	path := filepath.Join("..", "internal", "temu", "sync", "inventory_sync_helper.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(content), "management client") {
		t.Fatalf("%s mentions management client in fallback logs; use inventory/store capability names", path)
	}
}

func TestRetiredManagementServiceHotspotsUseCapabilityNames(t *testing.T) {
	paths := []string{
		filepath.Join("..", "internal", "sheinloginmanaged", "bridge.go"),
		filepath.Join("..", "internal", "shein", "publish", "exists_check.go"),
		filepath.Join("..", "internal", "shein", "publish", "variant_success.go"),
		filepath.Join("..", "internal", "app", "consumer", "listing_runtime_support.go"),
		filepath.Join("..", "internal", "shein", "client", "cookie_manager_local_test.go"),
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, phrase := range []string{
			"management client",
			"management store client",
			"managementapi",
		} {
			if strings.Contains(string(content), phrase) {
				t.Fatalf("%s mentions %q; use store API/runtime repository capability names", path, phrase)
			}
		}
	}
}

func TestRetiredManagementRuntimeNamingHotspotsUseCapabilityNames(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("..", "internal", "shein", "scheduler", "factory.go"): {
			"type managementRuntime",
			"managementClient",
			"platformbase.ManagementRuntime",
			"ManagementRuntime:",
			"GetManagementRuntime",
		},
		filepath.Join("..", "internal", "shein", "scheduler", "runtime_adapter.go"): {
			"ManagementRuntime",
			"NewManagementRuntime",
		},
		filepath.Join("..", "internal", "shein", "pipeline", "processor.go"): {
			"type managementRuntime",
		},
		filepath.Join("..", "internal", "shein", "pipeline", "dependencies_builder.go"): {
			"managementRuntime",
		},
		filepath.Join("..", "internal", "temu", "product", "submit_handler.go"): {
			"management_api",
		},
		filepath.Join("..", "internal", "temu", "pricing", "auto_pricing_service.go"): {
			"managementClient不能为空",
		},
		filepath.Join("..", "internal", "temu", "pricing", "pricing_decision_service.go"): {
			"managementClient不能为空",
		},
		filepath.Join("..", "internal", "app", "bootstrap", "resources", "shared_resources.go"): {
			"managementSchedulerFactoryRuntime",
			"managementProcessorRuntime",
		},
		filepath.Join("..", "internal", "platformbase", "base_factory.go"): {
			"ManagementRuntime",
			"GetManagementRuntime",
		},
		filepath.Join("..", "internal", "temu", "scheduler", "factory.go"): {
			"ManagementRuntime:",
			"GetManagementRuntime",
		},
		filepath.Join("..", "internal", "temu", "sync", "product_sync_service.go"): {
			"management_client",
		},
		filepath.Join("..", "internal", "temu", "sync", "product_sync_repository_test.go"): {
			"WithoutManagementClient",
		},
		filepath.Join("..", "internal", "shein", "productsync", "product_sync_repository_test.go"): {
			"WithoutManagementClient",
		},
		filepath.Join("..", "internal", "pricing", "cost_calculator_test.go"): {
			"managementClient",
		},
	}

	for path, phrases := range checks {
		content, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			t.Fatalf("read %s: %v", path, err)
		}
		for _, phrase := range phrases {
			if strings.Contains(string(content), phrase) {
				t.Fatalf("%s mentions %q; use runtime/repository/listingadmin capability names", path, phrase)
			}
		}
	}
}

func TestAppRunnerSchedulerStoreRuntimeUsesCapabilityNames(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "runner", "scheduler_task_starter.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	for _, phrase := range []string{
		"management client is not initialized",
		"client SchedulerRuntimeProvider",
		"schedulerStoreRuntimeAdapter{client:",
	} {
		if strings.Contains(string(content), phrase) {
			t.Fatalf("%s mentions %q; use scheduler runtime names for scheduler store runtime dependencies", path, phrase)
		}
	}
}

func TestAppRunnerProcessorLifecycleUsesRuntimeNames(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "runner", "processor_lifecycle.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, phrase := range []string{
		"s.managementClient == nil",
		"管理客户端未注入",
	} {
		if strings.Contains(string(content), phrase) {
			t.Fatalf("%s mentions %q; gate processor startup on processor runtime dependencies", path, phrase)
		}
	}
}

func TestAppRunnerHealthChecksUseRuntimeNames(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("..", "internal", "app", "runner", "health_checks.go"): {
			"ManagementClientHealthCheck",
			"management_client",
			"管理客户端未初始化",
		},
		filepath.Join("..", "internal", "app", "runner", "processor_service_impl.go"): {
			"ManagementClientHealthCheck",
		},
	}

	for path, phrases := range checks {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, phrase := range phrases {
			if strings.Contains(string(content), phrase) {
				t.Fatalf("%s mentions %q; name runner health checks after runtime dependencies", path, phrase)
			}
		}
	}
}

func TestTemuProcessorRuntimeUsesCapabilityNames(t *testing.T) {
	paths := []string{
		filepath.Join("..", "internal", "temu", "processor.go"),
		filepath.Join("..", "internal", "temu", "dependencies_builder.go"),
	}

	for _, path := range paths {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		ast.Inspect(file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.TypeSpec:
				if typed.Name != nil && typed.Name.Name == "managementRuntime" {
					t.Fatalf("%s defines managementRuntime; use processor runtime names for TEMU processor ports", path)
				}
			case *ast.Field:
				for _, name := range typed.Names {
					if name.Name == "ManagementRuntime" || name.Name == "managementRuntime" {
						t.Fatalf("%s exposes %s; use processor runtime names for TEMU processor ports", path, name.Name)
					}
				}
			case *ast.KeyValueExpr:
				if key, ok := typed.Key.(*ast.Ident); ok && key.Name == "ManagementRuntime" {
					t.Fatalf("%s assigns ManagementRuntime; use ProcessorRuntime for TEMU processor dependencies", path)
				}
			}
			return true
		})
	}
}

func TestAppBootstrapRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "bootstrap")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "app.go")):                        {},
		filepath.Clean(filepath.Join(root, "scheduler_factories.go")):        {},
		filepath.Clean(filepath.Join(root, "schedulers", "dependencies.go")): {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep app/bootstrap management dependencies limited to current application assembly retirement seams and prefer in-repository database/repository access for new bootstrap data", path, importPath)
			}
		}
	}
}

func TestListingRuntimeLocalDoesNotImportRetiredManagementPackage(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "listingruntime", "local"), []string{
		`"task-processor/internal/infra/clients/management"`,
	}, nil)
}

func TestListingKitRootOpenAIImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "ai_contracts.go")):     {},
		filepath.Clean(filepath.Join(root, "request_identity.go")): {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if filepath.Dir(path) != filepath.Clean(root) || strings.HasSuffix(filepath.Base(path), "_test.go") {
			continue
		}
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/openai") {
				t.Errorf("%s imports %s; keep ListingKit root concrete OpenAI client dependencies limited to current facade, settings, service, studio, and task seams", path, importPath)
			}
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
	assertNoBannedImports(t, filepath.Join("..", "internal"), []string{
		`"task-processor/internal/app/processor"`,
	}, nil)
}

func TestAppProcessorCompatibilityLayerIsRetired(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "processor")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s still exists; use internal/processor directly instead of the app compatibility layer", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func TestInternalPackagesDoNotImportAppStateCompatibilityLayer(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal"), []string{
		`"task-processor/internal/app/state"`,
	}, nil)
}

func TestAppStateCompatibilityLayerIsRetired(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "state")
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			t.Fatalf("%s still contains Go compatibility file %s; use internal/state directly instead", path, entry.Name())
		}
	}
}

func TestInfraProductCrawlerAdapterIsRetired(t *testing.T) {
	path := filepath.Join("..", "internal", "infra", "productcrawler")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s still exists; use product/sourcing plus app crawler fetchers instead of the unwired infra productcrawler adapter", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func TestAppCrawlerFetcherCompatibilityLayerIsRetired(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "crawler", "fetcher")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s still exists; use internal/crawler/fetcher for product fetcher contracts and implementations", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
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

func TestBusinessDomainsDoNotImportAppHTTPAPI(t *testing.T) {
	for _, domainRoot := range []string{
		filepath.Join("..", "internal", "amazon"),
		filepath.Join("..", "internal", "amazonlisting"),
		filepath.Join("..", "internal", "asset"),
		filepath.Join("..", "internal", "catalog"),
		filepath.Join("..", "internal", "listing"),
		filepath.Join("..", "internal", "listingkit"),
		filepath.Join("..", "internal", "marketplace"),
		filepath.Join("..", "internal", "pricing"),
		filepath.Join("..", "internal", "productenrich"),
		filepath.Join("..", "internal", "productimage"),
		filepath.Join("..", "internal", "publishing"),
		filepath.Join("..", "internal", "sds"),
		filepath.Join("..", "internal", "shein"),
		filepath.Join("..", "internal", "temu"),
	} {
		t.Run(filepath.Base(domainRoot), func(t *testing.T) {
			assertNoBannedImports(t, domainRoot, []string{
				`"task-processor/internal/app/httpapi"`,
			}, nil)
		})
	}
}

func TestProjectBoundaryDomainsDoNotImportListingKitFacade(t *testing.T) {
	for _, domainRoot := range []string{
		filepath.Join("..", "internal", "amazon"),
		filepath.Join("..", "internal", "asset"),
		filepath.Join("..", "internal", "catalog"),
		filepath.Join("..", "internal", "infra"),
		filepath.Join("..", "internal", "integration"),
		filepath.Join("..", "internal", "marketplace"),
		filepath.Join("..", "internal", "platform"),
		filepath.Join("..", "internal", "product", "sourcing"),
		filepath.Join("..", "internal", "productimage"),
		filepath.Join("..", "internal", "publishing"),
		filepath.Join("..", "internal", "shein"),
		filepath.Join("..", "internal", "temu"),
		filepath.Join("..", "internal", "workspace"),
	} {
		t.Run(filepath.ToSlash(domainRoot), func(t *testing.T) {
			assertNoBannedImportPrefixes(t, domainRoot, []string{
				"task-processor/internal/listingkit",
			}, nil)
		})
	}
}

func TestInfrastructurePackagesDoNotImportBusinessDomains(t *testing.T) {
	for _, infraRoot := range []string{
		filepath.Join("..", "internal", "infra"),
		filepath.Join("..", "internal", "integration"),
		filepath.Join("..", "internal", "platformbase"),
		filepath.Join("..", "internal", "platformtask"),
	} {
		t.Run(filepath.ToSlash(infraRoot), func(t *testing.T) {
			assertNoBannedImportPrefixes(t, infraRoot, []string{
				"task-processor/internal/amazon",
				"task-processor/internal/amazonlisting",
				"task-processor/internal/asset",
				"task-processor/internal/catalog",
				"task-processor/internal/listing",
				"task-processor/internal/listingkit",
				"task-processor/internal/marketplace",
				"task-processor/internal/pricing",
				"task-processor/internal/productenrich",
				"task-processor/internal/productimage",
				"task-processor/internal/publishing",
				"task-processor/internal/sds",
				"task-processor/internal/shein",
				"task-processor/internal/temu",
				"task-processor/internal/workspace",
			}, nil)
		})
	}
}

func TestBusinessDomainsDoNotImportAppRuntimeAssembly(t *testing.T) {
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join("..", "internal", "listingkit", "httpapi")) + string(os.PathSeparator): {},
	}

	for _, domainRoot := range []string{
		filepath.Join("..", "internal", "amazon"),
		filepath.Join("..", "internal", "amazonlisting"),
		filepath.Join("..", "internal", "asset"),
		filepath.Join("..", "internal", "catalog"),
		filepath.Join("..", "internal", "listing"),
		filepath.Join("..", "internal", "listingkit"),
		filepath.Join("..", "internal", "marketplace"),
		filepath.Join("..", "internal", "pricing"),
		filepath.Join("..", "internal", "productenrich"),
		filepath.Join("..", "internal", "productimage"),
		filepath.Join("..", "internal", "publishing"),
		filepath.Join("..", "internal", "sds"),
		filepath.Join("..", "internal", "shein"),
		filepath.Join("..", "internal", "temu"),
	} {
		t.Run(filepath.Base(domainRoot), func(t *testing.T) {
			assertNoBannedImportPrefixes(t, domainRoot, []string{
				"task-processor/internal/app/bootstrap",
				"task-processor/internal/app/consumer",
				"task-processor/internal/app/runner",
				"task-processor/internal/app/runtime",
			}, allowedFiles)
		})
	}
}

func TestBusinessImplementationPackagesDoNotImportGinDirectly(t *testing.T) {
	root := filepath.Join("..", "internal")
	allowedHTTPPackages := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "app", "httpapi")) + string(os.PathSeparator):            {},
		filepath.Clean(filepath.Join(root, "amazonlisting", "api")) + string(os.PathSeparator):      {},
		filepath.Clean(filepath.Join(root, "amazonlisting", "httpapi")) + string(os.PathSeparator):  {},
		filepath.Clean(filepath.Join(root, "httproute")) + string(os.PathSeparator):                 {},
		filepath.Clean(filepath.Join(root, "kernel", "module")) + string(os.PathSeparator):          {},
		filepath.Clean(filepath.Join(root, "listingkit", "api")) + string(os.PathSeparator):         {},
		filepath.Clean(filepath.Join(root, "listingkit", "httpapi")) + string(os.PathSeparator):     {},
		filepath.Clean(filepath.Join(root, "productimage", "httpapi")) + string(os.PathSeparator):   {},
		filepath.Clean(filepath.Join(root, "productenrich", "api")) + string(os.PathSeparator):      {},
		filepath.Clean(filepath.Join(root, "productenrich", "httpapi")) + string(os.PathSeparator):  {},
		filepath.Clean(filepath.Join(root, "promptmgmt", "api")) + string(os.PathSeparator):         {},
		filepath.Clean(filepath.Join(root, "sds", "httpapi")) + string(os.PathSeparator):            {},
		filepath.Clean(filepath.Join(root, "sdslogin")) + string(os.PathSeparator):                  {},
		filepath.Clean(filepath.Join(root, "sheinlogin")) + string(os.PathSeparator):                {},
		filepath.Clean(filepath.Join(root, "taskrpcapi")) + string(os.PathSeparator):                {},
		filepath.Clean(filepath.Join(root, "amazonlisting", "interfaces.go")):                       {},
		filepath.Clean(filepath.Join(root, "listingadmin", "category_handler.go")):                  {},
		filepath.Clean(filepath.Join(root, "listingadmin", "filter_rule_handler.go")):               {},
		filepath.Clean(filepath.Join(root, "listingadmin", "generation_topic_catalog_handler.go")):  {},
		filepath.Clean(filepath.Join(root, "listingadmin", "generation_topic_override_handler.go")): {},
		filepath.Clean(filepath.Join(root, "listingadmin", "generation_topic_policy_handler.go")):   {},
		filepath.Clean(filepath.Join(root, "listingadmin", "handler_helpers.go")):                   {},
		filepath.Clean(filepath.Join(root, "listingadmin", "import_task_handler.go")):               {},
		filepath.Clean(filepath.Join(root, "listingadmin", "operation_strategy_handler.go")):        {},
		filepath.Clean(filepath.Join(root, "listingadmin", "pricing_rule_handler.go")):              {},
		filepath.Clean(filepath.Join(root, "listingadmin", "product_data_handler.go")):              {},
		filepath.Clean(filepath.Join(root, "listingadmin", "product_import_mapping_handler.go")):    {},
		filepath.Clean(filepath.Join(root, "listingadmin", "profit_rule_handler.go")):               {},
		filepath.Clean(filepath.Join(root, "listingadmin", "request_context.go")):                   {},
		filepath.Clean(filepath.Join(root, "listingadmin", "sensitive_word_handler.go")):            {},
		filepath.Clean(filepath.Join(root, "listingadmin", "store_handler.go")):                     {},
		filepath.Clean(filepath.Join(root, "listingadmin", "store_statistics_handler.go")):          {},
		filepath.Clean(filepath.Join(root, "listingkit", "studio_session_handler.go")):              {},
		filepath.Clean(filepath.Join(root, "listingsubscription", "handler.go")):                    {},
		filepath.Clean(filepath.Join(root, "productenrich", "handler.go")):                          {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") || pathAllowed(path, allowedHTTPPackages) {
			continue
		}
		if _, ok := facts.imports[`"github.com/gin-gonic/gin"`]; ok {
			t.Errorf("%s imports gin directly; keep HTTP framework dependencies in api/httpapi or explicitly registered legacy HTTP adapter packages", path)
		}
	}
}

func TestProductImageExternalClientImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "productimage")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "failure_test.go")):                            {},
		filepath.Clean(filepath.Join(root, "httpapi", "model_provider_builder.go")):       {},
		filepath.Clean(filepath.Join(root, "httpapi", "model_provider_defaults_test.go")): {},
		filepath.Clean(filepath.Join(root, "httpapi", "runtime_builder.go")):              {},
		filepath.Clean(filepath.Join(root, "openai_image_edit_adapter.go")):               {},
		filepath.Clean(filepath.Join(root, "pipeline_test.go")):                           {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for _, bannedImport := range []string{
			`"task-processor/internal/infra/clients/nanobanana"`,
			`"task-processor/internal/infra/clients/openai"`,
		} {
			if _, ok := facts.imports[bannedImport]; ok {
				t.Errorf("%s imports %s; keep productimage concrete external clients limited to current provider adapter and runtime builder seams", path, bannedImport)
			}
		}
	}
}

func TestAmazonExternalClientImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "amazon")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "llm", "openai_llm_client.go")): {},
		filepath.Clean(filepath.Join(root, "processor.go")):                {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") ||
				importMatchesPrefix(importPath, "task-processor/internal/infra/clients/openai") {
				t.Errorf("%s imports %s; keep Amazon concrete external client dependencies limited to current management DTO and OpenAI LLM seams", path, importPath)
			}
		}
	}
}

func TestSheinBridgeExternalClientImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "sheinbridge")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "saleattribute", "handler.go")):      {},
		filepath.Clean(filepath.Join(root, "saleattribute", "handler_test.go")): {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") ||
				importMatchesPrefix(importPath, "task-processor/internal/infra/clients/openai") {
				t.Errorf("%s imports %s; keep sheinbridge concrete external client dependencies limited to current sale-attribute runtime bridge seams", path, importPath)
			}
		}
	}
}

func TestSheinRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "shein")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "activity", "mixed.go")):                        {},
		filepath.Clean(filepath.Join(root, "activity", "product_data_helper.go")):          {},
		filepath.Clean(filepath.Join(root, "activity", "profit.go")):                       {},
		filepath.Clean(filepath.Join(root, "activity", "registration.go")):                 {},
		filepath.Clean(filepath.Join(root, "activity", "registration_config.go")):          {},
		filepath.Clean(filepath.Join(root, "activity", "time_limited.go")):                 {},
		filepath.Clean(filepath.Join(root, "addresscopy", "service.go")):                   {},
		filepath.Clean(filepath.Join(root, "api", "image", "client.go")):                   {},
		filepath.Clean(filepath.Join(root, "authorizedbrand", "context_test.go")):          {},
		filepath.Clean(filepath.Join(root, "authorizedbrand", "types.go")):                 {},
		filepath.Clean(filepath.Join(root, "context", "context.go")):                       {},
		filepath.Clean(filepath.Join(root, "inventory", "api.go")):                         {},
		filepath.Clean(filepath.Join(root, "inventory", "change_checker.go")):              {},
		filepath.Clean(filepath.Join(root, "inventory", "cost_calculator.go")):             {},
		filepath.Clean(filepath.Join(root, "inventory", "monitor.go")):                     {},
		filepath.Clean(filepath.Join(root, "inventory", "price_strategy.go")):              {},
		filepath.Clean(filepath.Join(root, "inventory", "record.go")):                      {},
		filepath.Clean(filepath.Join(root, "inventory", "strategy.go")):                    {},
		filepath.Clean(filepath.Join(root, "inventory", "sync.go")):                        {},
		filepath.Clean(filepath.Join(root, "inventory", "types.go")):                       {},
		filepath.Clean(filepath.Join(root, "managedclient", "api_client_test.go")):         {},
		filepath.Clean(filepath.Join(root, "managedclient", "bridge.go")):                  {},
		filepath.Clean(filepath.Join(root, "managedclient", "manager.go")):                 {},
		filepath.Clean(filepath.Join(root, "mapping", "builder.go")):                       {},
		filepath.Clean(filepath.Join(root, "mapping", "service.go")):                       {},
		filepath.Clean(filepath.Join(root, "mapping", "strategies.go")):                    {},
		filepath.Clean(filepath.Join(root, "mapping", "strategies_test.go")):               {},
		filepath.Clean(filepath.Join(root, "mapping", "types.go")):                         {},
		filepath.Clean(filepath.Join(root, "models.go")):                                   {},
		filepath.Clean(filepath.Join(root, "pipeline", "processor.go")):                    {},
		filepath.Clean(filepath.Join(root, "pipeline", "task.go")):                         {},
		filepath.Clean(filepath.Join(root, "pipeline", "task_authorized_brand_test.go")):   {},
		filepath.Clean(filepath.Join(root, "pricing", "auto_pricing.go")):                  {},
		filepath.Clean(filepath.Join(root, "pricing", "calculator.go")):                    {},
		filepath.Clean(filepath.Join(root, "pricing", "calculator_test.go")):               {},
		filepath.Clean(filepath.Join(root, "pricing", "pricing_calculator.go")):            {},
		filepath.Clean(filepath.Join(root, "pricing", "pricing_evaluator.go")):             {},
		filepath.Clean(filepath.Join(root, "product", "skc", "skc_build_input.go")):        {},
		filepath.Clean(filepath.Join(root, "product", "skc", "variant_runtime_test.go")):   {},
		filepath.Clean(filepath.Join(root, "product", "sku", "sku_runtime_input.go")):      {},
		filepath.Clean(filepath.Join(root, "product", "sku", "strategy_test.go")):          {},
		filepath.Clean(filepath.Join(root, "productsync", "product_sync.go")):              {},
		filepath.Clean(filepath.Join(root, "productsync", "product_sync_enricher.go")):     {},
		filepath.Clean(filepath.Join(root, "productsync", "product_sync_types.go")):        {},
		filepath.Clean(filepath.Join(root, "publish", "checker.go")):                       {},
		filepath.Clean(filepath.Join(root, "publish", "exists_check.go")):                  {},
		filepath.Clean(filepath.Join(root, "publish", "handler.go")):                       {},
		filepath.Clean(filepath.Join(root, "publish", "handler_test.go")):                  {},
		filepath.Clean(filepath.Join(root, "publish", "mapping_helper.go")):                {},
		filepath.Clean(filepath.Join(root, "publish", "publish_input.go")):                 {},
		filepath.Clean(filepath.Join(root, "scheduler", "activity_task.go")):               {},
		filepath.Clean(filepath.Join(root, "scheduler", "factory.go")):                     {},
		filepath.Clean(filepath.Join(root, "scheduler", "inventory_sync_adapter.go")):      {},
		filepath.Clean(filepath.Join(root, "scheduler", "inventory_sync_adapter_test.go")): {},
		filepath.Clean(filepath.Join(root, "scheduler", "inventory_task.go")):              {},
		filepath.Clean(filepath.Join(root, "scheduler", "pricing_task.go")):                {},
		filepath.Clean(filepath.Join(root, "scheduler", "product_sync_adapter.go")):        {},
		filepath.Clean(filepath.Join(root, "scheduler", "product_sync_adapter_test.go")):   {},
		filepath.Clean(filepath.Join(root, "scheduler", "product_task.go")):                {},
		filepath.Clean(filepath.Join(root, "store", "store_id.go")):                        {},
		filepath.Clean(filepath.Join(root, "store", "store_info.go")):                      {},
		filepath.Clean(filepath.Join(root, "store", "store_info_test.go")):                 {},
		filepath.Clean(filepath.Join(root, "validation", "get_rule.go")):                   {},
		filepath.Clean(filepath.Join(root, "validation", "reapply_filter.go")):             {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep SHEIN retired management service dependencies limited to current inventory, scheduler, publish, validation, activity, mapping, and product seams", path, importPath)
			}
		}
	}
}

func TestSheinPipelineRuntimeDependenciesDoNotUseRetiredManagementNames(t *testing.T) {
	paths := []string{
		filepath.Join("..", "internal", "shein", "pipeline", "processor.go"),
		filepath.Join("..", "internal", "shein", "pipeline", "dependencies_builder.go"),
	}
	banned := []string{"ManagementClient", "managementClient"}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, token := range banned {
			if strings.Contains(string(content), token) {
				t.Fatalf("%s contains %s; name SHEIN runtime dependencies after the capability, not the retired management service", path, token)
			}
		}
	}
}

func TestSheinOpenAIImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "shein")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "category", "ai_selector.go")):                                  {},
		filepath.Clean(filepath.Join(root, "category", "manager.go")):                                      {},
		filepath.Clean(filepath.Join(root, "content", "optimizer.go")):                                     {},
		filepath.Clean(filepath.Join(root, "pipeline", "pipeline.go")):                                     {},
		filepath.Clean(filepath.Join(root, "product", "attribute", "input.go")):                            {},
		filepath.Clean(filepath.Join(root, "product", "attribute", "platform_value_fallback_llm.go")):      {},
		filepath.Clean(filepath.Join(root, "product", "attribute", "platform_value_fallback_llm_test.go")): {},
		filepath.Clean(filepath.Join(root, "product", "attribute", "sale", "batch_processor_test.go")):     {},
		filepath.Clean(filepath.Join(root, "product", "attribute", "sale", "handler.go")):                  {},
		filepath.Clean(filepath.Join(root, "product", "attribute", "sale", "single_processor.go")):         {},
		filepath.Clean(filepath.Join(root, "product", "attribute", "sale", "single_processor_test.go")):    {},
		filepath.Clean(filepath.Join(root, "product", "attribute", "selector.go")):                         {},
		filepath.Clean(filepath.Join(root, "product", "build", "skc_list.go")):                             {},
		filepath.Clean(filepath.Join(root, "product", "skc", "builder.go")):                                {},
		filepath.Clean(filepath.Join(root, "product", "skc", "translation.go")):                            {},
		filepath.Clean(filepath.Join(root, "product", "skc", "variant.go")):                                {},
		filepath.Clean(filepath.Join(root, "submitprep", "localized_content.go")):                          {},
		filepath.Clean(filepath.Join(root, "translate", "translate.go")):                                   {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/openai") {
				t.Errorf("%s imports %s; keep SHEIN concrete OpenAI client dependencies limited to current category, content, pipeline, product, submit-prep, and translate seams", path, importPath)
			}
		}
	}
}

func TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "httpapi")
	allowedFiles := map[string]struct{}{}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if !strings.Contains(strings.ToLower(filepath.Base(path)), "productimage") {
			continue
		}
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for _, bannedImport := range []string{
			`"task-processor/internal/infra/clients/nanobanana"`,
			`"task-processor/internal/infra/clients/openai"`,
		} {
			if _, ok := facts.imports[bannedImport]; ok {
				t.Errorf("%s imports %s; keep app/httpapi free of ProductImage concrete external clients; ProductImage provider assembly belongs in internal/productimage/httpapi", path, bannedImport)
			}
		}
	}
}

func TestAppHTTPAPIRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "httpapi")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "modules_shein_test.go")):   {},
		filepath.Clean(filepath.Join(root, "runtime_deps_methods.go")): {},
		filepath.Clean(filepath.Join(root, "runtime_deps_test.go")):    {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep app/httpapi management dependencies limited to current HTTP runtime dependency retirement seams and prefer in-repository database/repository access for new HTTP assembly data", path, importPath)
			}
		}
	}
}

func TestAppRuntimeListingRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "runtime", "listing")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "debug_task_runner.go")):      {},
		filepath.Clean(filepath.Join(root, "debug_task_runner_test.go")): {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep app/runtime/listing management dependencies limited to current listing debug runtime retirement seams and prefer in-repository database/repository access for new listing runtime data", path, importPath)
			}
		}
	}
}

func TestAppTaskStatusRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "taskstatus")
	allowedFiles := map[string]struct{}{}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep app/taskstatus management dependencies limited to current task-status retirement seams and prefer in-repository database/repository access for new task-status data", path, importPath)
			}
		}
	}
}

func TestPlatformTaskRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "platformtask")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "auto_pricing_task.go")):        {},
		filepath.Clean(filepath.Join(root, "auto_pricing_task_test.go")):   {},
		filepath.Clean(filepath.Join(root, "inventory_sync_task.go")):      {},
		filepath.Clean(filepath.Join(root, "inventory_sync_task_test.go")): {},
		filepath.Clean(filepath.Join(root, "product_sync_task.go")):        {},
		filepath.Clean(filepath.Join(root, "product_sync_task_test.go")):   {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep platformtask management dependencies limited to current platform task retirement seams and prefer in-repository database/repository access for new platform task data", path, importPath)
			}
		}
	}
}

func TestStateRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "state")
	allowedFiles := map[string]struct{}{}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep state management dependencies limited to current state-runtime retirement seams and prefer in-repository database/repository access for new state data", path, importPath)
			}
		}
	}
}

func TestPlatformBaseRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "platformbase")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "base_factory.go")): {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep platformbase management dependencies limited to current platform-factory retirement seams and prefer in-repository database/repository access for new platform data", path, importPath)
			}
		}
	}
}

func TestProcessorRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "processor")

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep processor free of retired management services and inject narrower runtime-owned ports instead", path, importPath)
			}
		}
	}
}

func TestTaskRPCAPIRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "taskrpcapi")
	allowedFiles := map[string]struct{}{}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep taskrpcapi management dependencies limited to current RPC assembly retirement seams and prefer in-repository database/repository access for new task RPC data", path, importPath)
			}
		}
	}
}

func TestSDSClientRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "sds", "client")
	allowedFiles := map[string]struct{}{}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep sds/client management dependencies limited to current SDS auth bootstrap retirement seams and prefer in-repository database/repository access for new SDS state data", path, importPath)
			}
		}
	}
}

func TestSheinLoginBootstrapRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "sheinlogin", "bootstrap")
	allowedFiles := map[string]struct{}{}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep sheinlogin/bootstrap management dependencies limited to current login bootstrap retirement seams and prefer in-repository database/repository access for new login bootstrap data", path, importPath)
			}
		}
	}
}

func TestSheinLoginManagedRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "sheinloginmanaged")
	allowedFiles := map[string]struct{}{}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep sheinloginmanaged management dependencies limited to current managed-login bridge retirement seams and prefer in-repository database/repository access for new managed-login data", path, importPath)
			}
		}
	}
}

func TestSheinLoginUsesManagementAPIPortAliases(t *testing.T) {
	roots := []string{
		filepath.Join("..", "internal", "sheinlogin"),
		filepath.Join("..", "internal", "sheinloginmanaged"),
	}
	for _, root := range roots {
		index, err := loadGoFileIndex(root, "")
		if err != nil {
			t.Fatal(err)
		}
		for path, facts := range index.files {
			if _, ok := facts.imports[`"task-processor/internal/infra/clients/management/api"`]; ok {
				t.Fatalf("%s imports concrete management API DTOs; use listingadmin/local runtime types", path)
			}
		}
	}
}

func TestSheinLoginServiceRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "sheinlogin")
	allowedFiles := map[string]struct{}{}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep sheinlogin management dependencies limited to current login-service retirement seams and prefer in-repository database/repository access for new login service data", path, importPath)
			}
		}
	}
}

func TestSharedPricingRetiredManagementImportsStayBlocked(t *testing.T) {
	root := filepath.Join("..", "internal", "pricing")
	allowedFiles := map[string]struct{}{}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep shared pricing management dependencies limited to current pricing-config retirement seams and prefer in-repository database/repository access for new pricing data", path, importPath)
			}
		}
	}
}

func TestTEMUSyncAndPricingRetiredManagementImportsStayBlocked(t *testing.T) {
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join("..", "internal", "temu", "pricing", "auto_pricing_service.go")):        {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "pricing", "interfaces.go")):                  {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "pricing", "pricing_data_service.go")):        {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "pricing", "pricing_decision_service.go")):    {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "pricing", "pricing_rule_calculator.go")):     {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "pricing", "store_config_service.go")):        {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "factory.go")):                        {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "inventory_sync_api.go")):             {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "inventory_sync_change_checker.go")):  {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "inventory_sync_concurrent.go")):      {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "inventory_sync_cost_calculator.go")): {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "inventory_sync_factory.go")):         {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "inventory_sync_interface.go")):       {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "inventory_sync_monitor.go")):         {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "inventory_sync_record.go")):          {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "inventory_sync_service.go")):         {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "inventory_sync_strategy.go")):        {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "inventory_sync_updater.go")):         {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "mapping.go")):                        {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "product_converter.go")):              {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "product_data_builder.go")):           {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "product_data_helpers.go")):           {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "product_sync_service.go")):           {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "product_sync_types.go")):             {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "product_utils.go")):                  {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "sku_details_handler.go")):            {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "sync", "sku_mapping_enricher.go")):           {},
	}

	for _, root := range []string{
		filepath.Join("..", "internal", "temu", "pricing"),
		filepath.Join("..", "internal", "temu", "sync"),
	} {
		t.Run(filepath.ToSlash(root), func(t *testing.T) {
			index, err := loadGoFileIndex(root, "")
			if err != nil {
				t.Fatal(err)
			}

			for path, facts := range index.files {
				if pathAllowed(path, allowedFiles) {
					continue
				}
				for quotedImport := range facts.imports {
					importPath := strings.Trim(quotedImport, `"`)
					if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
						t.Errorf("%s imports %s; keep TEMU retired management service dependencies limited to current sync and pricing seams", path, importPath)
					}
				}
			}
		})
	}
}

func TestTEMUProductStoreAndSchedulerRetiredManagementImportsStayBlocked(t *testing.T) {
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join("..", "internal", "temu", "product", "build_spu_handler.go")):           {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "product", "exists_check_handler.go")):        {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "product", "filter_checker.go")):              {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "product", "price_handler.go")):               {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "product", "publish_result_input.go")):        {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "product", "save_publish_result_handler.go")): {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "product", "spu_builder.go")):                 {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "product", "submit_handler.go")):              {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "scheduler", "auto_pricing_adapter.go")):      {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "scheduler", "auto_pricing_adapter_test.go")): {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "scheduler", "factory.go")):                   {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "scheduler", "inventory_sync_adapter.go")):    {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "scheduler", "inventory_task.go")):            {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "scheduler", "pricing_task.go")):              {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "scheduler", "product_sync_adapter.go")):      {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "scheduler", "product_sync_adapter_test.go")): {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "scheduler", "product_task.go")):              {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "store", "id_handler.go")):                    {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "store", "info_handler.go")):                  {},
	}

	for _, root := range []string{
		filepath.Join("..", "internal", "temu", "product"),
		filepath.Join("..", "internal", "temu", "scheduler"),
		filepath.Join("..", "internal", "temu", "store"),
	} {
		t.Run(filepath.ToSlash(root), func(t *testing.T) {
			index, err := loadGoFileIndex(root, "")
			if err != nil {
				t.Fatal(err)
			}

			for path, facts := range index.files {
				if pathAllowed(path, allowedFiles) {
					continue
				}
				for quotedImport := range facts.imports {
					importPath := strings.Trim(quotedImport, `"`)
					if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
						t.Errorf("%s imports %s; keep TEMU retired management service dependencies limited to current product, store, and scheduler seams", path, importPath)
					}
				}
			}
		})
	}
}

func TestTEMURuntimeAndBridgeRetiredManagementImportsStayBlocked(t *testing.T) {
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join("..", "internal", "temu", "api", "client", "client.go")):            {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "api", "client", "cookie_manager.go")):    {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "api", "client", "manager.go")):           {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "bulkrelist", "bulk_relist_entry.go")):    {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "handlerbase", "fulfillment_checker.go")): {},
		filepath.Clean(filepath.Join("..", "internal", "temu", "processor.go")):                          {},
	}

	for _, root := range []string{
		filepath.Join("..", "internal", "temu", "api", "client"),
		filepath.Join("..", "internal", "temu", "bulkrelist"),
		filepath.Join("..", "internal", "temu", "context"),
		filepath.Join("..", "internal", "temu", "filter"),
		filepath.Join("..", "internal", "temu", "handlerbase"),
		filepath.Join("..", "internal", "temu", "rules"),
	} {
		t.Run(filepath.ToSlash(root), func(t *testing.T) {
			index, err := loadGoFileIndex(root, "")
			if err != nil {
				t.Fatal(err)
			}

			for path, facts := range index.files {
				if pathAllowed(path, allowedFiles) {
					continue
				}
				for quotedImport := range facts.imports {
					importPath := strings.Trim(quotedImport, `"`)
					if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
						t.Errorf("%s imports %s; keep TEMU runtime and bridge management dependencies limited to current runtime-assembly retirement seams and prefer in-repository database/repository access for new TEMU runtime data", path, importPath)
					}
				}
			}
		})
	}

	root := filepath.Join("..", "internal", "temu")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	target := filepath.Clean(filepath.Join(root, "processor.go"))
	for path, facts := range index.files {
		if path != target || pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/management") {
				t.Errorf("%s imports %s; keep TEMU runtime and bridge management dependencies limited to current runtime-assembly retirement seams and prefer in-repository database/repository access for new TEMU runtime data", path, importPath)
			}
		}
	}
}

func TestTEMUOpenAIImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "temu")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "ai", "content_rewriter.go")):             {},
		filepath.Clean(filepath.Join(root, "ai", "property_mapper_core.go")):         {},
		filepath.Clean(filepath.Join(root, "ai", "service.go")):                      {},
		filepath.Clean(filepath.Join(root, "image", "dimension_annotator.go")):       {},
		filepath.Clean(filepath.Join(root, "image", "vision_detector.go")):           {},
		filepath.Clean(filepath.Join(root, "pipeline_registry.go")):                  {},
		filepath.Clean(filepath.Join(root, "product", "build_spu_handler.go")):       {},
		filepath.Clean(filepath.Join(root, "product", "spu_builder.go")):             {},
		filepath.Clean(filepath.Join(root, "sku", "ai_mapping_handler.go")):          {},
		filepath.Clean(filepath.Join(root, "sku", "ai_mapping_single_processor.go")): {},
		filepath.Clean(filepath.Join(root, "sku", "builder.go")):                     {},
		filepath.Clean(filepath.Join(root, "sku", "variant_processor.go")):           {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			if importMatchesPrefix(importPath, "task-processor/internal/infra/clients/openai") {
				t.Errorf("%s imports %s; keep TEMU concrete OpenAI client dependencies limited to current AI, image, SKU, product, and pipeline seams", path, importPath)
			}
		}
	}
}

func TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages(t *testing.T) {
	assertNoBannedImportPrefixes(t, filepath.Join("..", "internal", "platforms"), []string{
		"task-processor/internal/app/httpapi",
		"task-processor/internal/asset",
		"task-processor/internal/catalog",
		"task-processor/internal/listingkit",
		"task-processor/internal/marketplace",
		"task-processor/internal/productimage",
		"task-processor/internal/publishing",
		"task-processor/internal/workspace",
	}, nil)
}

func TestPlatformModulesHistoricalImplementationImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "platforms")
	allowedImports := map[string]map[string]struct{}{
		"task-processor/internal/amazon": {
			filepath.Clean(filepath.Join(root, "amazon", "module.go")): {},
		},
		"task-processor/internal/shein/pipeline": {
			filepath.Clean(filepath.Join(root, "shein", "module.go")): {},
		},
		"task-processor/internal/temu": {
			filepath.Clean(filepath.Join(root, "temu", "module.go")): {},
		},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}

	for path, facts := range index.files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") {
			continue
		}
		for _, bannedPrefix := range []string{
			"task-processor/internal/amazon",
			"task-processor/internal/shein",
			"task-processor/internal/temu",
		} {
			for quotedImport := range facts.imports {
				importPath := strings.Trim(quotedImport, `"`)
				if importMatchesPrefix(importPath, bannedPrefix) {
					if allowedFiles, ok := allowedImports[importPath]; ok {
						if _, allowed := allowedFiles[path]; allowed {
							continue
						}
					}
					t.Errorf("%s imports %s; keep internal/platforms as thin registration and route new platform rules to marketplace or publishing packages", path, importPath)
				}
			}
		}
	}
}

func TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages(t *testing.T) {
	assertNoBannedImportPrefixes(t, filepath.Join("..", "cmd"), []string{
		"task-processor/internal/amazon",
		"task-processor/internal/amazonlisting",
		"task-processor/internal/asset",
		"task-processor/internal/catalog",
		"task-processor/internal/infra",
		"task-processor/internal/listingkit",
		"task-processor/internal/marketplace",
		"task-processor/internal/productenrich",
		"task-processor/internal/productimage",
		"task-processor/internal/publishing",
		"task-processor/internal/shein",
		"task-processor/internal/temu",
		"task-processor/internal/workspace",
	}, nil)
}

func TestTemporalSDKImportsStayInRuntimeAndOrchestrationAdapters(t *testing.T) {
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join("..", "internal", "app", "runtime")) + string(os.PathSeparator):                  {},
		filepath.Clean(filepath.Join("..", "internal", "listingkit", "temporal")) + string(os.PathSeparator):          {},
		filepath.Clean(filepath.Join("..", "internal", "listingkit", "task_temporal_submission_activity_support.go")): {},
	}
	assertNoBannedImports(t, filepath.Join("..", "internal"), []string{
		`"go.temporal.io/api/enums/v1"`,
		`"go.temporal.io/api/serviceerror"`,
		`"go.temporal.io/sdk/activity"`,
		`"go.temporal.io/sdk/client"`,
		`"go.temporal.io/sdk/converter"`,
		`"go.temporal.io/sdk/temporal"`,
		`"go.temporal.io/sdk/testsuite"`,
		`"go.temporal.io/sdk/worker"`,
		`"go.temporal.io/sdk/workflow"`,
	}, allowedFiles)
}

func TestTemporalRuntimePackagesDoNotImportHTTPAPI(t *testing.T) {
	for _, root := range []string{
		filepath.Join("..", "internal", "app", "runtime"),
		filepath.Join("..", "internal", "listingkit", "temporal"),
		filepath.Join("..", "internal", "listingkit", "workflow"),
	} {
		t.Run(filepath.ToSlash(root), func(t *testing.T) {
			assertNoBannedImportPrefixes(t, root, []string{
				"task-processor/internal/app/httpapi",
				"task-processor/internal/listingkit/httpapi",
			}, nil)
		})
	}
}

func TestAppHTTPAPIRootListingKitHelpersStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "httpapi")
	allowed := map[string]struct{}{
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

func TestListingKitSupportFileStaysRetired(t *testing.T) {
	path := filepath.Join("..", "internal", "app", "httpapi", "listingkit_support.go")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s still exists; keep ListingKit runtime input shaping in feature_builder_listingkit.go instead of reviving the transition bucket", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func TestAppHTTPAPIModuleBuildersStayAllowlisted(t *testing.T) {
	filePath := filepath.Join("..", "internal", "app", "httpapi", "modules.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
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

func TestAppHTTPAPIRouteDescriptorHelpersStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "httpapi")
	allowed := map[string]struct{}{
		"appendTaskRPCRouteDescriptors":    {},
		"appendSheinLoginRouteDescriptors": {},
		"appendSDSLoginRouteDescriptors":   {},
		"appendSDSCatalogRouteDescriptors": {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for path, facts := range index.files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, facts.source, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			name := fn.Name.Name
			if !strings.HasPrefix(name, "append") || !strings.HasSuffix(name, "RouteDescriptors") {
				continue
			}
			if _, ok := allowed[name]; !ok {
				t.Errorf("%s declares app/httpapi route descriptor helper %s; add new feature routes in the owning domain httpapi package instead", path, name)
			}
		}
	}
}

func TestAppHTTPAPIListingKitSupportImportsStayAllowlisted(t *testing.T) {
	filePath := filepath.Join("..", "internal", "app", "httpapi", "listingkit_support.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
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

func TestAppHTTPAPIListingKitRootImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "httpapi")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "runtime_support_listingkit.go")): {},
		filepath.Clean(filepath.Join(root, "route_handler_types.go")):        {},
		filepath.Clean(filepath.Join(root, "types.go")):                      {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") {
			continue
		}
		if _, ok := facts.imports[`"task-processor/internal/listingkit"`]; !ok {
			continue
		}
		if _, ok := allowedFiles[path]; !ok {
			t.Errorf("%s imports task-processor/internal/listingkit; keep app/httpapi ListingKit root dependencies limited to runtime support types and move new feature logic into internal/listingkit/httpapi", path)
		}
	}
}

func TestAppHTTPAPIListingKitHTTPAPIImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "httpapi")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "composition_builder.go")):        {},
		filepath.Clean(filepath.Join(root, "feature_module_builders.go")):    {},
		filepath.Clean(filepath.Join(root, "feature_builder_listingkit.go")): {},
		filepath.Clean(filepath.Join(root, "http_modules.go")):               {},
		filepath.Clean(filepath.Join(root, "listingkit_temporal_worker.go")): {},
		filepath.Clean(filepath.Join(root, "runtime_login_modules.go")):      {},
		filepath.Clean(filepath.Join(root, "runtime.go")):                    {},
		filepath.Clean(filepath.Join(root, "runtime_deps_methods.go")):       {},
		filepath.Clean(filepath.Join(root, "route_handler_types.go")):        {},
		filepath.Clean(filepath.Join(root, "server.go")):                     {},
		filepath.Clean(filepath.Join(root, "server_auth.go")):                {},
		filepath.Clean(filepath.Join(root, "types.go")):                      {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") {
			continue
		}
		if _, ok := facts.imports[`"task-processor/internal/listingkit/httpapi"`]; !ok {
			continue
		}
		if _, ok := allowedFiles[path]; !ok {
			t.Errorf("%s imports task-processor/internal/listingkit/httpapi; keep app/httpapi feature-owned ListingKit HTTPAPI dependencies limited to current module, runtime, route, and server assembly files", path)
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
		if pathAllowed(path, allowedFiles) {
			continue
		}
		for _, banned := range bannedImports {
			if _, ok := facts.imports[banned]; ok {
				t.Errorf("%s imports banned boundary package %s", path, banned)
			}
		}
	}
}

func assertNoProductionBannedImports(t *testing.T, root string, bannedImports []string, allowedFiles map[string]struct{}) {
	t.Helper()

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") || pathAllowed(path, allowedFiles) {
			continue
		}
		for _, banned := range bannedImports {
			if _, ok := facts.imports[banned]; ok {
				t.Errorf("%s imports banned production boundary package %s", path, banned)
			}
		}
	}
}

func assertNoBannedImportPrefixes(t *testing.T, root string, bannedPrefixes []string, allowedFiles map[string]struct{}) {
	t.Helper()

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") || pathAllowed(path, allowedFiles) {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			for _, bannedPrefix := range bannedPrefixes {
				if importMatchesPrefix(importPath, bannedPrefix) {
					t.Errorf("%s imports banned boundary package prefix %s via %s", path, bannedPrefix, importPath)
				}
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
