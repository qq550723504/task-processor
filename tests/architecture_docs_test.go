package tests

import (
	"os"
	"path/filepath"
	"regexp"
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
		"concrete Temporal worker bootstrap",
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
		"TestListingPreviewPackageStaysPlatformNeutral",
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
		"TestListingKitDoesNotImportLegacySheinRuntime",
		"TestListingKitDoesNotImportSheinAPIRoot",
		"TestListingKitNonAPISheinImportsStayAllowlisted",
		"TestListingKitAmazonListingImportsStayAllowlisted",
		"TestCatalogDoesNotDependOnProductEnrichAliases",
		"TestCanonicalTypesDoNotUseProductEnrichCompatibilityAliases",
		"TestSheinPipelineDoesNotImportListingKitFacade",
		"TestSheinSubmitPrepDoesNotImportListingKitTenantContext",
		"TestListingKitRootSheinHelpersStayAllowlisted",
		"TestListingKitRootServiceSubmitFilesStayAllowlisted",
		"TestListingKitRootTaskSubmissionFilesStayAllowlisted",
		"TestListingKitRootServiceGenerationFilesStayAllowlisted",
		"TestListingKitRootGenerationFilesStayAllowlisted",
		"TestListingPreviewPackageStaysPlatformNeutral",
		"TestTemporalSDKImportsStayInRuntimeAndOrchestrationAdapters",
		"TestTemporalRuntimePackagesDoNotImportHTTPAPI",
		"TestProductImageExternalClientImportsStayAllowlisted",
		"TestAmazonExternalClientImportsStayAllowlisted",
		"TestSheinBridgeExternalClientImportsStayAllowlisted",
		"TestSheinManagementClientImportsStayAllowlisted",
		"TestSheinOpenAIImportsStayAllowlisted",
		"TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted",
		"TestPublishingSheinOpenAIImportsStayAllowlisted",
		"TestPublishingSheinManagedAPIImportsStayAllowlisted",
		"TestPublishingSheinManagedManagementImportsStayAllowlisted",
		"TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit",
		"TestPublishingCommonUsesCanonicalPackage",
		"TestPublishingCommonDoesNotImportPlatformImplementations",
		"TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages",
		"TestPlatformModulesHistoricalImplementationImportsStayAllowlisted",
		"TestPlatformRegistrationPackagesStayThin",
		"TestPlatformRegistrationPackagesContainNoLocalArtifacts",
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
		"TestHTTPAPITypesDoesNotOwnRunOptions",
		"TestHTTPAPIModulesFileDoesNotOwnFeatureBuildWrappers",
		"TestHTTPAPIModulesFileDoesNotOwnBootstrapOrchestration",
		"TestHTTPAPIModulesFileDoesNotOwnLegacyBuildHandlersFacade",
		"TestHTTPAPIModulesFileDoesNotOwnWorkerRuntimeSupport",
		"TestHTTPAPIModulesFileDoesNotOwnLoginRuntimeSupport",
		"TestHTTPAPICompositionBuilderDoesNotOwnLoginBootstrapTypes",
		"TestHTTPAPICompositionBuilderDoesNotOwnLoginFeatureAssembly",
		"TestHTTPAPIRuntimeStateDoesNotOwnLoginBootstrapResultTypes",
		"TestHTTPAPIRuntimeStateDoesNotOwnFeatureHTTPAPIModuleTypes",
		"TestHTTPAPIRuntimeDepsMethodsDoNotOwnFeatureHTTPAPIModuleTypes",
		"TestHTTPModulesDoNotExposeFeatureHTTPAPIModuleTypesInSignatures",
		"TestHTTPAPIFeatureBuildersDoNotExposeFeatureHTTPAPIModuleTypesInSignatures",
		"TestFeatureModuleBuilderContractsReturnLocalModuleAliases",
		"TestHTTPAPIRuntimeStateDoesNotOwnSupportModuleResultTypes",
		"TestHTTPAPICompositionBuilderDoesNotOwnSupportModuleBuilderContracts",
		"TestHTTPAPICompositionBuilderDoesNotOwnSupportFeatureAssembly",
		"TestHTTPAPIModulesFileDoesNotOwnListingKitSDSRuntimeSupportHook",
		"TestHTTPAPICompositionBuilderDoesNotOwnProductImageRuntimeInputs",
		"TestHTTPAPICompositionBuilderDoesNotOwnAmazonListingRuntimeInput",
		"TestHTTPAPICompositionBuilderDoesNotOwnListingKitRuntimeInput",
		"TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated",
		"TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsOpenAIRuntimeAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsSharedResourceAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsRuntimeDepsMethodsDedicated",
		"TestHTTPAPIRuntimeKeepsPromptRuntimeAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsProductEnrichRuntimeAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsPathResolutionDedicated",
		"TestHTTPAPIRuntimeKeepsConfigLoadingDedicated",
		"TestHTTPAPIAdaptersKeepTaskRepositoryAssemblyDedicated",
		"TestHTTPAPIAdaptersKeepPromptStoreAssemblyDedicated",
		"TestBootstrapKeepsModelProviderAssemblyInDedicatedFile",
		"TestBootstrapKeepsLLMScorerAssemblyInDedicatedFile",
		"TestBootstrapKeepsAssetPublisherAssemblyInDedicatedFile",
		"TestBootstrapKeepsTaskRepositoryAssemblyInDedicatedFile",
		"TestBootstrapKeepsImagePipelineComponentAssemblyInDedicatedFile",
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
		"TestListingKitDoesNotImportLegacySheinRuntime",
		"TestListingKitDoesNotImportSheinAPIRoot",
		"TestListingKitNonAPISheinImportsStayAllowlisted",
		"TestListingKitAmazonListingImportsStayAllowlisted",
		"TestCatalogDoesNotDependOnProductEnrichAliases",
		"TestCanonicalTypesDoNotUseProductEnrichCompatibilityAliases",
		"TestSheinPipelineDoesNotImportListingKitFacade",
		"TestSheinSubmitPrepDoesNotImportListingKitTenantContext",
		"TestListingKitRootSheinHelpersStayAllowlisted",
		"TestListingKitRootServiceSubmitFilesStayAllowlisted",
		"TestListingKitRootTaskSubmissionFilesStayAllowlisted",
		"TestListingKitRootServiceGenerationFilesStayAllowlisted",
		"TestListingKitRootGenerationFilesStayAllowlisted",
		"TestInfrastructurePackagesDoNotImportBusinessDomains",
		"TestBusinessImplementationPackagesDoNotImportGinDirectly",
		"TestDomainHTTPPackagesDoNotImportAppHTTPAPI",
		"TestAppHTTPAPIRootListingKitHelpersStayAllowlisted",
		"TestAppHTTPAPIModuleBuildersStayAllowlisted",
		"TestAppHTTPAPIRouteDescriptorHelpersStayAllowlisted",
		"TestAppHTTPAPIListingKitSupportImportsStayAllowlisted",
		"TestAppHTTPAPIListingKitRootImportsStayAllowlisted",
		"TestAppHTTPAPIListingKitHTTPAPIImportsStayAllowlisted",
		"TestHTTPAPITypesDoesNotOwnRunOptions",
		"TestHTTPAPIModulesFileDoesNotOwnFeatureBuildWrappers",
		"TestHTTPAPIModulesFileDoesNotOwnBootstrapOrchestration",
		"TestHTTPAPIModulesFileDoesNotOwnLegacyBuildHandlersFacade",
		"TestHTTPAPIModulesFileDoesNotOwnWorkerRuntimeSupport",
		"TestHTTPAPIModulesFileDoesNotOwnLoginRuntimeSupport",
		"TestHTTPAPICompositionBuilderDoesNotOwnLoginBootstrapTypes",
		"TestHTTPAPICompositionBuilderDoesNotOwnLoginFeatureAssembly",
		"TestHTTPAPIRuntimeStateDoesNotOwnLoginBootstrapResultTypes",
		"TestHTTPAPIRuntimeStateDoesNotOwnFeatureHTTPAPIModuleTypes",
		"TestHTTPAPIRuntimeDepsMethodsDoNotOwnFeatureHTTPAPIModuleTypes",
		"TestHTTPModulesDoNotExposeFeatureHTTPAPIModuleTypesInSignatures",
		"TestHTTPAPIFeatureBuildersDoNotExposeFeatureHTTPAPIModuleTypesInSignatures",
		"TestFeatureModuleBuilderContractsReturnLocalModuleAliases",
		"TestHTTPAPIRuntimeStateDoesNotOwnSupportModuleResultTypes",
		"TestHTTPAPICompositionBuilderDoesNotOwnSupportModuleBuilderContracts",
		"TestHTTPAPICompositionBuilderDoesNotOwnSupportFeatureAssembly",
		"TestHTTPAPIModulesFileDoesNotOwnListingKitSDSRuntimeSupportHook",
		"TestHTTPAPICompositionBuilderDoesNotOwnProductImageRuntimeInputs",
		"TestHTTPAPICompositionBuilderDoesNotOwnAmazonListingRuntimeInput",
		"TestHTTPAPICompositionBuilderDoesNotOwnListingKitRuntimeInput",
		"TestBusinessDomainsDoNotImportAppRuntimeAssembly",
		"TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages",
		"TestPlatformModulesHistoricalImplementationImportsStayAllowlisted",
		"TestPlatformRegistrationPackagesStayThin",
		"TestPlatformRegistrationPackagesContainNoLocalArtifacts",
		"TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit",
		"TestPublishingSheinNonAPISheinImportsStayAllowlisted",
		"TestPublishingCommonUsesCanonicalPackage",
		"TestPublishingCommonDoesNotImportPlatformImplementations",
		"TestCmdContainsOnlyOfficialEntrypoints",
		"TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages",
		"TestCmdPackagesDoNotImportAppCompatibilityLayers",
		"TestHackContainsOnlyManagedSupportAreas",
		"TestHackSupportAreasContainNoLocalArtifacts",
		"TestTrackedLocalArtifactsStayOutOfProductionEntrypoints",
		"TestProductionEntrypointsContainNoLocalArtifacts",
		"TestTrackedLocalArtifactsStayOutOfTools",
		"TestToolsContainNoLocalArtifacts",
		"TestInternalPackagesContainNoLocalArtifacts",
		"TestSDSLoginRuntimeStateStaysOutOfInternalPackages",
		"TestInternalPackagesDoNotImportAppProcessorCompatibilityLayer",
		"TestAppProcessorCompatibilityLayerIsRetired",
		"TestInternalPackagesDoNotImportAppStateCompatibilityLayer",
		"TestAppStateCompatibilityLayerIsRetired",
		"TestInfraProductCrawlerAdapterIsRetired",
		"TestAppCrawlerFetcherCompatibilityLayerIsRetired",
		"TestCmdPackagesDoNotImportAppCompatibilityLayers",
		"TestProductImageExternalClientImportsStayAllowlisted",
		"TestAmazonExternalClientImportsStayAllowlisted",
		"TestSheinBridgeExternalClientImportsStayAllowlisted",
		"TestSheinManagementClientImportsStayAllowlisted",
		"TestSheinOpenAIImportsStayAllowlisted",
		"TestPublishingSheinManagedManagementImportsStayAllowlisted",
		"TestAppTaskManagementClientImportsStayAllowlisted",
		"TestAppRunnerManagementClientImportsStayAllowlisted",
		"TestAppConsumerManagementClientImportsStayAllowlisted",
		"TestAppBootstrapManagementClientImportsStayAllowlisted",
		"TestAppHTTPAPIManagementClientImportsStayAllowlisted",
		"TestAppRuntimeListingManagementClientImportsStayAllowlisted",
		"TestAppTaskStatusManagementClientImportsStayAllowlisted",
		"TestPlatformTaskManagementClientImportsStayAllowlisted",
		"TestStateManagementClientImportsStayAllowlisted",
		"TestPlatformBaseManagementClientImportsStayAllowlisted",
		"TestProcessorManagementClientImportsStayAllowlisted",
		"TestTaskRPCAPIManagementClientImportsStayAllowlisted",
		"TestSDSClientManagementClientImportsStayAllowlisted",
		"TestSheinLoginBootstrapManagementClientImportsStayAllowlisted",
		"TestSheinLoginServiceManagementClientImportsStayAllowlisted",
		"TestSheinLoginManagedManagementClientImportsStayAllowlisted",
		"TestSharedPricingManagementClientImportsStayAllowlisted",
		"TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted",
		"TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated",
		"TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsOpenAIRuntimeAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsSharedResourceAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsRuntimeDepsMethodsDedicated",
		"TestHTTPAPIRuntimeKeepsPromptRuntimeAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsProductEnrichRuntimeAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsPathResolutionDedicated",
		"TestHTTPAPIRuntimeKeepsConfigLoadingDedicated",
		"TestHTTPAPIAdaptersKeepTaskRepositoryAssemblyDedicated",
		"TestHTTPAPIAdaptersKeepPromptStoreAssemblyDedicated",
		"TestBootstrapKeepsModelProviderAssemblyInDedicatedFile",
		"TestBootstrapKeepsLLMScorerAssemblyInDedicatedFile",
		"TestBootstrapKeepsAssetPublisherAssemblyInDedicatedFile",
		"TestBootstrapKeepsTaskRepositoryAssemblyInDedicatedFile",
		"TestBootstrapKeepsImagePipelineComponentAssemblyInDedicatedFile",
		"TestPublishingSheinOpenAIImportsStayAllowlisted",
		"TestPublishingSheinManagedAPIImportsStayAllowlisted",
		"TestListingKitHTTPAPIExternalClientImportsStayAllowlisted",
		"TestListingKitHTTPAPIManagementClientImportsStayAllowlisted",
		"TestListingKitSheinSyncLegacyPromotionImportsStayAllowlisted",
		"TestListingKitRootOpenAIImportsStayAllowlisted",
		"TestListingKitRootDoesNotImportManagementAPI",
		"TestListingKitSupportFileStaysRetired",
		"TestPublishingSheinSubmitPrepUsesOnlySensitiveWordAdapter",
		"TestTEMUSyncAndPricingManagementImportsStayAllowlisted",
		"TestTEMUProductStoreAndSchedulerManagementImportsStayAllowlisted",
		"TestTEMURuntimeAndBridgeManagementImportsStayAllowlisted",
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

func TestArchitectureReviewChecklistReferencesCurrentGuardCoverageSource(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	reviewReferences := markdownSection(t, string(content), "## Review References")
	required := "docs/architecture/next-steps.md"
	if !strings.Contains(reviewReferences, required) {
		t.Fatalf("%s Review References must list %q so reviewers can find the current guard coverage baseline", path, required)
	}
}

func TestArchitectureReviewChecklistSeparatesStableRulesFromCurrentGuardBaseline(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	reviewReferences := markdownSection(t, string(content), "## Review References")
	required := []string{
		"stable source of truth for long-lived boundary rules",
		"current guard coverage baseline",
	}
	for _, phrase := range required {
		if !strings.Contains(reviewReferences, phrase) {
			t.Errorf("%s Review References must mention %q so review policy does not confuse stable rules with the current guard baseline", path, phrase)
		}
	}
}

func TestArchitectureReviewChecklistReferencesArchitectureIndexEntrypoints(t *testing.T) {
	readmePath := filepath.Join("..", "docs", "architecture", "README.md")
	readmeContent, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}

	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	reviewReferences := markdownSection(t, string(checklistContent), "## Review References")
	required := []string{"docs/architecture/README.md"}
	for _, heading := range []string{"## Stable Boundary Documents", "## Development Boundary Documents", "## Current Guard Baseline"} {
		required = append(required, normalizedArchitectureDocReferences(t, markdownSection(t, string(readmeContent), heading))...)
	}

	for _, reference := range required {
		if reference == "docs/architecture/architecture-review-checklist.md" {
			continue
		}
		if !strings.Contains(reviewReferences, reference) {
			t.Errorf("%s Review References must include %q from %s so the checklist stays aligned with the architecture index", checklistPath, reference, readmePath)
		}
	}
}

func TestArchitectureReviewChecklistPreservesArchitectureIndexReferenceOrder(t *testing.T) {
	readmePath := filepath.Join("..", "docs", "architecture", "README.md")
	readmeContent, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}

	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	reviewReferences := markdownSection(t, string(checklistContent), "## Review References")
	referenceListStart := strings.Index(reviewReferences, "- ")
	if referenceListStart == -1 {
		t.Fatalf("%s Review References must include a reference list", checklistPath)
	}
	referenceList := reviewReferences[referenceListStart:]
	required := []string{"docs/architecture/README.md"}
	for _, heading := range []string{"## Stable Boundary Documents", "## Development Boundary Documents", "## Current Guard Baseline"} {
		required = append(required, normalizedArchitectureDocReferences(t, markdownSection(t, string(readmeContent), heading))...)
	}

	lastIndex := -1
	for _, reference := range required {
		if reference == "docs/architecture/architecture-review-checklist.md" {
			continue
		}
		index := strings.Index(referenceList, reference)
		if index == -1 {
			t.Fatalf("%s Review References must include %q from %s before order can be checked", checklistPath, reference, readmePath)
		}
		if index < lastIndex {
			t.Errorf("%s Review References must preserve the %s order; %q appears out of order", checklistPath, readmePath, reference)
		}
		lastIndex = index
	}
}

func TestArchitectureReviewChecklistReferencesExistingDocuments(t *testing.T) {
	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	reviewReferences := markdownSection(t, string(checklistContent), "## Review References")
	const referenceRule = "Every review reference must resolve to an existing repository document"
	if !strings.Contains(reviewReferences, referenceRule) {
		t.Errorf("%s Review References must state %q so review entrypoints cannot drift into dead links", checklistPath, referenceRule)
	}

	assertMarkdownReferencesExistingDocuments(t, checklistPath, reviewReferences)
}

func TestArchitectureReviewChecklistReferencesAreUnique(t *testing.T) {
	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	reviewReferences := markdownSection(t, string(checklistContent), "## Review References")
	const uniqueRule = "Review references must not contain duplicate document entries"
	if !strings.Contains(reviewReferences, uniqueRule) {
		t.Errorf("%s Review References must state %q so formal review entrypoints remain a clear set", checklistPath, uniqueRule)
	}

	referenceListStart := strings.Index(reviewReferences, "- ")
	if referenceListStart == -1 {
		t.Fatalf("%s Review References must include a reference list", checklistPath)
	}
	referenceList := reviewReferences[referenceListStart:]
	seen := map[string]bool{}
	for _, reference := range normalizedArchitectureDocReferences(t, referenceList) {
		if seen[reference] {
			t.Errorf("%s Review References must not list %q more than once", checklistPath, reference)
		}
		seen[reference] = true
	}
}

func TestArchitectureReviewChecklistKeepsDocumentPathsInReferenceList(t *testing.T) {
	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	reviewReferences := markdownSection(t, string(checklistContent), "## Review References")
	const listOnlyRule = "Document paths in Review References must be listed only in the reference list"
	if !strings.Contains(reviewReferences, listOnlyRule) {
		t.Errorf("%s Review References must state %q so explanatory prose does not become an implicit reference list", checklistPath, listOnlyRule)
	}

	referenceListStart := strings.Index(reviewReferences, "- ")
	if referenceListStart == -1 {
		t.Fatalf("%s Review References must include a reference list", checklistPath)
	}
	prose := reviewReferences[:referenceListStart]
	referencePattern := regexp.MustCompile("`([^`]+\\.md)`")
	if matches := referencePattern.FindAllStringSubmatch(prose, -1); len(matches) > 0 {
		for _, match := range matches {
			t.Errorf("%s Review References prose must not contain document path %q; put formal paths in the reference list", checklistPath, match[1])
		}
	}
}

func TestArchitectureReviewChecklistExcludesTimeBoundedReviewReferences(t *testing.T) {
	readmePath := filepath.Join("..", "docs", "architecture", "README.md")
	readmeContent, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}

	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	reviewReferences := markdownSection(t, string(checklistContent), "## Review References")
	const timeBoundedRule = "Time-bounded plans, runbooks, and evaluations must not be listed as review references"
	if !strings.Contains(reviewReferences, timeBoundedRule) {
		t.Errorf("%s Review References must state %q so temporary context documents do not become formal review entrypoints", checklistPath, timeBoundedRule)
	}

	patterns := architectureReadmeTimeBoundedPatterns(t, string(readmeContent))
	for _, reference := range normalizedArchitectureDocReferences(t, reviewReferences) {
		name := filepath.Base(reference)
		if matchesAnyPattern(t, name, patterns) {
			t.Errorf("%s Review References must not include time-bounded context document %q matching patterns %v", checklistPath, reference, patterns)
		}
	}
}

func TestArchitectureReviewChecklistExcludesSupportingContextReferences(t *testing.T) {
	readmePath := filepath.Join("..", "docs", "architecture", "README.md")
	readmeContent, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}

	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	reviewReferences := markdownSection(t, string(checklistContent), "## Review References")
	const supportingContextRule = "Supporting context documents must not be listed as review references unless promoted into stable boundary documents"
	if !strings.Contains(reviewReferences, supportingContextRule) {
		t.Errorf("%s Review References must state %q so background documents do not become formal review entrypoints", checklistPath, supportingContextRule)
	}

	supportingContext := markdownSection(t, string(readmeContent), "## Supporting Context")
	for _, reference := range normalizedArchitectureDocReferences(t, supportingContext) {
		if strings.Contains(reviewReferences, reference) {
			t.Errorf("%s Review References must not include supporting context document %q from %s", checklistPath, reference, readmePath)
		}
	}
}

func TestArchitectureReviewChecklistReferencesOnlyArchitectureIndexEntrypoints(t *testing.T) {
	readmePath := filepath.Join("..", "docs", "architecture", "README.md")
	readmeContent, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}

	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	reviewReferences := markdownSection(t, string(checklistContent), "## Review References")
	const indexedEntrypointsRule = "Every review reference must come from the architecture index stable, development, or current guard baseline sections"
	if !strings.Contains(reviewReferences, indexedEntrypointsRule) {
		t.Errorf("%s Review References must state %q so ad-hoc documents cannot become formal review entrypoints", checklistPath, indexedEntrypointsRule)
	}

	allowed := map[string]bool{"docs/architecture/README.md": true}
	for _, heading := range []string{"## Stable Boundary Documents", "## Development Boundary Documents", "## Current Guard Baseline"} {
		for _, reference := range normalizedArchitectureDocReferences(t, markdownSection(t, string(readmeContent), heading)) {
			allowed[reference] = true
		}
	}

	for _, reference := range normalizedArchitectureDocReferences(t, reviewReferences) {
		if !allowed[reference] {
			t.Errorf("%s Review References includes %q, but it is not indexed in %s as a stable, development, or current guard baseline entrypoint", checklistPath, reference, readmePath)
		}
	}
}

func TestArchitectureReviewChecklistExplainsReviewReferenceRoles(t *testing.T) {
	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	reviewReferences := markdownSection(t, string(checklistContent), "## Review References")
	required := []string{
		"stable source of truth for long-lived boundary rules",
		"current guard coverage baseline",
		"development boundary documents define long-lived repository structure rules",
	}
	for _, phrase := range required {
		if !strings.Contains(reviewReferences, phrase) {
			t.Errorf("%s Review References must explain %q so reviewers know how to use each entrypoint class", checklistPath, phrase)
		}
	}
}

func TestArchitectureReviewChecklistKeepsContextDocumentsOutOfReviewPolicy(t *testing.T) {
	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	workingRule := markdownSection(t, string(checklistContent), "## Working Rule")
	required := []string{
		"plans, runbooks, or contextual notes",
		"copied or linked into a stable boundary document",
		"before being used as review policy",
	}
	for _, phrase := range required {
		if !strings.Contains(workingRule, phrase) {
			t.Errorf("%s Working Rule must mention %q so contextual documents cannot silently become review policy", checklistPath, phrase)
		}
	}
}

func TestArchitectureReviewChecklistCoversRepositoryStructureRules(t *testing.T) {
	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	requiredChecks := markdownSection(t, string(checklistContent), "## Required Checks")
	required := []string{
		"docs/development/repository-structure.md",
		"`cmd/`",
		"official entrypoints",
		"`hack/`",
		"managed support areas",
		"local artifacts",
	}
	for _, phrase := range required {
		if !strings.Contains(requiredChecks, phrase) {
			t.Errorf("%s Required Checks must mention %q so repository layout rules are reviewed with architecture changes", checklistPath, phrase)
		}
	}
}

func TestArchitectureReviewChecklistRequiresGuardBaselineUpdates(t *testing.T) {
	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	requiredChecks := markdownSection(t, string(checklistContent), "## Required Checks")
	required := []string{
		"docs/architecture/next-steps.md",
		"Current guard coverage",
		"guard baseline",
	}
	for _, phrase := range required {
		if !strings.Contains(requiredChecks, phrase) {
			t.Errorf("%s Required Checks must mention %q so guard baseline changes stay aligned with review actions", checklistPath, phrase)
		}
	}
}

func TestArchitectureReviewChecklistGuardBaselineStaysSubsetOfCurrentCoverage(t *testing.T) {
	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	checklistContent, err := os.ReadFile(checklistPath)
	if err != nil {
		t.Fatalf("read %s: %v", checklistPath, err)
	}

	nextStepsPath := filepath.Join("..", "docs", "architecture", "next-steps.md")
	nextStepsContent, err := os.ReadFile(nextStepsPath)
	if err != nil {
		t.Fatalf("read %s: %v", nextStepsPath, err)
	}

	guardBaseline := markdownSection(t, string(checklistContent), "## Guard Baseline")
	const subsetRule = "Representative guard references must remain a subset of the current guard coverage baseline"
	if !strings.Contains(guardBaseline, subsetRule) {
		t.Errorf("%s Guard Baseline must state %q so checklist guard examples cannot drift from %s", checklistPath, subsetRule, nextStepsPath)
	}

	currentGuardCoverage := markdownBlockBetween(t, string(nextStepsContent), "Current guard coverage:", "后续重点")
	guardPattern := regexp.MustCompile("`(Test[A-Za-z0-9_]+)`")
	for _, match := range guardPattern.FindAllStringSubmatch(guardBaseline, -1) {
		if !strings.Contains(currentGuardCoverage, match[1]) {
			t.Errorf("%s Guard Baseline references %q, but %s Current guard coverage does not include it", checklistPath, match[1], nextStepsPath)
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
		"TestListingKitDoesNotImportLegacySheinRuntime",
		"TestListingKitDoesNotImportSheinAPIRoot",
		"TestListingKitNonAPISheinImportsStayAllowlisted",
		"TestListingKitAmazonListingImportsStayAllowlisted",
		"TestCatalogDoesNotDependOnProductEnrichAliases",
		"TestCanonicalTypesDoNotUseProductEnrichCompatibilityAliases",
		"TestSheinPipelineDoesNotImportListingKitFacade",
		"TestSheinSubmitPrepDoesNotImportListingKitTenantContext",
		"TestListingKitRootSheinHelpersStayAllowlisted",
		"TestListingKitRootServiceSubmitFilesStayAllowlisted",
		"TestListingKitRootTaskSubmissionFilesStayAllowlisted",
		"TestListingKitRootServiceGenerationFilesStayAllowlisted",
		"TestListingKitRootGenerationFilesStayAllowlisted",
		"TestInternalPackagesDoNotImportAppProcessorCompatibilityLayer",
		"TestAppProcessorCompatibilityLayerIsRetired",
		"TestInternalPackagesDoNotImportAppStateCompatibilityLayer",
		"TestAppStateCompatibilityLayerIsRetired",
		"TestInfraProductCrawlerAdapterIsRetired",
		"TestAppCrawlerFetcherCompatibilityLayerIsRetired",
		"TestCmdPackagesDoNotImportAppCompatibilityLayers",
		"TestDomainHTTPPackagesDoNotImportAppHTTPAPI",
		"TestAppHTTPAPIRootListingKitHelpersStayAllowlisted",
		"TestAppHTTPAPIListingKitSupportImportsStayAllowlisted",
		"TestAppHTTPAPIListingKitRootImportsStayAllowlisted",
		"TestAppHTTPAPIListingKitHTTPAPIImportsStayAllowlisted",
		"TestAppHTTPAPIModuleBuildersStayAllowlisted",
		"TestAppHTTPAPIRouteDescriptorHelpersStayAllowlisted",
		"TestHTTPAPITypesDoesNotOwnRunOptions",
		"TestHTTPAPIModulesFileDoesNotOwnFeatureBuildWrappers",
		"TestHTTPAPIModulesFileDoesNotOwnBootstrapOrchestration",
		"TestHTTPAPIModulesFileDoesNotOwnLegacyBuildHandlersFacade",
		"TestHTTPAPIModulesFileDoesNotOwnWorkerRuntimeSupport",
		"TestHTTPAPIModulesFileDoesNotOwnLoginRuntimeSupport",
		"TestHTTPAPICompositionBuilderDoesNotOwnLoginBootstrapTypes",
		"TestHTTPAPICompositionBuilderDoesNotOwnLoginFeatureAssembly",
		"TestHTTPAPIRuntimeStateDoesNotOwnLoginBootstrapResultTypes",
		"TestHTTPAPIRuntimeStateDoesNotOwnFeatureHTTPAPIModuleTypes",
		"TestHTTPAPIRuntimeDepsMethodsDoNotOwnFeatureHTTPAPIModuleTypes",
		"TestHTTPModulesDoNotExposeFeatureHTTPAPIModuleTypesInSignatures",
		"TestHTTPAPIFeatureBuildersDoNotExposeFeatureHTTPAPIModuleTypesInSignatures",
		"TestFeatureModuleBuilderContractsReturnLocalModuleAliases",
		"TestHTTPAPIRuntimeStateDoesNotOwnSupportModuleResultTypes",
		"TestHTTPAPICompositionBuilderDoesNotOwnSupportModuleBuilderContracts",
		"TestHTTPAPICompositionBuilderDoesNotOwnSupportFeatureAssembly",
		"TestHTTPAPIModulesFileDoesNotOwnListingKitSDSRuntimeSupportHook",
		"TestHTTPAPICompositionBuilderDoesNotOwnProductImageRuntimeInputs",
		"TestHTTPAPICompositionBuilderDoesNotOwnAmazonListingRuntimeInput",
		"TestHTTPAPICompositionBuilderDoesNotOwnListingKitRuntimeInput",
		"TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated",
		"TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsOpenAIRuntimeAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsSharedResourceAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsRuntimeDepsMethodsDedicated",
		"TestHTTPAPIRuntimeKeepsPromptRuntimeAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsProductEnrichRuntimeAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsPathResolutionDedicated",
		"TestHTTPAPIRuntimeKeepsConfigLoadingDedicated",
		"TestHTTPAPIAdaptersKeepTaskRepositoryAssemblyDedicated",
		"TestHTTPAPIAdaptersKeepPromptStoreAssemblyDedicated",
		"TestBootstrapKeepsModelProviderAssemblyInDedicatedFile",
		"TestBootstrapKeepsLLMScorerAssemblyInDedicatedFile",
		"TestBootstrapKeepsAssetPublisherAssemblyInDedicatedFile",
		"TestBootstrapKeepsTaskRepositoryAssemblyInDedicatedFile",
		"TestBootstrapKeepsImagePipelineComponentAssemblyInDedicatedFile",
		"TestCmdContainsOnlyOfficialEntrypoints",
		"TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages",
		"TestHackContainsOnlyManagedSupportAreas",
		"TestHackSupportAreasContainNoLocalArtifacts",
		"TestTrackedLocalArtifactsStayOutOfProductionEntrypoints",
		"TestProductionEntrypointsContainNoLocalArtifacts",
		"TestTrackedLocalArtifactsStayOutOfTools",
		"TestToolsContainNoLocalArtifacts",
		"TestInternalPackagesContainNoLocalArtifacts",
		"TestSDSLoginRuntimeStateStaysOutOfInternalPackages",
		"TestBusinessImplementationPackagesDoNotImportGinDirectly",
		"TestBusinessDomainsDoNotImportAppRuntimeAssembly",
		"TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages",
		"TestPlatformModulesHistoricalImplementationImportsStayAllowlisted",
		"TestPlatformRegistrationPackagesStayThin",
		"TestPlatformRegistrationPackagesContainNoLocalArtifacts",
		"TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit",
		"TestPublishingSheinNonAPISheinImportsStayAllowlisted",
		"TestPublishingCommonUsesCanonicalPackage",
		"TestPublishingSheinSubmitPrepUsesOnlySensitiveWordAdapter",
		"TestPublishingSheinManagedAPIImportsStayAllowlisted",
		"TestPublishingSheinManagedManagementImportsStayAllowlisted",
		"TestPublishingCommonDoesNotImportPlatformImplementations",
		"TestInfrastructurePackagesDoNotImportBusinessDomains",
		"TestProductImageExternalClientImportsStayAllowlisted",
		"TestAmazonExternalClientImportsStayAllowlisted",
		"TestSheinBridgeExternalClientImportsStayAllowlisted",
		"TestSheinManagementClientImportsStayAllowlisted",
		"TestSheinOpenAIImportsStayAllowlisted",
		"TestAppTaskManagementClientImportsStayAllowlisted",
		"TestAppRunnerManagementClientImportsStayAllowlisted",
		"TestAppConsumerManagementClientImportsStayAllowlisted",
		"TestAppBootstrapManagementClientImportsStayAllowlisted",
		"TestAppHTTPAPIManagementClientImportsStayAllowlisted",
		"TestAppRuntimeListingManagementClientImportsStayAllowlisted",
		"TestAppTaskStatusManagementClientImportsStayAllowlisted",
		"TestPlatformTaskManagementClientImportsStayAllowlisted",
		"TestStateManagementClientImportsStayAllowlisted",
		"TestPlatformBaseManagementClientImportsStayAllowlisted",
		"TestProcessorManagementClientImportsStayAllowlisted",
		"TestTaskRPCAPIManagementClientImportsStayAllowlisted",
		"TestSDSClientManagementClientImportsStayAllowlisted",
		"TestSheinLoginBootstrapManagementClientImportsStayAllowlisted",
		"TestSheinLoginServiceManagementClientImportsStayAllowlisted",
		"TestSheinLoginManagedManagementClientImportsStayAllowlisted",
		"TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted",
		"TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated",
		"TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsOpenAIRuntimeAssemblyDedicated",
		"TestPublishingSheinOpenAIImportsStayAllowlisted",
		"TestPublishingSheinManagedManagementImportsStayAllowlisted",
		"TestListingKitHTTPAPIExternalClientImportsStayAllowlisted",
		"TestListingKitHTTPAPIManagementClientImportsStayAllowlisted",
		"TestListingKitSheinSyncLegacyPromotionImportsStayAllowlisted",
		"TestListingKitRootOpenAIImportsStayAllowlisted",
		"TestListingKitRootDoesNotImportManagementAPI",
		"TestListingKitSupportFileStaysRetired",
		"TestSharedPricingManagementClientImportsStayAllowlisted",
		"TestTEMUSyncAndPricingManagementImportsStayAllowlisted",
		"TestTEMUProductStoreAndSchedulerManagementImportsStayAllowlisted",
		"TestTEMURuntimeAndBridgeManagementImportsStayAllowlisted",
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

func TestNextTechnicalPrioritiesCurrentGuardCoveragePointsToReviewEntrypoints(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "next-steps.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	currentGuardCoverage := markdownBlockBetween(t, string(content), "Current guard coverage:", "后续重点")
	required := []string{
		"docs/architecture/README.md",
		"docs/architecture/architecture-review-checklist.md",
	}
	for _, phrase := range required {
		if !strings.Contains(currentGuardCoverage, phrase) {
			t.Errorf("%s Current guard coverage must mention %q so the active guard baseline points back to the review entrypoints", path, phrase)
		}
	}
}

func TestNextTechnicalPrioritiesReferencesExistingDocuments(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "next-steps.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	currentGuardCoverage := markdownBlockBetween(t, string(content), "Current guard coverage:", "后续重点")
	const referenceRule = "Every next-step reference must resolve to an existing repository document"
	if !strings.Contains(currentGuardCoverage, referenceRule) {
		t.Errorf("%s Current guard coverage must state %q so active baseline links cannot drift into dead references", path, referenceRule)
	}

	assertMarkdownReferencesExistingDocuments(t, path, currentGuardCoverage)
}

func TestNextTechnicalPrioritiesCurrentGuardCoverageReferencesImplementedTests(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "next-steps.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	currentGuardCoverage := markdownBlockBetween(t, string(content), "Current guard coverage:", "后续重点")
	const implementedTestRule = "Every guard listed in current coverage must resolve to an implemented test function"
	if !strings.Contains(currentGuardCoverage, implementedTestRule) {
		t.Errorf("%s Current guard coverage must state %q so the active baseline cannot drift into phantom test names", path, implementedTestRule)
	}

	implementedTests := implementedTestNames(t)
	guardPattern := regexp.MustCompile("`(Test[A-Za-z0-9_]+)`")
	for _, match := range guardPattern.FindAllStringSubmatch(currentGuardCoverage, -1) {
		if !implementedTests[match[1]] {
			t.Errorf("%s Current guard coverage references %q, but no matching test function exists under tests/", path, match[1])
		}
	}
}

func markdownSection(t *testing.T, content, heading string) string {
	t.Helper()

	start := strings.Index(content, heading)
	if start == -1 {
		t.Fatalf("markdown content must include heading %q", heading)
	}

	section := content[start+len(heading):]
	if nextHeading := strings.Index(section, "\n## "); nextHeading != -1 {
		section = section[:nextHeading]
	}
	return section
}

func normalizedArchitectureDocReferences(t *testing.T, content string) []string {
	t.Helper()

	referencePattern := regexp.MustCompile("`([^`]+\\.md)`")
	matches := referencePattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		t.Fatalf("markdown content must include at least one doc reference")
	}

	references := make([]string, 0, len(matches))
	for _, match := range matches {
		reference := match[1]
		if !strings.HasPrefix(reference, "docs/") {
			reference = "docs/architecture/" + reference
		}
		references = append(references, reference)
	}
	return references
}

func implementedTestNames(t *testing.T) map[string]bool {
	t.Helper()

	testNames := make(map[string]bool)
	testFuncPattern := regexp.MustCompile(`func (Test[A-Za-z0-9_]+)\(`)
	repoRoot := filepath.Join("..")
	err := filepath.WalkDir(repoRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range testFuncPattern.FindAllStringSubmatch(string(content), -1) {
			testNames[match[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("collect implemented test names: %v", err)
	}

	if len(testNames) == 0 {
		t.Fatalf("expected to find implemented test functions under repository root %s", repoRoot)
	}
	return testNames
}

func architectureReadmeExplicitReferences(t *testing.T, content string) map[string]bool {
	t.Helper()

	references := make(map[string]bool)
	for _, reference := range normalizedArchitectureDocReferences(t, content) {
		name := filepath.Base(reference)
		if !strings.Contains(name, "*") {
			references[name] = true
		}
	}
	return references
}

func architectureReadmeTimeBoundedPatterns(t *testing.T, content string) []string {
	t.Helper()

	plansRunbooksAndEvaluations := markdownSection(t, content, "## Plans, runbooks, and evaluations")
	referencePattern := regexp.MustCompile("`(\\*-[^`]+\\.md)`")
	matches := referencePattern.FindAllStringSubmatch(plansRunbooksAndEvaluations, -1)
	if len(matches) == 0 {
		t.Fatalf("Plans, runbooks, and evaluations must include time-bounded context patterns")
	}

	patterns := make([]string, 0, len(matches))
	for _, match := range matches {
		patterns = append(patterns, match[1])
	}
	return patterns
}

func matchesAnyPattern(t *testing.T, name string, patterns []string) bool {
	t.Helper()

	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			t.Fatalf("invalid time-bounded context pattern %q: %v", pattern, err)
		}
		if matched {
			return true
		}
	}
	return false
}

func markdownBlockBetween(t *testing.T, content, startMarker, endMarker string) string {
	t.Helper()

	start := strings.Index(content, startMarker)
	if start == -1 {
		t.Fatalf("markdown content must include start marker %q", startMarker)
	}

	block := content[start+len(startMarker):]
	end := strings.Index(block, endMarker)
	if end == -1 {
		t.Fatalf("markdown content after %q must include end marker %q", startMarker, endMarker)
	}
	return block[:end]
}

func TestNextTechnicalPrioritiesTracksEveryImportBoundaryGuard(t *testing.T) {
	nextStepsPath := filepath.Join("..", "docs", "architecture", "next-steps.md")
	assertDocumentTracksEveryImportBoundaryGuard(t, nextStepsPath, "Current guard coverage so active import-boundary guards stay visible to reviewers")
}

func TestArchitectureReviewChecklistTracksEveryImportBoundaryGuard(t *testing.T) {
	checklistPath := filepath.Join("..", "docs", "architecture", "architecture-review-checklist.md")
	assertDocumentTracksEveryImportBoundaryGuard(t, checklistPath, "Guard Baseline so architecture review covers every active import-boundary guard")
}

func assertDocumentTracksEveryImportBoundaryGuard(t *testing.T, documentPath string, sectionDescription string) {
	t.Helper()

	documentContent, err := os.ReadFile(documentPath)
	if err != nil {
		t.Fatalf("read %s: %v", documentPath, err)
	}

	testContent, err := os.ReadFile(filepath.Join(".", "import_boundaries_test.go"))
	if err != nil {
		t.Fatalf("read import_boundaries_test.go: %v", err)
	}

	testNamePattern := regexp.MustCompile(`(?m)^func (Test\w+)`)
	for _, match := range testNamePattern.FindAllStringSubmatch(string(testContent), -1) {
		testName := match[1]
		if !strings.Contains(string(documentContent), testName) {
			t.Errorf("%s must list %s in %s", documentPath, testName, sectionDescription)
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
		"TestPlatformRegistrationPackagesContainNoLocalArtifacts",
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
		"TestPlatformRegistrationPackagesStayThin",
		"TestPlatformRegistrationPackagesContainNoLocalArtifacts",
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
		"management retirement target",
		"in-repository database/repository access",
		"freeze current seams",
		"`internal/infra/clients/openai`",
		"`internal/infra/clients/nanobanana`",
		"Hotspots",
		"`internal/listingkit`",
		"`internal/publishing/shein`",
		"`internal/shein`",
		"`internal/sheinbridge`",
		"`internal/temu`",
		"`internal/pricing`",
		"Local Interface Rule",
		"Next Slice Candidates",
		"TestProductImageExternalClientImportsStayAllowlisted",
		"TestAmazonExternalClientImportsStayAllowlisted",
		"TestSheinBridgeExternalClientImportsStayAllowlisted",
		"TestSheinManagementClientImportsStayAllowlisted",
		"TestSheinOpenAIImportsStayAllowlisted",
		"TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted",
		"TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated",
		"TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsOpenAIRuntimeAssemblyDedicated",
		"TestPublishingSheinOpenAIImportsStayAllowlisted",
		"TestPublishingSheinManagedAPIImportsStayAllowlisted",
		"TestPublishingSheinManagedManagementImportsStayAllowlisted",
		"TestListingKitHTTPAPIExternalClientImportsStayAllowlisted",
		"TestListingKitHTTPAPIManagementClientImportsStayAllowlisted",
		"TestListingKitSheinSyncLegacyPromotionImportsStayAllowlisted",
		"TestListingKitRootOpenAIImportsStayAllowlisted",
		"TestAppTaskManagementClientImportsStayAllowlisted",
		"TestAppRunnerManagementClientImportsStayAllowlisted",
		"TestAppConsumerManagementClientImportsStayAllowlisted",
		"TestAppBootstrapManagementClientImportsStayAllowlisted",
		"TestAppHTTPAPIManagementClientImportsStayAllowlisted",
		"TestAppRuntimeListingManagementClientImportsStayAllowlisted",
		"TestAppTaskStatusManagementClientImportsStayAllowlisted",
		"TestPlatformTaskManagementClientImportsStayAllowlisted",
		"TestStateManagementClientImportsStayAllowlisted",
		"TestPlatformBaseManagementClientImportsStayAllowlisted",
		"TestProcessorManagementClientImportsStayAllowlisted",
		"TestTaskRPCAPIManagementClientImportsStayAllowlisted",
		"TestSDSClientManagementClientImportsStayAllowlisted",
		"TestSheinLoginBootstrapManagementClientImportsStayAllowlisted",
		"TestSheinLoginServiceManagementClientImportsStayAllowlisted",
		"TestSheinLoginManagedManagementClientImportsStayAllowlisted",
		"TestSharedPricingManagementClientImportsStayAllowlisted",
		"TestTEMUSyncAndPricingManagementImportsStayAllowlisted",
		"TestTEMUProductStoreAndSchedulerManagementImportsStayAllowlisted",
		"TestTEMURuntimeAndBridgeManagementImportsStayAllowlisted",
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

func TestArchitectureReadmeIndexesCurrentGuardCoverageBaseline(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "README.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"Current Guard Baseline",
		"next-steps.md",
		"Current guard coverage",
		"guard coverage baseline",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so the architecture index points reviewers to the active guard baseline", path, phrase)
		}
	}
}

func TestArchitectureReadmeCurrentGuardBaselinePointsToReviewEntrypoints(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "README.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	currentGuardBaseline := markdownSection(t, string(content), "## Current Guard Baseline")
	required := []string{
		"docs/architecture/next-steps.md",
		"docs/architecture/architecture-review-checklist.md",
	}
	for _, phrase := range required {
		if !strings.Contains(currentGuardBaseline, phrase) {
			t.Errorf("%s Current Guard Baseline must mention %q so the architecture index uses the same review entrypoint paths as the checklist", path, phrase)
		}
	}
}

func TestArchitectureReadmeClassifiesTimeBoundedContextDocumentPatterns(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "README.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	plansRunbooksAndEvaluations := markdownSection(t, string(content), "## Plans, runbooks, and evaluations")

	tests := []struct {
		name       string
		pattern    string
		policyTerm string
		reason     string
	}{
		{
			name:       "playbooks",
			pattern:    "*-playbook.md",
			policyTerm: "boundary rules",
			reason:     "playbooks stay contextual until promoted into stable boundary docs",
		},
		{
			name:       "validation docs",
			pattern:    "*-validation.md",
			policyTerm: "review policy",
			reason:     "validation findings stay contextual until promoted into stable boundary docs",
		},
		{
			name:       "split plans",
			pattern:    "*-split.md",
			policyTerm: "review policy",
			reason:     "split plans stay contextual until promoted into stable boundary docs",
		},
		{
			name:       "management notes",
			pattern:    "*-management.md",
			policyTerm: "review policy",
			reason:     "management notes stay contextual until promoted into stable boundary docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, phrase := range []string{tt.pattern, tt.policyTerm} {
				if !strings.Contains(plansRunbooksAndEvaluations, phrase) {
					t.Errorf("%s Plans, runbooks, and evaluations must mention %q so %s", path, phrase, tt.reason)
				}
			}
		})
	}
}

func TestArchitectureReadmeIndexesRuntimeFlowSupportingContext(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "README.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	supportingContext := markdownSection(t, string(content), "## Supporting Context")
	required := []string{
		"amazon-crawler-runtime-flow.md",
		"runtime flow",
	}
	for _, phrase := range required {
		if !strings.Contains(supportingContext, phrase) {
			t.Errorf("%s Supporting Context must mention %q so runtime-flow background stays discoverable without becoming stable policy", path, phrase)
		}
	}
}

func TestArchitectureReadmeDescribesSupportingContextRoles(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "README.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	supportingContext := markdownSection(t, string(content), "## Supporting Context")
	required := []string{
		"target architecture context",
		"status lifecycle context",
		"TEMU architecture pattern context",
		"TEMU pipeline stage context",
		"ListingKit refactor status context",
	}
	for _, phrase := range required {
		if !strings.Contains(supportingContext, phrase) {
			t.Errorf("%s Supporting Context must mention %q so background documents have explicit roles instead of becoming an undifferentiated file list", path, phrase)
		}
	}
}

func TestArchitectureReadmeWorkingRuleCoversContextualNotes(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "README.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	workingRule := markdownSection(t, string(content), "## Working Rule")
	required := []string{
		"prefer the stable boundary document",
		"contextual note",
		"instead of creating a third interpretation",
	}
	for _, phrase := range required {
		if !strings.Contains(workingRule, phrase) {
			t.Errorf("%s Working Rule must mention %q so every non-stable architecture note is resolved through stable boundary docs", path, phrase)
		}
	}
}

func TestArchitectureReadmeCoversEveryArchitectureDocument(t *testing.T) {
	readmePath := filepath.Join("..", "docs", "architecture", "README.md")
	readmeContent, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}

	for _, phrase := range []string{"Every architecture document", "indexed above", "time-bounded context pattern"} {
		if !strings.Contains(string(readmeContent), phrase) {
			t.Errorf("%s must mention %q so new architecture documents cannot drift outside the index contract", readmePath, phrase)
		}
	}

	explicitReferences := architectureReadmeExplicitReferences(t, string(readmeContent))
	contextPatterns := architectureReadmeTimeBoundedPatterns(t, string(readmeContent))
	documents, err := filepath.Glob(filepath.Join("..", "docs", "architecture", "*.md"))
	if err != nil {
		t.Fatalf("glob architecture docs: %v", err)
	}

	for _, document := range documents {
		name := filepath.Base(document)
		if name == "README.md" {
			continue
		}
		if explicitReferences[name] || matchesAnyPattern(t, name, contextPatterns) {
			continue
		}
		t.Errorf("%s must be explicitly indexed in %s or match one of the time-bounded context patterns %v", name, readmePath, contextPatterns)
	}
}

func TestArchitectureReadmeReferencesExistingDocuments(t *testing.T) {
	readmePath := filepath.Join("..", "docs", "architecture", "README.md")
	readmeContent, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}

	assertMarkdownReferencesExistingDocuments(t, readmePath, string(readmeContent))
}

func TestArchitectureReadmeRequiresBoundaryDocumentsToHaveDocumentTests(t *testing.T) {
	readmePath := filepath.Join("..", "docs", "architecture", "README.md")
	readmeContent, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}

	readmeText := string(readmeContent)
	const testRule = "Every stable or development boundary document must have a document test"
	if !strings.Contains(readmeText, testRule) {
		t.Errorf("%s must state %q so new long-lived architecture entrypoints are added with automated guard coverage", readmePath, testRule)
	}

	testPath := filepath.Join(".", "architecture_docs_test.go")
	testContent, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("read %s: %v", testPath, err)
	}

	for _, heading := range []string{"## Stable Boundary Documents", "## Development Boundary Documents"} {
		for _, reference := range normalizedArchitectureDocReferences(t, markdownSection(t, readmeText, heading)) {
			if reference == "docs/architecture/architecture-review-checklist.md" {
				continue
			}
			if !strings.Contains(string(testContent), reference) && !strings.Contains(string(testContent), filepath.Base(reference)) {
				t.Errorf("%s lists %q as a boundary document, but %s does not reference it", readmePath, reference, testPath)
			}
		}
	}
}

func assertMarkdownReferencesExistingDocuments(t *testing.T, sourcePath string, markdownContent string) {
	t.Helper()

	for _, reference := range normalizedArchitectureDocReferences(t, markdownContent) {
		name := filepath.Base(reference)
		if strings.Contains(name, "*") {
			continue
		}

		targetPath := filepath.Join("..", filepath.FromSlash(reference))
		if _, err := os.Stat(targetPath); err != nil {
			t.Errorf("%s references %s, but it must resolve to an existing repository document: %v", sourcePath, reference, err)
		}
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
		"TestCmdPackagesDoNotImportAppCompatibilityLayers",
		"TestHackContainsOnlyManagedSupportAreas",
		"TestHackSupportAreasContainNoLocalArtifacts",
		"TestTrackedLocalArtifactsStayOutOfProductionEntrypoints",
		"TestProductionEntrypointsContainNoLocalArtifacts",
		"TestTrackedLocalArtifactsStayOutOfTools",
		"TestToolsContainNoLocalArtifacts",
		"TestInternalPackagesContainNoLocalArtifacts",
		"TestSDSLoginRuntimeStateStaysOutOfInternalPackages",
		"TestPlatformRegistrationPackagesStayThin",
		"TestPlatformRegistrationPackagesContainNoLocalArtifacts",
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
		"TestAppBootstrapManagementClientImportsStayAllowlisted",
		"TestAppTaskManagementClientImportsStayAllowlisted",
		"TestAppRunnerManagementClientImportsStayAllowlisted",
		"TestAppConsumerManagementClientImportsStayAllowlisted",
		"TestAppHTTPAPIManagementClientImportsStayAllowlisted",
		"TestAppRuntimeListingManagementClientImportsStayAllowlisted",
		"TestAppTaskStatusManagementClientImportsStayAllowlisted",
		"TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted",
		"TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated",
		"TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsOpenAIRuntimeAssemblyDedicated",
		"TestHTTPAPIRuntimeKeepsSharedResourceAssemblyDedicated",
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
		"TestAppProcessorCompatibilityLayerIsRetired",
		"TestInternalPackagesDoNotImportAppStateCompatibilityLayer",
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
		"TestProjectBoundaryDomainsDoNotImportListingKitFacade",
		"TestListingPreviewPackageStaysPlatformNeutral",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so preview extraction keeps a stable platform-neutral boundary", path, phrase)
		}
	}
}
