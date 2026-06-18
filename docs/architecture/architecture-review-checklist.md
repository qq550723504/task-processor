# Architecture Review Checklist

## Goal

Use this checklist when reviewing framework, refactoring, HTTP API assembly,
Temporal, or platform-boundary changes. It turns the current structure rules
into repeatable review questions so boundaries do not depend on memory.

## Required Checks

Before merging a structural or feature PR, verify:

1. No new reverse dependency points from domain, product, or marketplace code
   back into `internal/app/httpapi`.
2. No new direct imports of retired compatibility paths such as
   `internal/app/processor` or `internal/app/state`.
3. New route registration lives in the owning module `internal/*/httpapi`
   package, with `internal/app/httpapi` limited to shared runtime aggregation.
4. Business helpers are not added to app-layer assembly packages.
5. New Temporal usage follows `docs/architecture/temporal-boundaries.md` and
   keeps SDK types out of domain-facing contracts.
6. New platform-specific rules do not grow the root `internal/listingkit`
   facade when a marketplace, publishing, or product module owns the behavior.
7. Preview extraction follows `docs/architecture/listing-preview-boundaries.md`:
   platform-neutral preview behavior belongs in `internal/listing/preview`, not
   in marketplace-specific implementation packages.
8. New remote-service behavior follows
   `docs/architecture/external-client-boundary-inventory.md`: prefer a local
   interface and avoid leaking a concrete external client adapter into
   domain-facing contracts.
   Treat `management` as a retirement target: new business data access should
   prefer in-repository database/repository access instead of new management API
   call sites.
9. Repository layout changes follow `docs/development/repository-structure.md`:
   `cmd/` stays limited to official entrypoints, `hack/` stays limited to
   managed support areas, and local artifacts stay out of production entrypoint
   and long-lived tool directories.
10. Any boundary exception is documented with a narrow scope and a follow-up
   cleanup path.
11. Relevant import-boundary and architecture tests were run. If a guard is
   added, removed, or renamed, update the `docs/architecture/next-steps.md`
   `Current guard coverage` guard baseline in the same change.

## Guard Baseline

Use the `Current guard coverage` section in
`docs/architecture/next-steps.md` as the current import-boundary baseline.
Representative guard references must remain a subset of the current guard coverage baseline.
At minimum, structural review should consider representative guards such as:

- `TestBusinessDomainsDoNotImportAppHTTPAPI`
- `TestProjectBoundaryDomainsDoNotImportListingKitFacade`
- `TestListingKitSubdomainsDoNotImportRootFacade`
- `TestListingKitRootSheinWorkspaceBridgesDoNotImportWorkspaceDomainDirectly`
- `TestListingKitRootNonTestFilesDoNotImportWorkspaceDomainDirectly`
- `TestListingKitSheinWorkspaceBridgeDoesNotImportLegacyWorkspaceDomain`
- `TestListingKitDoesNotImportLegacySheinRuntime`
- `TestListingKitDoesNotImportSheinAPIRoot`
- `TestListingKitNonAPISheinImportsStayAllowlisted`
- `TestListingKitRootDoesNotImportManagementAPI`
- `TestListingKitSheinSyncLegacyPromotionImportsStayAllowlisted`
- `TestListingKitAmazonListingImportsStayAllowlisted`
- `TestCatalogDoesNotDependOnProductEnrichAliases`
- `TestCanonicalTypesDoNotUseProductEnrichCompatibilityAliases`
- `TestSheinPipelineDoesNotImportListingKitFacade`
- `TestSheinSubmitPrepDoesNotImportListingKitTenantContext`
- `TestPublishingSheinSubmitPrepUsesOnlySensitiveWordAdapter`
- `TestListingKitRootSheinHelpersStayAllowlisted`
- `TestListingKitRootServiceSubmitFilesStayAllowlisted`
- `TestListingKitRootTaskSubmissionFilesStayAllowlisted`
- `TestListingKitRootServiceGenerationFilesStayAllowlisted`
- `TestListingKitRootGenerationFilesStayAllowlisted`
- `TestInfrastructurePackagesDoNotImportBusinessDomains`
- `TestBusinessImplementationPackagesDoNotImportGinDirectly`
- `TestDomainHTTPPackagesDoNotImportAppHTTPAPI`
- `TestAppHTTPAPIRootListingKitHelpersStayAllowlisted`
- `TestAppHTTPAPIModuleBuildersStayAllowlisted`
- `TestAppHTTPAPIRouteDescriptorHelpersStayAllowlisted`
- `TestListingKitSupportFileStaysRetired`
- `TestAppHTTPAPIListingKitSupportImportsStayAllowlisted`
- `TestAppHTTPAPIListingKitRootImportsStayAllowlisted`
- `TestAppHTTPAPIListingKitHTTPAPIImportsStayAllowlisted`
- `TestBusinessDomainsDoNotImportAppRuntimeAssembly`
- `TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages`
- `TestPlatformModulesHistoricalImplementationImportsStayAllowlisted`
- `TestPlatformRegistrationPackagesStayThin`
- `TestPlatformRegistrationPackagesContainNoLocalArtifacts`
- `TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit`
- `TestPublishingSheinNonAPISheinImportsStayAllowlisted`
- `TestPublishingSheinManagedAPIImportsStayAllowlisted`
- `TestPublishingSheinManagedManagementImportsStayAllowlisted`
- `TestPublishingCommonUsesCanonicalPackage`
- `TestPublishingCommonDoesNotImportPlatformImplementations`
- `TestCmdContainsOnlyOfficialEntrypoints`
- `TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages`
- `TestHackContainsOnlyManagedSupportAreas`
- `TestHackSupportAreasContainNoLocalArtifacts`
- `TestTrackedLocalArtifactsStayOutOfProductionEntrypoints`
- `TestProductionEntrypointsContainNoLocalArtifacts`
- `TestTrackedLocalArtifactsStayOutOfTools`
- `TestToolsContainNoLocalArtifacts`
- `TestInternalPackagesContainNoLocalArtifacts`
- `TestSDSLoginRuntimeStateStaysOutOfInternalPackages`
- `TestInternalPackagesDoNotImportAppProcessorCompatibilityLayer`
- `TestAppProcessorCompatibilityLayerIsRetired`
- `TestInternalPackagesDoNotImportAppStateCompatibilityLayer`
- `TestAppStateCompatibilityLayerIsRetired`
- `TestInfraProductCrawlerAdapterIsRetired`
- `TestAppCrawlerFetcherCompatibilityLayerIsRetired`
- `TestCmdPackagesDoNotImportAppCompatibilityLayers`
- `TestProductImageExternalClientImportsStayAllowlisted`
- `TestAmazonExternalClientImportsStayAllowlisted`
- `TestSheinManagementClientImportsStayAllowlisted`
- `TestSheinOpenAIImportsStayAllowlisted`
- `TestAppTaskManagementClientImportsStayAllowlisted`
- `TestAppRunnerManagementClientImportsStayAllowlisted`
- `TestAppConsumerManagementClientImportsStayAllowlisted`
- `TestAppBootstrapManagementClientImportsStayAllowlisted`
- `TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted`
- `TestPublishingSheinOpenAIImportsStayAllowlisted`
- `TestListingKitHTTPAPIExternalClientImportsStayAllowlisted`
- `TestListingKitHTTPAPIManagementClientImportsStayAllowlisted`
- `TestListingKitRootOpenAIImportsStayAllowlisted`
- `TestTEMUSyncAndPricingManagementImportsStayAllowlisted`
- `TestTEMUProductStoreAndSchedulerManagementImportsStayAllowlisted`
- `TestTEMUOpenAIImportsStayAllowlisted`
- `TestTemporalSDKImportsStayInRuntimeAndOrchestrationAdapters`
- `TestTemporalRuntimePackagesDoNotImportHTTPAPI`
- `TestListingPreviewPackageStaysPlatformNeutral`

If a PR changes the intended boundary, update the owning architecture document
and its document test in the same change as the code exception.

## Review References

Use these documents during boundary-sensitive review. The stable architecture
documents are the stable source of truth for long-lived boundary rules, while
development boundary documents define long-lived repository structure rules.
The current guard baseline entry points to the current guard coverage baseline:
Document paths in Review References must be listed only in the reference list.
Every review reference must resolve to an existing repository document.
Every review reference must come from the architecture index stable, development, or current guard baseline sections.
Review references must not contain duplicate document entries.
Time-bounded plans, runbooks, and evaluations must not be listed as review references.
Supporting context documents must not be listed as review references unless promoted into stable boundary documents.

- `docs/architecture/README.md`
- `docs/architecture/project-boundaries.md`
- `docs/architecture/httpapi-assembly-boundaries.md`
- `docs/architecture/app-assembly-boundaries.md`
- `docs/architecture/temporal-boundaries.md`
- `docs/architecture/platform-boundary-strategy.md`
- `docs/architecture/historical-platform-migration-inventory.md`
- `docs/architecture/external-client-boundary-inventory.md`
- `docs/architecture/compatibility-retirement.md`
- `docs/architecture/listing-preview-boundaries.md`
- `docs/development/repository-structure.md`
- `docs/architecture/next-steps.md` for the current guard coverage baseline

## Working Rule

If a change makes the dependency direction less obvious, require either a
smaller adapter, a local interface, or an explicit documented exception before
merging.

If plans, runbooks, or contextual notes introduce a long-lived boundary rule,
that rule must be copied or linked into a stable boundary document before being used as review policy.
