package tests

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

type listingKitImageBoundarySource struct {
	path string
	text string
}

type listingKitImageBoundaryRule struct {
	name  string
	paths func(string) bool
	match *regexp.Regexp
}

type listingKitImageBoundaryViolation struct {
	rule string
	path string
}

func TestPhase3ListingKitImageWorkflowBoundary(t *testing.T) {
	violations := findListingKitImageBoundaryViolations(trackedListingKitImageBoundarySources(t))
	for _, violation := range violations {
		t.Errorf("%s violates %s", violation.path, violation.rule)
	}
}

func TestPhase3ListingKitImageWorkflowBoundaryRejectsRetiredMutations(t *testing.T) {
	tests := []struct {
		name     string
		source   listingKitImageBoundarySource
		wantRule string
	}{
		{
			name:     "studio route",
			source:   listingKitImageBoundarySource{path: "internal/listingkit/httpapi/routes.go", text: `const path = "/api/v1/listing-kits/studio/sessions"`},
			wantRule: "retired Studio route",
		},
		{
			name:     "studio table",
			source:   listingKitImageBoundarySource{path: "internal/listingkit/schema/runtime.go", text: `db.Table("shein_studio_sessions")`},
			wantRule: "retired Studio data ownership",
		},
		{
			name:     "studio request field",
			source:   listingKitImageBoundarySource{path: "internal/listingkit/types.go", text: "SheinStudio any `json:\"shein_studio\"`"},
			wantRule: "retired Studio data ownership",
		},
		{
			name:     "retired config key",
			source:   listingKitImageBoundarySource{path: "config/config.yaml", text: "aiCapability:\n  studioImageRoutingMode: active"},
			wantRule: "retired ListingKit image runtime",
		},
		{
			name:     "frontend studio proxy",
			source:   listingKitImageBoundarySource{path: "web/listingkit-ui/src/app/api/listing-kits/proxy.ts", text: `if (path[0] === "studio") return 60000`},
			wantRule: "retired Studio route",
		},
		{
			name:     "listingkit imageagent import",
			source:   listingKitImageBoundarySource{path: "internal/listingkit/httpapi/bridge.go", text: `import "task-processor/internal/imageagent"`},
			wantRule: "ListingKit to ImageAgent bridge",
		},
		{
			name:     "studio module",
			source:   listingKitImageBoundarySource{path: "internal/listingsubscription/types.go", text: `const ModuleStudio = "studio"`},
			wantRule: "retired subscription module",
		},
		{
			name:     "studio module config",
			source:   listingKitImageBoundarySource{path: "config/plans.yaml", text: `module_code: studio`},
			wantRule: "retired subscription module",
		},
		{
			name:     "studio metric",
			source:   listingKitImageBoundarySource{path: "internal/integration/openmeter/usage_event.go", text: `const metric = "studio_design_jobs_succeeded"`},
			wantRule: "retired subscription module",
		},
		{
			name:     "tracked sql studio table",
			source:   listingKitImageBoundarySource{path: "scripts/reintroduce_studio.sql", text: `SELECT * FROM shein_studio_sessions`},
			wantRule: "retired Studio data ownership",
		},
		{
			name:     "tracked mjs studio route",
			source:   listingKitImageBoundarySource{path: "tools/reintroduce-studio.mjs", text: `export const endpoint = "/api/v1/listing-kits/studio/sessions";`},
			wantRule: "retired Studio route",
		},
		{
			name:     "tracked Dockerfile studio module",
			source:   listingKitImageBoundarySource{path: "deployments/docker/Dockerfile.listingkit", text: `ENV SUBSCRIPTION_MODULE_CODE=studio`},
			wantRule: "retired subscription module",
		},
		{
			name: "arbitrarily named listingkit provider adapter",
			source: listingKitImageBoundarySource{
				path: "internal/listingkit/httpapi/whatever.go",
				text: "package httpapi\n\nimport secretprovider \"task-processor/internal/integration/openai\"\n\nvar _ = secretprovider.NewClient\n",
			},
			wantRule: "ListingKit provider implementation ownership",
		},
		{
			name: "listingkit http image runtime adapter",
			source: listingKitImageBoundarySource{
				path: "internal/listingkit/httpapi/fetch.go",
				text: "package httpapi\n\nimport runtimeadapter \"task-processor/internal/integration/httpimage\"\n\nvar _ = runtimeadapter.Fetch\n",
			},
			wantRule: "ListingKit provider implementation ownership",
		},
		{
			name: "listingkit product image capability owner",
			source: listingKitImageBoundarySource{
				path: "internal/listingkit/httpapi/generate.go",
				text: "package httpapi\n\nimport imagecapability \"task-processor/internal/product/image\"\n\nvar _ imagecapability.Generator\n",
			},
			wantRule: "ListingKit provider implementation ownership",
		},
		{
			name: "future integration provider name is denied by default",
			source: listingKitImageBoundarySource{
				path: "internal/listingkit/httpapi/future.go",
				text: "package httpapi\n\nimport runtimeadapter \"task-processor/internal/integration/fluxrender\"\n\nvar _ = runtimeadapter.New\n",
			},
			wantRule: "ListingKit provider implementation ownership",
		},
		{
			name: "approved asset persistence child is denied",
			source: listingKitImageBoundarySource{
				path: "internal/listingkit/httpapi/persistence_child.go",
				text: "package httpapi\n\nimport runtimeadapter \"task-processor/internal/integration/persistence/product/asset/fluxrender\"\n\nvar _ = runtimeadapter.New\n",
			},
			wantRule: "ListingKit provider implementation ownership",
		},
		{
			name: "s3 child is denied",
			source: listingKitImageBoundarySource{
				path: "internal/listingkit/httpapi/s3_child.go",
				text: "package httpapi\n\nimport runtimeadapter \"task-processor/internal/integration/s3/fluxrender\"\n\nvar _ = runtimeadapter.New\n",
			},
			wantRule: "ListingKit provider implementation ownership",
		},
		{
			name: "product asset child is denied",
			source: listingKitImageBoundarySource{
				path: "internal/listingkit/product_asset_child.go",
				text: "package listingkit\n\nimport runtimeadapter \"task-processor/internal/product/asset/fluxrender\"\n\nvar _ = runtimeadapter.New\n",
			},
			wantRule: "ListingKit provider implementation ownership",
		},
		{
			name: "product catalog child is denied",
			source: listingKitImageBoundarySource{
				path: "internal/listingkit/product_catalog_child.go",
				text: "package listingkit\n\nimport runtimeadapter \"task-processor/internal/product/catalog/fluxrender\"\n\nvar _ = runtimeadapter.New\n",
			},
			wantRule: "ListingKit provider implementation ownership",
		},
		{
			name:     "retired product generate endpoint",
			source:   listingKitImageBoundarySource{path: "docs/api/listingkit-asset.openapi.yaml", text: `/api/v1/products/generate:`},
			wantRule: "retired product task API",
		},
		{
			name:     "retired image review endpoint",
			source:   listingKitImageBoundarySource{path: "internal/app/httpapi/routes.go", text: `router.POST("/api/v1/images/tasks/:id/review", handler)`},
			wantRule: "retired product task API",
		},
		{
			name:     "retired product task table",
			source:   listingKitImageBoundarySource{path: "internal/app/schema/runtime.go", text: `db.Table("product_enrich_tasks")`},
			wantRule: "retired product task table",
		},
		{
			name:     "retired product worker pool",
			source:   listingKitImageBoundarySource{path: "config/config.yaml", text: `worker_pool: "product_image"`},
			wantRule: "retired product worker or queue",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations := findListingKitImageBoundaryViolations([]listingKitImageBoundarySource{test.source})
			for _, violation := range violations {
				if violation.rule == test.wantRule {
					return
				}
			}
			t.Fatalf("violations = %+v, want rule %q", violations, test.wantRule)
		})
	}
}

func TestPhase3ProductHardCutRuntimeBoundary(t *testing.T) {
	violations := findListingKitImageBoundaryViolations(trackedPhase3ProductHardCutSources(t))
	for _, violation := range violations {
		t.Errorf("%s violates %s", violation.path, violation.rule)
	}
}

func TestPhase3ProductHardCutProductionSelectionCoversRuntimeContractsOnly(t *testing.T) {
	for _, path := range []string{
		"internal/app/httpapi/routes.go",
		"cmd/product-listing-api/main.go",
		"config/config.yaml",
		"deployments/workers/product-image.yaml",
		"scripts/start-product-image-worker",
		"tools/run-product-image-worker",
		"Dockerfile",
		"docs/api/listingkit-asset.openapi.yaml",
	} {
		if !isPhase3ProductHardCutProductionFile(path) {
			t.Errorf("isPhase3ProductHardCutProductionFile(%q) = false, want true", path)
		}
	}
	for _, path := range []string{
		"internal/app/httpapi/routes_test.go",
		"internal/app/httpapi/testdata/routes.go",
		"tests/target_architecture_phase3_product_test.go",
		"docs/superpowers/plans/historical.md",
		"internal/app/generated/routes.go",
		"web/listingkit-ui/src/lib/api/generated/types.gen.ts",
		"web/listingkit-ui/src/lib/api/client.type-test.ts",
	} {
		if isPhase3ProductHardCutProductionFile(path) {
			t.Errorf("isPhase3ProductHardCutProductionFile(%q) = true, want false", path)
		}
	}

	scanned := make(map[string]struct{})
	for _, source := range trackedPhase3ProductHardCutSources(t) {
		scanned[source.path] = struct{}{}
	}
	for _, path := range []string{"docs/api/listingkit-asset.openapi.yaml"} {
		if _, ok := scanned[path]; !ok {
			t.Errorf("production scan set does not contain tracked contract %s", path)
		}
	}
}

func TestPhase3ProductHardCutSelectionAndRuntimeRulesRejectTrackedArtifactMutations(t *testing.T) {
	tests := []listingKitImageBoundarySource{
		{path: "deployments/workers/product-image.yaml", text: "queue: product_image\n"},
		{path: "config/workers.yaml", text: "queue: product_enrich\n"},
		{path: "Dockerfile", text: "ENV WORKER_QUEUE=product_image\n"},
		{path: "scripts/start-product-image-worker", text: "exec worker --queue product_image\n"},
		{path: "tools/run-product-enrich-worker", text: "worker --queue=product_enrich\n"},
	}
	for _, mutation := range tests {
		t.Run(mutation.path, func(t *testing.T) {
			selected := selectPhase3ProductHardCutSources([]listingKitImageBoundarySource{mutation})
			if len(selected) != 1 {
				t.Fatalf("tracked production mutation %q was omitted from the hard-cut source set", mutation.path)
			}
			violations := findListingKitImageBoundaryViolations(selected)
			for _, violation := range violations {
				if violation.rule == "retired product worker or queue" {
					return
				}
			}
			t.Fatalf("violations = %+v, want retired product worker or queue", violations)
		})
	}

	legitimate := listingKitImageBoundarySource{
		path: "internal/imageagent/policy/scene.go",
		text: `package policy; var scene = Scene{SceneStyle: "studio"}`,
	}
	selected := selectPhase3ProductHardCutSources([]listingKitImageBoundarySource{legitimate})
	if violations := findListingKitImageBoundaryViolations(selected); len(violations) != 0 {
		t.Fatalf("legitimate ImageAgent scene produced violations: %+v", violations)
	}
}

func TestListingKitImageBoundaryProductionSelectionUsesTrackedTextRatherThanExtensions(t *testing.T) {
	for _, path := range []string{
		"scripts/upgrade.sql",
		"tools/runtime.mjs",
		"deployments/docker/Dockerfile.listingkit",
	} {
		if !isListingKitImageBoundaryProductionFile(path) {
			t.Errorf("isListingKitImageBoundaryProductionFile(%q) = false, want true", path)
		}
	}
	for _, path := range []string{
		"docs/architecture.md",
		"tests/fixture.sql",
		"internal/listingkit/generated/client.go",
		"internal/listingkit/testdata/fixture.json",
		"internal/listingkit/httpapi/handler_test.go",
	} {
		if isListingKitImageBoundaryProductionFile(path) {
			t.Errorf("isListingKitImageBoundaryProductionFile(%q) = true, want false", path)
		}
	}
}

func TestPhase3ListingKitImageWorkflowBoundaryAllowsImageAgentStudioScene(t *testing.T) {
	sources := []listingKitImageBoundarySource{
		{path: "internal/imageagent/policy/scene.go", text: `scene_style: "studio"`},
		{path: "internal/imageagent/prompts/keys.go", text: `const key = "productimage.studio_generation.subject"`},
	}
	if violations := findListingKitImageBoundaryViolations(sources); len(violations) != 0 {
		t.Fatalf("legitimate ImageAgent studio semantics produced violations: %+v", violations)
	}
}

func TestListingKitImageDependencyOwnershipAllowsNeutralPortsAndApprovedAdapters(t *testing.T) {
	sources := []listingKitImageBoundarySource{
		{path: "internal/listingkit/httpapi/chat.go", text: "package httpapi\nimport _ \"task-processor/internal/ai\""},
		{path: "internal/listingkit/approved_assets.go", text: "package listingkit\nimport _ \"task-processor/internal/product/asset\""},
		{path: "internal/listingkit/product_snapshot.go", text: "package listingkit\nimport _ \"task-processor/internal/product/catalog/canonical\""},
		{path: "internal/listingkit/httpapi/repositories.go", text: "package httpapi\nimport _ \"task-processor/internal/integration/persistence/product/asset\""},
		{path: "internal/listingkit/httpapi/object_store.go", text: "package httpapi\nimport _ \"task-processor/internal/integration/s3\""},
	}
	if violations := findListingKitImageBoundaryViolations(sources); len(violations) != 0 {
		t.Fatalf("neutral ListingKit dependencies were rejected: %v", violations)
	}
}

func trackedListingKitImageBoundarySources(t *testing.T) []listingKitImageBoundarySource {
	t.Helper()
	return trackedProductionTextSources(t, []string{"internal", "cmd", "config", "deployments", "scripts", "tools", "web/listingkit-ui/src"}, isListingKitImageBoundaryProductionFile)
}

func trackedPhase3ProductHardCutSources(t *testing.T) []listingKitImageBoundarySource {
	t.Helper()
	return trackedProductionTextSources(t, []string{"."}, isPhase3ProductHardCutProductionFile)
}

func selectPhase3ProductHardCutSources(sources []listingKitImageBoundarySource) []listingKitImageBoundarySource {
	selected := make([]listingKitImageBoundarySource, 0, len(sources))
	for _, source := range sources {
		if isPhase3ProductHardCutProductionFile(source.path) {
			selected = append(selected, source)
		}
	}
	return selected
}

func trackedProductionTextSources(t *testing.T, scopes []string, include func(string) bool) []listingKitImageBoundarySource {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	var sources []listingKitImageBoundarySource
	for _, scope := range scopes {
		for _, relative := range trackedFiles(t, scope) {
			relative = filepath.ToSlash(relative)
			if !include(relative) {
				continue
			}
			content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
			if os.IsNotExist(readErr) {
				continue
			}
			if readErr != nil {
				t.Fatalf("read %s: %v", relative, readErr)
			}
			if bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content) {
				continue
			}
			sources = append(sources, listingKitImageBoundarySource{path: relative, text: string(content)})
		}
	}
	return sources
}

func isPhase3ProductHardCutProductionFile(path string) bool {
	path = filepath.ToSlash(path)
	base := filepath.Base(path)
	if strings.HasPrefix(path, "tests/") || strings.HasPrefix(path, ".superpowers/") ||
		strings.HasPrefix(path, "generated/") || strings.Contains(path, "/generated/") ||
		strings.HasPrefix(path, "testdata/") || strings.Contains(path, "/testdata/") ||
		strings.EqualFold(filepath.Ext(base), ".md") || strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".test.") || strings.Contains(base, ".type-test.") {
		return false
	}
	if strings.HasPrefix(path, "docs/") {
		return strings.HasPrefix(path, "docs/api/")
	}
	return true
}

func isListingKitImageBoundaryProductionFile(path string) bool {
	path = filepath.ToSlash(path)
	base := filepath.Base(path)
	if strings.HasPrefix(path, "docs/") || strings.HasPrefix(path, "tests/") || strings.HasPrefix(path, ".superpowers/") ||
		strings.Contains(path, "/generated/") || strings.Contains(path, "/testdata/") ||
		strings.HasSuffix(base, "_test.go") || strings.Contains(base, ".test.") || strings.Contains(base, ".type-test.") {
		return false
	}
	return true
}

func findListingKitImageBoundaryViolations(sources []listingKitImageBoundarySource) []listingKitImageBoundaryViolation {
	rules := listingKitImageBoundaryRules()
	var violations []listingKitImageBoundaryViolation
	for _, source := range sources {
		path := filepath.ToSlash(source.path)
		if strings.HasPrefix(path, "internal/listingkit/") && strings.HasSuffix(path, ".go") {
			for _, imported := range listingKitDisallowedOwnedDependencyImports(source.text) {
				violations = append(violations, listingKitImageBoundaryViolation{
					rule: "ListingKit provider implementation ownership",
					path: path + " -> " + imported,
				})
			}
		}
		for _, rule := range rules {
			if rule.paths(path) && rule.match.MatchString(source.text) {
				violations = append(violations, listingKitImageBoundaryViolation{rule: rule.name, path: path})
			}
		}
	}
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].path == violations[j].path {
			return violations[i].rule < violations[j].rule
		}
		return violations[i].path < violations[j].path
	})
	return violations
}

var listingKitDependencyOwnershipBoundaries = []struct {
	namespace string
	allowed   map[string]struct{}
}{
	{
		namespace: "task-processor/internal/integration",
		allowed: map[string]struct{}{
			"task-processor/internal/integration/persistence/product/asset": {},
			"task-processor/internal/integration/s3":                        {},
		},
	},
	{
		namespace: "task-processor/internal/product",
		allowed: map[string]struct{}{
			"task-processor/internal/product/asset":             {},
			"task-processor/internal/product/catalog":           {},
			"task-processor/internal/product/catalog/canonical": {},
		},
	},
}

func listingKitDisallowedOwnedDependencyImports(source string) []string {
	parsed, err := parser.ParseFile(token.NewFileSet(), "listingkit.go", source, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	var forbidden []string
	for _, spec := range parsed.Imports {
		imported, err := decodeGoImportPath(spec.Path.Value)
		if err != nil {
			continue
		}
		for _, boundary := range listingKitDependencyOwnershipBoundaries {
			if !importMatchesPrefix(imported, boundary.namespace) {
				continue
			}
			_, allowed := boundary.allowed[imported]
			if !allowed {
				forbidden = append(forbidden, imported)
			}
			break
		}
	}
	return forbidden
}

func listingKitImageBoundaryRules() []listingKitImageBoundaryRule {
	all := func(string) bool { return true }
	listingKitOrConfig := func(path string) bool {
		return strings.HasPrefix(path, "internal/listingkit/") ||
			strings.HasPrefix(path, "internal/core/config/") ||
			strings.HasPrefix(path, "config/")
	}
	listingKit := func(path string) bool { return strings.HasPrefix(path, "internal/listingkit/") }
	notRetiredSubscriptionMigration := func(path string) bool {
		return path != "internal/listingsubscription/gorm_retired_module.go"
	}
	subscriptionWiring := func(path string) bool {
		return (strings.HasPrefix(path, "internal/listingsubscription/") && notRetiredSubscriptionMigration(path)) ||
			strings.HasPrefix(path, "internal/listingkit/api/") ||
			strings.HasPrefix(path, "internal/listingkit/httpapi/") ||
			path == "internal/listingkit/usage_settlement.go" ||
			strings.HasPrefix(path, "web/listingkit-ui/src/components/listingkit/subscription/") ||
			path == "web/listingkit-ui/src/lib/api/subscription.ts"
	}
	frontend := func(path string) bool { return strings.HasPrefix(path, "web/listingkit-ui/src/") }
	phase3ProductRuntime := func(path string) bool { return isPhase3ProductHardCutProductionFile(path) }
	notRetiredConfigRegistry := func(path string) bool {
		return listingKitOrConfig(path) && path != "internal/core/config/retired_product_runtime.go"
	}
	return []listingKitImageBoundaryRule{
		{name: "retired Studio route", paths: all, match: regexp.MustCompile(`(?i)(?:/api/v1/listing-kits/studio(?:[/\s?"']|$)|/api/listing-kits/studio(?:[/\s?"']|$)|path\s*\[\s*0\s*\]\s*===?\s*["']studio["'])`)},
		{name: "retired Studio data ownership", paths: all, match: regexp.MustCompile(`(?i)\b(?:listingkit_studio_[a-z0-9_]*|shein_studio_(?:sessions?|designs?)|SheinStudio[A-Za-z0-9_]*|shein_studio)\b`)},
		{name: "retired ListingKit image runtime", paths: notRetiredConfigRegistry, match: regexp.MustCompile(`\b(?:AIImageGenerator|AIAsyncImage[A-Za-z0-9_]*|StudioBackgroundRemover|CapabilityListingKitStudioImage|studioImageRoutingMode)\b`)},
		{name: "retired subscription module", paths: notRetiredSubscriptionMigration, match: regexp.MustCompile(`(?i)\b(?:ModuleStudio|studio_design_jobs_succeeded)\b|\b[A-Z0-9_]*module(?:_code|Code)?\b[^\r\n]{0,40}[:=]\s*["']?studio\b`)},
		{name: "retired subscription module", paths: subscriptionWiring, match: regexp.MustCompile(`["']studio["']`)},
		{name: "ListingKit to ImageAgent bridge", paths: listingKit, match: regexp.MustCompile(`["']task-processor/internal/imageagent(?:/[^"']*)?["']|\b(?:ImageAgentWorkspace|NewImageAgentAuthorizedAssetCatalog|imageAgentCatalogFromTask)[A-Za-z0-9_]*\b|/(?:api/v1/)?image-agent/runs\b|/image-agent-runs\b`)},
		{name: "ListingKit to ImageAgent bridge", paths: frontend, match: regexp.MustCompile(`\b(?:ImageAgentLaunchPanel|createImageAgentWorkspaceRun|getImageAgentWorkspaceAssets)\b|/image-agent-(?:runs|assets)\b`)},
		{name: "retired frontend image owner", paths: frontend, match: regexp.MustCompile(`(?i)(?:shein-studio|style-gallery|sds-workbench|image_nanobanana|image_background_removal)`)},
		{name: "retired product task API", paths: phase3ProductRuntime, match: regexp.MustCompile(`(?i)/api/v1/(?:products/(?:generate|tasks)|images/(?:process|tasks))(?:[/\s?:"']|$)`)},
		{name: "retired product task table", paths: phase3ProductRuntime, match: regexp.MustCompile(`\bproduct_(?:enrich|image)_tasks\b`)},
		{name: "retired product worker or queue", paths: phase3ProductRuntime, match: regexp.MustCompile(`(?i)\bproduct_(?:enrich|image)\b`)},
	}
}
