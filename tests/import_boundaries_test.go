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
	assertNoBannedImports(t, filepath.Join("..", "internal", "listingkit", "workspace", "shein"), []string{
		`"task-processor/internal/workspace/shein"`,
	}, nil)
}

func TestListingKitHTTPAPIExternalClientImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit", "httpapi")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "ai_credential_store_adapter.go")):           {},
		filepath.Clean(filepath.Join(root, "ai_image_generator_adapter.go")):            {},
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

func TestTEMUSyncAndPricingManagementImportsStayAllowlisted(t *testing.T) {
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
						t.Errorf("%s imports %s; keep TEMU concrete management client dependencies limited to current sync and pricing seams", path, importPath)
					}
				}
			}
		})
	}
}

func TestTEMUProductStoreAndSchedulerManagementImportsStayAllowlisted(t *testing.T) {
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
						t.Errorf("%s imports %s; keep TEMU concrete management client dependencies limited to current product, store, and scheduler seams", path, importPath)
					}
				}
			}
		})
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
