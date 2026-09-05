package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLegacyPreviewAdapterStaysRetired(t *testing.T) {
	sources := trackedProductionTextSources(t, []string{"internal/compatibility/listingkit"}, func(path string) bool {
		return strings.HasSuffix(path, ".go")
	})
	violations, err := legacyPreviewAdapterViolations(sources)
	require.NoError(t, err)
	require.Empty(t, violations, "legacy preview adapter must remain retired under every filename")
}

func TestLegacyPreviewAdapterGuardRejectsRenamedRevival(t *testing.T) {
	sources := []listingKitImageBoundarySource{
		{path: "internal/compatibility/listingkit/legacy_preview.go", text: "package listingkit\nfunc AdaptLegacyPreviewShell() {}\n"},
		{path: "internal/compatibility/listingkit/revived_bridge.go", text: "package listingkit\nimport _ \"task-processor/internal/listing/preview\"\n"},
		{path: "internal/compatibility/listingkit/sourcehandoff/allowed.go", text: "package sourcehandoff\nimport _ \"task-processor/internal/product/sourcing\"\n"},
		{path: "internal/compatibility/listingkit/similar.go", text: "package listingkit\nimport _ \"task-processor/internal/listing/previewcache\"\n"},
	}
	violations, err := legacyPreviewAdapterViolations(sources)
	require.NoError(t, err)
	require.Equal(t, []string{
		"internal/compatibility/listingkit/legacy_preview.go -> AdaptLegacyPreviewShell",
		"internal/compatibility/listingkit/revived_bridge.go -> task-processor/internal/listing/preview",
	}, violations)
}

func legacyPreviewAdapterViolations(sources []listingKitImageBoundarySource) ([]string, error) {
	var violations []string
	for _, source := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), source.path, source.text, 0)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Name.Name == "AdaptLegacyPreviewShell" {
				violations = append(violations, filepath.ToSlash(source.path)+" -> AdaptLegacyPreviewShell")
			}
		}
		for _, imported := range file.Imports {
			importPath, err := decodeGoImportPath(imported.Path.Value)
			if err != nil {
				return nil, err
			}
			if importMatchesPrefix(importPath, "task-processor/internal/listing/preview") {
				violations = append(violations, filepath.ToSlash(source.path)+" -> "+importPath)
			}
		}
	}
	sort.Strings(violations)
	return violations, nil
}

func TestLegacyConsumerFreezeRejectsNewEdges(t *testing.T) {
	const old = "internal/app/httpapi/composition_builder.go"
	const legacy = "task-processor/internal/compatibility/listingkit/sourcehandoff/a1688"
	baseline := map[string]struct{}{old + " -> " + legacy: {}}
	for _, tc := range []struct {
		name, path, imported string
		blocked              bool
	}{
		{"existing drain", old, legacy, false},
		{"new file same package", "internal/app/httpapi/new.go", legacy, true},
		{"new import same file", old, legacy + "/httpapi", true},
		{"new compatibility root", old, "task-processor/internal/compatibility/new", true},
		{"new tenant caller", "cmd/new/main.go", "task-processor/internal/tenantbridge", true},
		{"new tenant child", "tools/new/main.go", "task-processor/internal/tenantbridge/bootstrap", true},
		{"test consumer", "internal/product/new_test.go", legacy, true},
		{"current owner", old, "task-processor/internal/product/catalog", false},
		{"similar name", old, "task-processor/internal/tenantbridgex", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, literal := range []string{`"` + tc.imported + `"`, "`" + tc.imported + "`", `"` + strings.Replace(tc.imported, "internal", `\x69nternal`, 1) + `"`} {
				sources := []listingKitImageBoundarySource{{path: tc.path, text: "//go:build guard_fixture\n\npackage fixture\nimport alias " + literal + "\n"}}
				violations, err := legacyConsumerViolations(sources, baseline)
				require.NoError(t, err)
				require.Equal(t, tc.blocked, len(violations) > 0, "%v", violations)
			}
		})
	}
}

func TestLegacyDrainDepguardRules(t *testing.T) {
	rules := loadDepguardRules(t, filepath.Join("..", ".golangci.yml"))
	for _, root := range []string{"compatibility", "tenantbridge"} {
		rule := requireDepguardRule(t, rules, "legacy_"+root+"_consumer_freeze")
		wantFiles := []string{"**/*.go"}
		seen := map[string]bool{}
		for edge := range legacyConsumerBaseline {
			parts := strings.Split(edge, " -> ")
			if importMatchesPrefix(parts[1], "task-processor/internal/"+root) && !seen[parts[0]] {
				wantFiles = append(wantFiles, "!**/"+parts[0])
				seen[parts[0]] = true
			}
		}
		assertExactStringSet(t, root+" files", rule.Files, wantFiles)
		denied := depguardDenyPackageSet(rule)
		for _, suffix := range []string{"$", "/"} {
			_, ok := denied["task-processor/internal/"+root+suffix]
			require.True(t, ok)
		}
	}
}

// Drain-only baseline at main 2fd42cc06. Remove entries as callers drain; never regenerate upward.
// Includes test callers and intra-legacy edges so neither can hide new consumers.
var legacyConsumerBaseline = map[string]struct{}{
	"internal/app/httpapi/composition_builder.go -> task-processor/internal/compatibility/listingkit/sourcehandoff/a1688":                                       {},
	"internal/app/httpapi/composition_builder.go -> task-processor/internal/compatibility/listingkit/sourcehandoff/a1688/httpapi":                               {},
	"internal/app/httpapi/current_product_http_e2e_test.go -> task-processor/internal/tenantbridge":                                                             {},
	"internal/app/httpapi/http_module_test.go -> task-processor/internal/compatibility/listingkit/sourcehandoff/a1688":                                          {},
	"internal/app/httpapi/http_module_test.go -> task-processor/internal/compatibility/listingkit/sourcehandoff/a1688/httpapi":                                  {},
	"internal/app/httpapi/server_test.go -> task-processor/internal/tenantbridge":                                                                               {},
	"internal/app/httpapi/types.go -> task-processor/internal/compatibility/listingkit/sourcehandoff/a1688/httpapi":                                             {},
	"internal/app/runtime/listingkitidentitypreflight/runtime.go -> task-processor/internal/tenantbridge":                                                       {},
	"internal/app/runtime/listingkitidentitypreflight/runtime_test.go -> task-processor/internal/tenantbridge":                                                  {},
	"internal/compatibility/listingkit/sourcehandoff/a1688/command.go -> task-processor/internal/compatibility/listingkit/sourcehandoff":                        {},
	"internal/compatibility/listingkit/sourcehandoff/a1688/command.go -> task-processor/internal/tenantbridge":                                                  {},
	"internal/compatibility/listingkit/sourcehandoff/a1688/httpapi/handler.go -> task-processor/internal/compatibility/listingkit/sourcehandoff/a1688":          {},
	"internal/compatibility/listingkit/sourcehandoff/a1688/httpapi/handler_test.go -> task-processor/internal/compatibility/listingkit/sourcehandoff/a1688":     {},
	"internal/compatibility/listingkit/sourcehandoff/a1688/httpapi/http_module_test.go -> task-processor/internal/compatibility/listingkit/sourcehandoff/a1688": {},
	"internal/compatibility/listingkit/sourcehandoff/a1688/listingkit_task.go -> task-processor/internal/compatibility/listingkit/sourcehandoff":                {},
	"internal/crawler/alibaba1688/api_service.go -> task-processor/internal/tenantbridge":                                                                       {},
	"internal/crawler/alibaba1688/api_service.go -> task-processor/internal/tenantbridge/bootstrap":                                                             {},
	"internal/crawler/alibaba1688/api_service_test.go -> task-processor/internal/tenantbridge":                                                                  {},
	"internal/listingadmin/handler_helpers.go -> task-processor/internal/tenantbridge":                                                                          {},
	"internal/listingadmin/store_handler_test.go -> task-processor/internal/tenantbridge":                                                                       {},
	"internal/listingkit/api/admin_dispatch_event_handler.go -> task-processor/internal/tenantbridge":                                                           {},
	"internal/listingkit/api/shein_sync_handler_support.go -> task-processor/internal/tenantbridge":                                                             {},
	"internal/listingkit/api/shein_sync_handler_test.go -> task-processor/internal/tenantbridge":                                                                {},
	"internal/listingkit/api/subscription_guard.go -> task-processor/internal/tenantbridge":                                                                     {},
	"internal/listingkit/httpapi/builders_legacy_tenant.go -> task-processor/internal/tenantbridge/bootstrap":                                                   {},
	"internal/listingkit/httpapi/runtime_support_shein_adapter_helpers.go -> task-processor/internal/tenantbridge":                                              {},
	"internal/listingkit/httpapi/shein_sync_runtime_bridge_helpers.go -> task-processor/internal/tenantbridge":                                                  {},
	"internal/listingkit/httpapi/shein_sync_runtime_strategy_helpers.go -> task-processor/internal/tenantbridge":                                                {},
	"internal/listingkit/httpapi/shein_sync_runtime_strategy_helpers_test.go -> task-processor/internal/tenantbridge":                                           {},
	"internal/listingkit/service_upload_logic.go -> task-processor/internal/tenantbridge":                                                                       {},
	"internal/listingkit/shein_settings.go -> task-processor/internal/tenantbridge":                                                                             {},
	"internal/listingkit/store_access_test.go -> task-processor/internal/tenantbridge":                                                                          {},
	"internal/listingkit/store_profile_service_test.go -> task-processor/internal/tenantbridge":                                                                 {},
	"internal/listingkit/task_lifecycle_service_support.go -> task-processor/internal/tenantbridge":                                                             {},
	"internal/sheinlogin/tenant_context.go -> task-processor/internal/tenantbridge":                                                                             {},
	"internal/sheinlogin/tenant_context_test.go -> task-processor/internal/tenantbridge":                                                                        {},
	"internal/tenantbridge/bootstrap/configure.go -> task-processor/internal/tenantbridge":                                                                      {},
	"internal/tenantbridge/bootstrap/configure_test.go -> task-processor/internal/tenantbridge":                                                                 {},
	"tests/a1688_source_facts_flow_test.go -> task-processor/internal/compatibility/listingkit/sourcehandoff":                                                   {},
	"tests/a1688_source_to_task_flow_test.go -> task-processor/internal/compatibility/listingkit/sourcehandoff/a1688":                                           {},
	"tests/a1688_source_to_task_flow_test.go -> task-processor/internal/compatibility/listingkit/sourcehandoff/a1688/httpapi":                                   {},
}

func legacyConsumerViolations(sources []listingKitImageBoundarySource, baseline map[string]struct{}) ([]string, error) {
	edges, err := phase3BannedImportDeclarationViolations(sources, []string{
		"task-processor/internal/compatibility", "task-processor/internal/tenantbridge",
	})
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, edge := range edges {
		if _, allowed := baseline[edge]; !allowed {
			violations = append(violations, edge)
		}
	}
	return violations, nil
}

func TestCurrentOwnerLegacyRootDepguardCoverage(t *testing.T) {
	rules := loadDepguardRules(t, filepath.Join("..", ".golangci.yml"))
	rule := requireDepguardRule(t, rules, "current_owners_legacy_listingkit_root")
	var files []string
	for _, owner := range []string{"product", "listing", "marketplace", "agent", "commercetool", "console", "businesstask"} {
		files = append(files, "**/internal/"+owner+"/*.go", "**/internal/"+owner+"/**/*.go")
	}
	assertExactStringSet(t, "current owner coverage", rule.Files, files)
	_, denied := depguardDenyPackageSet(rule)["task-processor/internal/listingkit$"]
	require.True(t, denied, "mixed root service owner must be denied")
}

func TestLegacyImportParserRejectsMalformedSources(t *testing.T) {
	_, err := legacyConsumerViolations([]listingKitImageBoundarySource{{path: "broken.go", text: "package broken\nimport ("}}, nil)
	require.Error(t, err)
}
