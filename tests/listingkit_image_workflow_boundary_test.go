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
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	var sources []listingKitImageBoundarySource
	for _, scope := range []string{"internal", "cmd", "config", "deployments", "scripts", "tools", "web/listingkit-ui/src"} {
		for _, relative := range trackedFiles(t, scope) {
			relative = filepath.ToSlash(relative)
			if !isListingKitImageBoundaryProductionFile(relative) {
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
	allowed   []string
}{
	{
		namespace: "task-processor/internal/integration",
		allowed: []string{
			"task-processor/internal/integration/persistence/product/asset",
			"task-processor/internal/integration/s3",
		},
	},
	{
		namespace: "task-processor/internal/product",
		allowed: []string{
			"task-processor/internal/product/asset",
			"task-processor/internal/product/catalog",
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
			allowed := false
			for _, root := range boundary.allowed {
				if importMatchesPrefix(imported, root) {
					allowed = true
					break
				}
			}
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
	}
}
