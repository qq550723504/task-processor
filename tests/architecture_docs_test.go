package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemporalBoundaryDocumentDefinesStableReviewRules(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "temporal-boundaries.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"# Temporal Boundaries",
		"HTTP API",
		"service facade",
		"workflow runtime",
		"RabbitMQ",
		"Review Questions",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so Temporal changes have a stable review boundary", path, phrase)
		}
	}
}

func TestProjectBoundaryDocumentKeepsRouteRegistrationInOwningHTTPAPI(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "project-boundaries.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	text := string(content)
	if !strings.Contains(text, "| API route registration | owning module `internal/*/httpapi` first; `internal/app/httpapi` only for shared runtime aggregation |") {
		t.Fatalf("%s must keep new route registration owned by module httpapi packages, with app/httpapi limited to shared aggregation", path)
	}
}

func TestArchitectureReviewChecklistCoversBoundaryRegressionRisks(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"reverse dependency",
		"internal/app/httpapi",
		"internal/app/processor",
		"internal/app/state",
		"owning module `internal/*/httpapi`",
		"Temporal",
		"platform-boundary-strategy.md",
		"historical-platform-migration-inventory.md",
		"external-client-boundary-inventory.md",
		"local interface",
		"concrete external client",
		"boundary exception",
		"import-boundary and architecture tests",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so architecture review catches common boundary regressions", path, phrase)
		}
	}
}

func TestNextTechnicalPrioritiesTracksImplementedBoundaryGuards(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "next-steps.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"Current guard coverage",
		"TestBusinessDomainsDoNotImportAppHTTPAPI",
		"TestInternalPackagesDoNotImportAppProcessorCompatibilityLayer",
		"TestInternalPackagesDoNotImportAppStateCompatibilityLayer",
		"TestAppHTTPAPIModuleBuildersStayAllowlisted",
		"TestAppHTTPAPIRouteDescriptorHelpersStayAllowlisted",
		"TestBusinessImplementationPackagesDoNotImportGinDirectly",
		"TestBusinessDomainsDoNotImportAppRuntimeAssembly",
		"TestPlatformModulesHistoricalImplementationImportsStayAllowlisted",
		"TestInfrastructurePackagesDoNotImportBusinessDomains",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so completed boundary guards do not drift back into open-ended priorities", path, phrase)
		}
	}
}

func TestPlatformBoundaryStrategyDefinesConvergenceRoles(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "platform-boundary-strategy.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"# Platform Boundary Strategy",
		"Historical platform packages",
		"`internal/publishing/*`",
		"`internal/listingkit`",
		"`internal/platforms/*`",
		"Migration Rules",
		"Review Questions",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so platform convergence has a stable review target", path, phrase)
		}
	}
}

func TestHistoricalPlatformMigrationInventoryDefinesCostSlices(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "historical-platform-migration-inventory.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"# Historical Platform Migration Inventory",
		"`internal/shein`",
		"`internal/temu`",
		"`internal/amazon`",
		"Cost Tiers",
		"Next Slice Candidates",
		"Non-goals",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so historical platform migration cost is reviewable", path, phrase)
		}
	}
}

func TestExternalClientBoundaryInventoryDefinesCouplingHotspots(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "external-client-boundary-inventory.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"# External Client Boundary Inventory",
		"`internal/infra/clients/management`",
		"`internal/infra/clients/openai`",
		"`internal/infra/clients/nanobanana`",
		"Hotspots",
		"`internal/listingkit`",
		"`internal/publishing/shein`",
		"`internal/shein`",
		"`internal/temu`",
		"Local Interface Rule",
		"Next Slice Candidates",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so external client coupling can be reduced intentionally", path, phrase)
		}
	}
}

func TestArchitectureReadmeIndexesStableBoundaryDocs(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "README.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"# Architecture Documentation",
		"Stable Boundary Documents",
		"project-boundaries.md",
		"httpapi-assembly-boundaries.md",
		"app-assembly-boundaries.md",
		"temporal-boundaries.md",
		"platform-boundary-strategy.md",
		"historical-platform-migration-inventory.md",
		"external-client-boundary-inventory.md",
		"compatibility-retirement.md",
		"architecture-review-checklist.md",
		"Plans, runbooks, and evaluations",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so stable architecture rules stay discoverable", path, phrase)
		}
	}
}

func TestAppAssemblyBoundaryDocumentDefinesStableAssemblyVocabulary(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "app-assembly-boundaries.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"# App Assembly Boundaries",
		"Assembly Vocabulary",
		"build / initialize",
		"register",
		"start",
		"coordinate",
		"`bootstrap` builds and registers",
		"`runner` starts and supervises",
		"`consumer` assembles and coordinates",
		"Review Questions",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so app-layer assembly changes keep the stable bootstrap/runner/consumer vocabulary", path, phrase)
		}
	}
}

func TestCompatibilityRetirementDocumentCapturesAppCompatibilityStatus(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "compatibility-retirement.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"# Compatibility Retirement",
		"`internal/app/processor`",
		"`internal/app/state`",
		"Retired",
		"zero in-repository imports",
		"TestInternalPackagesDoNotImportAppProcessorCompatibilityLayer",
		"TestAppStateCompatibilityLayerIsRetired",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so retired compatibility paths stay explicit", path, phrase)
		}
	}
}
