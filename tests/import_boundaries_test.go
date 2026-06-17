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

func TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit(t *testing.T) {
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join("..", "internal", "publishing", "shein", "submit_validation.go")): {},
	}
	assertNoBannedImports(t, filepath.Join("..", "internal", "publishing", "shein"), []string{
		`"task-processor/internal/listingkit"`,
		`"task-processor/internal/listingkit/tenantctx"`,
		`"task-processor/internal/productenrich"`,
		`"task-processor/internal/shein/pipeline"`,
		`"task-processor/internal/shein/publish"`,
		`"task-processor/internal/shein/product/build"`,
	}, allowedFiles)
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

func TestAppHTTPAPIListingKitRootImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "app", "httpapi")
	allowedFiles := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "runtime_support_listingkit.go")): {},
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
		filepath.Clean(filepath.Join(root, "feature_builder_listingkit.go")): {},
		filepath.Clean(filepath.Join(root, "http_modules.go")):               {},
		filepath.Clean(filepath.Join(root, "listingkit_support.go")):         {},
		filepath.Clean(filepath.Join(root, "listingkit_temporal_worker.go")): {},
		filepath.Clean(filepath.Join(root, "modules.go")):                    {},
		filepath.Clean(filepath.Join(root, "runtime.go")):                    {},
		filepath.Clean(filepath.Join(root, "server.go")):                     {},
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
