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
		"TestTemporalSDKImportsStayInRuntimeAndOrchestrationAdapters",
		"TestTemporalRuntimePackagesDoNotImportHTTPAPI",
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

func TestProjectBoundaryDocumentKeepsPreviewPlacementAlignedWithStablePreviewBoundary(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "project-boundaries.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"| Platform-neutral preview rules | `internal/listing/preview`; see `listing-preview-boundaries.md` |",
		"| Legacy preview facade / task-result aggregation | `internal/listingkit` during migration |",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so new preview code follows the stable preview boundary", path, phrase)
		}
	}
}

func TestProjectBoundaryDocumentTracksCurrentEnforcementTests(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "project-boundaries.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"Current Enforcement",
		"import-boundary tests",
		"TestBusinessDomainsDoNotImportAppHTTPAPI",
		"TestProjectBoundaryDomainsDoNotImportListingKitFacade",
		"TestListingKitSubdomainsDoNotImportRootFacade",
		"TestListingKitRootSheinWorkspaceBridgesDoNotImportWorkspaceDomainDirectly",
		"TestListingKitRootNonTestFilesDoNotImportWorkspaceDomainDirectly",
		"TestListingKitSheinWorkspaceBridgeDoesNotImportLegacyWorkspaceDomain",
		"TestInfrastructurePackagesDoNotImportBusinessDomains",
		"TestBusinessImplementationPackagesDoNotImportGinDirectly",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so project boundary enforcement reflects the active guard tests", path, phrase)
		}
	}
}

func TestHTTPAPIAssemblyBoundaryDocumentTracksGuardTests(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "httpapi-assembly-boundaries.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"# HTTP API Assembly Boundaries",
		"`cmd/* -> internal/app/httpapi -> internal/*/httpapi -> domain service/runtime/contracts`",
		"TestDomainHTTPPackagesDoNotImportAppHTTPAPI",
		"TestBusinessDomainsDoNotImportAppHTTPAPI",
		"TestAppHTTPAPIRootListingKitHelpersStayAllowlisted",
		"TestAppHTTPAPIModuleBuildersStayAllowlisted",
		"TestAppHTTPAPIRouteDescriptorHelpersStayAllowlisted",
		"TestAppHTTPAPIListingKitSupportImportsStayAllowlisted",
		"TestAppHTTPAPIListingKitRootImportsStayAllowlisted",
		"TestAppHTTPAPIListingKitHTTPAPIImportsStayAllowlisted",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so HTTP API assembly rules stay connected to guard tests", path, phrase)
		}
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
		"listing-preview-boundaries.md",
		"local interface",
		"concrete external client",
		"boundary exception",
		"import-boundary and architecture tests",
		"Current guard coverage",
		"TestBusinessDomainsDoNotImportAppHTTPAPI",
		"TestProjectBoundaryDomainsDoNotImportListingKitFacade",
		"TestListingKitSubdomainsDoNotImportRootFacade",
		"TestListingKitRootSheinWorkspaceBridgesDoNotImportWorkspaceDomainDirectly",
		"TestListingKitRootNonTestFilesDoNotImportWorkspaceDomainDirectly",
		"TestListingKitSheinWorkspaceBridgeDoesNotImportLegacyWorkspaceDomain",
		"TestInfrastructurePackagesDoNotImportBusinessDomains",
		"TestBusinessImplementationPackagesDoNotImportGinDirectly",
		"TestDomainHTTPPackagesDoNotImportAppHTTPAPI",
		"TestAppHTTPAPIRootListingKitHelpersStayAllowlisted",
		"TestAppHTTPAPIModuleBuildersStayAllowlisted",
		"TestAppHTTPAPIRouteDescriptorHelpersStayAllowlisted",
		"TestAppHTTPAPIListingKitSupportImportsStayAllowlisted",
		"TestAppHTTPAPIListingKitRootImportsStayAllowlisted",
		"TestAppHTTPAPIListingKitHTTPAPIImportsStayAllowlisted",
		"TestBusinessDomainsDoNotImportAppRuntimeAssembly",
		"TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages",
		"TestPlatformModulesHistoricalImplementationImportsStayAllowlisted",
		"TestPlatformRegistrationPackagesStayThin",
		"TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit",
		"TestPublishingSheinNonAPISheinImportsStayAllowlisted",
		"TestPublishingCommonUsesCanonicalPackage",
		"TestPublishingCommonDoesNotImportPlatformImplementations",
		"TestCmdContainsOnlyOfficialEntrypoints",
		"TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages",
		"TestHackContainsOnlyManagedSupportAreas",
		"TestTrackedLocalArtifactsStayOutOfProductionEntrypoints",
		"TestTrackedLocalArtifactsStayOutOfTools",
		"TestInternalPackagesDoNotImportAppProcessorCompatibilityLayer",
		"TestInternalPackagesDoNotImportAppStateCompatibilityLayer",
		"TestAppStateCompatibilityLayerIsRetired",
		"TestInfraProductCrawlerAdapterIsRetired",
		"TestAppCrawlerFetcherCompatibilityLayerIsRetired",
		"TestCmdPackagesDoNotImportAppCompatibilityLayers",
		"TestProductImageExternalClientImportsStayAllowlisted",
		"TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted",
		"TestPublishingSheinOpenAIImportsStayAllowlisted",
		"TestListingKitHTTPAPIExternalClientImportsStayAllowlisted",
		"TestListingKitRootOpenAIImportsStayAllowlisted",
		"TestTEMUSyncAndPricingManagementImportsStayAllowlisted",
		"TestTEMUProductStoreAndSchedulerManagementImportsStayAllowlisted",
		"TestTEMUOpenAIImportsStayAllowlisted",
		"TestTemporalSDKImportsStayInRuntimeAndOrchestrationAdapters",
		"TestTemporalRuntimePackagesDoNotImportHTTPAPI",
		"TestListingPreviewPackageStaysPlatformNeutral",
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
		"TestProjectBoundaryDomainsDoNotImportListingKitFacade",
		"TestListingKitSubdomainsDoNotImportRootFacade",
		"TestListingKitRootSheinWorkspaceBridgesDoNotImportWorkspaceDomainDirectly",
		"TestListingKitRootNonTestFilesDoNotImportWorkspaceDomainDirectly",
		"TestListingKitSheinWorkspaceBridgeDoesNotImportLegacyWorkspaceDomain",
		"TestInternalPackagesDoNotImportAppProcessorCompatibilityLayer",
		"TestInternalPackagesDoNotImportAppStateCompatibilityLayer",
		"TestAppHTTPAPIModuleBuildersStayAllowlisted",
		"TestAppHTTPAPIRouteDescriptorHelpersStayAllowlisted",
		"TestCmdContainsOnlyOfficialEntrypoints",
		"TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages",
		"TestHackContainsOnlyManagedSupportAreas",
		"TestTrackedLocalArtifactsStayOutOfProductionEntrypoints",
		"TestTrackedLocalArtifactsStayOutOfTools",
		"TestBusinessImplementationPackagesDoNotImportGinDirectly",
		"TestBusinessDomainsDoNotImportAppRuntimeAssembly",
		"TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages",
		"TestPlatformModulesHistoricalImplementationImportsStayAllowlisted",
		"TestPlatformRegistrationPackagesStayThin",
		"TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit",
		"TestPublishingSheinNonAPISheinImportsStayAllowlisted",
		"TestPublishingCommonUsesCanonicalPackage",
		"TestPublishingCommonDoesNotImportPlatformImplementations",
		"TestInfrastructurePackagesDoNotImportBusinessDomains",
		"TestProductImageExternalClientImportsStayAllowlisted",
		"TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted",
		"TestPublishingSheinOpenAIImportsStayAllowlisted",
		"TestListingKitHTTPAPIExternalClientImportsStayAllowlisted",
		"TestListingKitRootOpenAIImportsStayAllowlisted",
		"TestTEMUSyncAndPricingManagementImportsStayAllowlisted",
		"TestTEMUProductStoreAndSchedulerManagementImportsStayAllowlisted",
		"TestTEMUOpenAIImportsStayAllowlisted",
		"TestTemporalSDKImportsStayInRuntimeAndOrchestrationAdapters",
		"TestTemporalRuntimePackagesDoNotImportHTTPAPI",
		"TestListingPreviewPackageStaysPlatformNeutral",
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
		"TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit",
		"TestPublishingSheinNonAPISheinImportsStayAllowlisted",
		"TestPublishingCommonUsesCanonicalPackage",
		"TestPublishingCommonDoesNotImportPlatformImplementations",
		"TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages",
		"TestPlatformModulesHistoricalImplementationImportsStayAllowlisted",
		"TestPlatformRegistrationPackagesStayThin",
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
		"TestProductImageExternalClientImportsStayAllowlisted",
		"TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted",
		"TestPublishingSheinOpenAIImportsStayAllowlisted",
		"TestListingKitHTTPAPIExternalClientImportsStayAllowlisted",
		"TestListingKitRootOpenAIImportsStayAllowlisted",
		"TestTEMUSyncAndPricingManagementImportsStayAllowlisted",
		"TestTEMUProductStoreAndSchedulerManagementImportsStayAllowlisted",
		"TestTEMUOpenAIImportsStayAllowlisted",
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
		"listing-preview-boundaries.md",
		"architecture-review-checklist.md",
		"Development Boundary Documents",
		"docs/development/repository-structure.md",
		"Plans, runbooks, and evaluations",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so stable architecture rules stay discoverable", path, phrase)
		}
	}

	text := string(content)
	stableIndex := strings.Index(text, "listing-preview-boundaries.md")
	supportingIndex := strings.Index(text, "## Supporting Context")
	if stableIndex == -1 || supportingIndex == -1 || stableIndex > supportingIndex {
		t.Errorf("%s must list listing-preview-boundaries.md as a stable boundary document, not only supporting context", path)
	}

	repositoryIndex := strings.Index(text, "docs/development/repository-structure.md")
	plansIndex := strings.Index(text, "## Plans, runbooks, and evaluations")
	if repositoryIndex == -1 || plansIndex == -1 || repositoryIndex > plansIndex {
		t.Errorf("%s must list docs/development/repository-structure.md before plan/runbook context so repository layout rules stay discoverable", path)
	}
}

func TestRepositoryStructureDocumentTracksDirectoryGuardTests(t *testing.T) {
	path := filepath.Join("..", "docs", "development", "repository-structure.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"# Repository Structure",
		"`cmd/`",
		"`hack/`",
		"`tools/`",
		"TestCmdContainsOnlyOfficialEntrypoints",
		"TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages",
		"TestHackContainsOnlyManagedSupportAreas",
		"TestTrackedLocalArtifactsStayOutOfProductionEntrypoints",
		"TestTrackedLocalArtifactsStayOutOfTools",
		"TestPlatformRegistrationPackagesStayThin",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so repository layout rules stay connected to guard tests", path, phrase)
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
		"TestBusinessDomainsDoNotImportAppRuntimeAssembly",
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
		"`internal/infra/productcrawler`",
		"`internal/app/crawler/fetcher`",
		"TestInfraProductCrawlerAdapterIsRetired",
		"TestAppCrawlerFetcherCompatibilityLayerIsRetired",
		"TestCmdPackagesDoNotImportAppCompatibilityLayers",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so retired compatibility paths stay explicit", path, phrase)
		}
	}
}

func TestListingPreviewBoundaryDocumentTracksPlatformNeutralGuard(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "listing-preview-boundaries.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"# Listing Preview Boundaries",
		"`internal/listing/preview`",
		"`internal/listingkit`",
		"platform-neutral",
		"TestListingPreviewPackageStaysPlatformNeutral",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so preview extraction keeps a stable platform-neutral boundary", path, phrase)
		}
	}
}
