# Project Boundary Rules

This document defines the default package and dependency boundaries for the Task Processor / ListingKit codebase.

It complements the project-wide refactoring authority:

- [`project-target-architecture.md`](./project-target-architecture.md)
- [`docs/refactoring/project-wide-refactoring-plan.md`](../refactoring/project-wide-refactoring-plan.md)

When implementing broad refactoring, use these rules unless a newer ADR or refactoring document explicitly supersedes them.

## How To Use This Document

Use this document as the default repository-wide entrypoint for package ownership and dependency direction decisions. Start here before adding new code, moving code, or accepting a new package dependency.

Specialized architecture documents can narrow a boundary for one area, such as HTTP API assembly or listing preview behavior. If a specialized document seems broader or conflicts with this file, treat the narrower rule as the active rule and update both documents before widening the boundary.

## 1. Architecture Style

The project should remain a modular monolith until package boundaries are stable.

Do not split runtime services merely to compensate for unclear package ownership. First clarify module boundaries inside the monolith.

## 2. High-level Layers

Preferred dependency direction:

```text
cmd
  -> app
  -> listing / product / marketplace modules
  -> platform / integration interfaces
```

Current package names may not fully match the final target names yet. During migration, apply the same direction to existing packages:

```text
cmd/*
  -> internal/app/*
  -> internal/listingkit, internal/catalog, internal/asset, internal/publishing/*, internal/workspace/*, internal/{amazon,shein,temu,walmart}
  -> internal/infra, internal/platform, internal/integration through interfaces
```

## 3. Layer Ownership

### 3.1 `cmd/*`

Owns process entrypoints only.

Allowed:

- Load process-level command options.
- Start API servers or workers.
- Call `internal/app/*` builders.

Avoid:

- Business logic.
- Repository implementation details.
- Marketplace rules.
- Direct workflow internals.

### 3.2 `internal/app/*`

Owns runtime assembly.

Allowed:

- Config loading and validation.
- Dependency construction.
- Route registration.
- Worker registration.
- Wiring handlers to service interfaces.
- Runtime support glue.

Avoid:

- Business rules.
- Marketplace-specific decision logic.
- Product or asset transformation rules.
- Platform publishing rules.

### 3.3 `internal/listingkit`

Current role: compatibility facade and listing orchestration surface.

Allowed:

- Task lifecycle and orchestration.
- Workflow entrypoints and request normalization.
- Preview/export aggregation.
- Revision/history facade.
- Submission coordination.
- Studio/task workspace facade behavior.
- API-facing shell models.
- Cross-platform listing task concepts.

Avoid:

- New SHEIN category, attribute, sale-attribute, pricing, or publishing rules.
- New SHEIN editor, repair, revision UX, inspection, or workspace rules.
- New Amazon/TEMU/Walmart rules that can live in platform packages.
- Direct ownership of product facts or reusable visual asset rules.

### 3.4 Product modules

Current examples:

- `internal/catalog`
- `internal/asset`
- `internal/productimage`

Own:

- Product facts.
- Canonical product models.
- Reusable image and asset models.
- Asset bundle construction.
- Product image transformations not tied to a marketplace rule.

Must not depend on:

- `internal/listingkit`
- HTTP handlers.
- Marketplace workspace facade code.

### 3.5 Marketplace modules

Current examples:

- `internal/marketplace/shein/publishing`
- `internal/marketplace/shein/workspace`
- `internal/amazon`
- `internal/shein`
- `internal/temu`

Own:

- Marketplace-specific publishing rules.
- Marketplace-specific API payload builders.
- Marketplace-specific category / attribute / price / validation rules.
- Marketplace workspace and editor behavior.

Must not depend on:

- `internal/listingkit` root facade.
- HTTP runtime assembly.

### 3.6 Platform and integration modules

Future target examples:

- `internal/platform/config`
- `internal/platform/logging`
- `internal/platform/metrics`
- `internal/platform/database`
- `internal/platform/redis`
- `internal/platform/temporal`
- `internal/platform/objectstore`
- `internal/integration/openai`
- `internal/integration/s3`
- `internal/integration/playwright`

Own:

- Runtime infrastructure adapters.
- External client construction.
- Health checks.
- Low-level connection and retry behavior.

Should expose small interfaces to business modules.

Must not depend on:

- ListingKit business services.
- Marketplace business rules.
- HTTP handlers.

## 4. Forbidden Import Directions

These import directions are forbidden by default:

```text
internal/catalog        -> internal/listingkit
internal/asset          -> internal/listingkit
internal/productimage   -> internal/listingkit
internal/publishing/*   -> internal/listingkit
internal/workspace/*    -> internal/listingkit
internal/amazon         -> internal/listingkit
internal/shein          -> internal/listingkit
internal/temu           -> internal/listingkit
internal/walmart        -> internal/listingkit
internal/infra/*        -> internal/listingkit
internal/platform/*     -> internal/listingkit
internal/integration/*  -> internal/listingkit
```

Also forbidden:

```text
domain/product/marketplace code -> gin
domain/product/marketplace code -> app/httpapi
domain/product/marketplace code -> concrete Temporal worker bootstrap
domain/product/marketplace code -> concrete external clients when a local interface is sufficient
```

## 5. Exception Policy

Some legacy imports may exist temporarily during migration.

If an exception is necessary:

1. Add a short note in the related refactoring plan or PR description.
2. Keep the exception narrow.
3. Prefer an adapter or local interface.
4. Add a follow-up cleanup task.
5. Do not treat the exception as a precedent.

## 6. Placement Rules for New Code

Use this table when adding new code:

| New code type | Preferred home |
| --- | --- |
| API route registration | owning module `internal/*/httpapi` first; `internal/app/httpapi` only for shared runtime aggregation |
| Request parsing / response writing | `internal/listingkit/api` or API package owned by the module |
| Task lifecycle | `internal/listingkit/task` during migration |
| Workflow orchestration | `internal/listingkit/workflow` during migration |
| Platform-neutral preview rules | `internal/listing/preview`; see `listing-preview-boundaries.md` |
| Legacy preview facade / task-result aggregation | `internal/listingkit` during migration |
| Export aggregation | `internal/listingkit/export` during migration |
| Revision/history facade | `internal/listingkit/revision` during migration |
| Submission state / retry / recovery | `internal/listing/submission` target; use `internal/listingkit/submission` only as a temporary compatibility bridge during migration |
| Product facts | `internal/catalog` |
| Reusable asset facts | `internal/asset` |
| Product image processing | `internal/productimage` or `internal/asset` depending on ownership |
| SHEIN publishing rules | `internal/marketplace/shein/publishing` |
| SHEIN workspace/editor/repair rules | `internal/marketplace/shein/workspace` |
| Amazon-specific rules | `internal/amazon` now; later `internal/marketplace/amazon` |
| TEMU-specific rules | `internal/temu` now; later `internal/marketplace/temu` |
| OpenAI client adapter | `internal/infra/clients/openai` now; later `internal/integration/openai` |
| S3/object storage adapter | current object storage package now; later `internal/platform/objectstore` or `internal/integration/s3` |

## 7. Review Checklist

Before merging a refactoring or feature PR, check:

- Does it move business logic out of `app` packages?
- Does it avoid adding new marketplace rules to root `listingkit`?
- Does it keep product facts outside ListingKit?
- Does it hide external clients behind local interfaces where useful?
- Does it reduce or at least not increase root `internal/listingkit` complexity?
- Does it avoid creating new import cycles?
- Are behavior-preserving moves separated from feature changes?
- Were relevant tests run?

## 8. Current Enforcement

The active import-boundary tests in `tests/import_boundaries_test.go` and architecture document tests in `tests/architecture_docs_test.go` are the executable version of this document. When this list changes, update this document in the same change so future refactors can find the enforced boundary from one place.

- `TestDomainHTTPPackagesDoNotImportAppHTTPAPI`
- `TestBusinessDomainsDoNotImportAppHTTPAPI`
- `TestProjectBoundaryDomainsDoNotImportListingKitFacade`
- `TestListingKitSubdomainsDoNotImportRootFacade`
- `TestListingKitRootSheinWorkspaceBridgesDoNotImportWorkspaceDomainDirectly`
- `TestListingKitRootNonTestFilesDoNotImportWorkspaceDomainDirectly`
- `TestListingKitSheinWorkspaceBridgeDoesNotImportLegacyWorkspaceDomain`
- `TestListingKitDoesNotImportLegacySheinRuntime`
- `TestListingKitDoesNotImportSheinAPIRoot`
- `TestListingKitNonAPISheinImportsStayAllowlisted`
- `TestListingKitAmazonListingImportsStayAllowlisted`
- `TestCatalogDoesNotDependOnProductEnrichAliases`
- `TestCanonicalTypesDoNotUseProductEnrichCompatibilityAliases`
- `TestSheinPipelineDoesNotImportListingKitFacade`
- `TestSheinSubmitPrepDoesNotImportListingKitTenantContext`
- `TestListingKitRootSheinHelpersStayAllowlisted`
- `TestListingKitRootServiceSubmitFilesStayAllowlisted`
- `TestListingKitRootTaskSubmissionFilesStayAllowlisted`
- `TestListingKitRootServiceGenerationFilesStayAllowlisted`
- `TestListingKitRootGenerationFilesStayAllowlisted`
- `TestListingPreviewPackageStaysPlatformNeutral`
- `TestTemporalSDKImportsStayInRuntimeAndOrchestrationAdapters`
- `TestTemporalRuntimePackagesDoNotImportHTTPAPI`
- `TestProductImageExternalClientImportsStayAllowlisted`
- `TestAmazonExternalClientImportsStayAllowlisted`
- `TestSheinBridgeExternalClientImportsStayAllowlisted`
- `TestSheinRetiredManagementImportsStayBlocked`
- `TestSheinOpenAIImportsStayAllowlisted`
- `TestListingKitHTTPAPIExternalClientImportsStayAllowlisted`
- `TestListingKitSheinSyncLegacyPromotionImportsStayAllowlisted`
- `TestListingKitRootOpenAIImportsStayAllowlisted`
- `TestListingKitRootDoesNotImportManagementAPI`
- `TestListingKitSupportFileStaysRetired`
- `TestPublishingSheinSubmitPrepUsesOnlySensitiveWordAdapter`
- `TestTEMUSyncAndPricingRetiredManagementImportsStayBlocked`
- `TestTEMUProductStoreAndSchedulerRetiredManagementImportsStayBlocked`
- `TestTEMURuntimeAndBridgeRetiredManagementImportsStayBlocked`
- `TestTEMUOpenAIImportsStayAllowlisted`
- `TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted`
- `TestPublishingSheinOpenAIImportsStayAllowlisted`
- `TestPublishingSheinManagedAPIImportsStayAllowlisted`
- `TestPublishingSheinManagedRetiredManagementImportsStayBlocked`
- `TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit`
- `TestPublishingCommonUsesCanonicalPackage`
- `TestPublishingCommonDoesNotImportPlatformImplementations`
- `TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated`
- `TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated`
- `TestAppHTTPAPIRootListingKitHelpersStayAllowlisted`
- `TestAppHTTPAPIListingKitSupportImportsStayAllowlisted`
- `TestAppHTTPAPIListingKitRootImportsStayAllowlisted`
- `TestAppHTTPAPIListingKitHTTPAPIImportsStayAllowlisted`
- `TestAppHTTPAPIModuleBuildersStayAllowlisted`
- `TestAppHTTPAPIRouteDescriptorHelpersStayAllowlisted`
- `TestHTTPAPITypesDoesNotOwnRunOptions`
- `TestHTTPAPIModulesFileDoesNotOwnFeatureBuildWrappers`
- `TestHTTPAPIModulesFileDoesNotOwnBootstrapOrchestration`
- `TestHTTPAPILegacyBuildHandlersFacadeStaysRetired`
- `TestHTTPAPIModulesFileDoesNotOwnWorkerRuntimeSupport`
- `TestHTTPAPIModulesFileDoesNotOwnLoginRuntimeSupport`
- `TestHTTPAPICompositionBuilderDoesNotOwnLoginBootstrapTypes`
- `TestHTTPAPICompositionBuilderDoesNotOwnLoginFeatureAssembly`
- `TestHTTPAPIRuntimeStateDoesNotOwnLoginBootstrapResultTypes`
- `TestHTTPAPIRuntimeStateUsesOwningFeatureHTTPAPIModuleTypes`
- `TestHTTPAPIRuntimeDepsMethodsUseOwningFeatureHTTPAPIModuleTypes`
- `TestHTTPModulesUseOwningFeatureHTTPAPIModuleTypesInSignatures`
- `TestHTTPAPIFeatureBuildersUseOwningFeatureHTTPAPIModuleTypesInSignatures`
- `TestFeatureModuleBuilderContractsUseOwningModuleTypes`
- `TestHTTPAPIRuntimeStateUsesOwningSupportModuleResultTypes`
- `TestHTTPAPICompositionBuilderDoesNotOwnSupportModuleBuilderContracts`
- `TestHTTPAPICompositionBuilderDoesNotOwnSupportFeatureAssembly`
- `TestHTTPAPIModulesFileDoesNotOwnListingKitSDSRuntimeSupportHook`
- `TestHTTPAPICompositionBuilderDoesNotOwnProductImageRuntimeInputs`
- `TestHTTPAPICompositionBuilderDoesNotOwnAmazonListingRuntimeInput`
- `TestHTTPAPICompositionBuilderDoesNotOwnListingKitRuntimeInput`
- `TestHTTPAPIRuntimeKeepsOpenAIRuntimeAssemblyDedicated`
- `TestHTTPAPIRuntimeKeepsSharedResourceAssemblyDedicated`
- `TestHTTPAPIRuntimeKeepsPromptRuntimeAssemblyDedicated`
- `TestHTTPAPIRuntimeKeepsProductEnrichRuntimeAssemblyDedicated`
- `TestHTTPAPIRuntimeKeepsPathResolutionDedicated`
- `TestHTTPAPIRuntimeKeepsConfigLoadingDedicated`
- `TestHTTPAPIRuntimeKeepsRuntimeDepsMethodsDedicated`
- `TestHTTPAPIAdaptersKeepTaskRepositoryAssemblyDedicated`
- `TestHTTPAPIAdaptersKeepPromptStoreAssemblyDedicated`
- `TestBootstrapKeepsTaskRepositoryAssemblyInDedicatedFile`
- `TestBootstrapKeepsModelProviderAssemblyInDedicatedFile`
- `TestBootstrapKeepsLLMScorerAssemblyInDedicatedFile`
- `TestBootstrapKeepsAssetPublisherAssemblyInDedicatedFile`
- `TestBootstrapKeepsImagePipelineComponentAssemblyInDedicatedFile`
- `TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages`
- `TestPlatformModulesHistoricalImplementationImportsStayAllowlisted`
- `TestPlatformRegistrationPackagesStayThin`
- `TestPlatformRegistrationPackagesContainNoLocalArtifacts`
- `TestBusinessDomainsDoNotImportAppRuntimeAssembly`
- `TestAppBootstrapRetiredManagementImportsStayBlocked`
- `TestAppTaskRetiredManagementImportsStayBlocked`
- `TestAppTaskFetcherDoesNotStoreRetiredManagementService`
- `TestAppTaskDispatchGuardUsesCapabilityNames`
- `TestAppTaskDispatcherUsesCapabilityNames`
- `TestAppTaskStatusUpdatesUseCapabilityNames`
- `TestAppRunnerRetiredManagementImportsStayBlocked`
- `TestAppConsumerRetiredManagementImportsStayBlocked`
- `TestAppHTTPAPIRetiredManagementImportsStayBlocked`
- `TestAppRuntimeListingRetiredManagementImportsStayBlocked`
- `TestAppTaskStatusRetiredManagementImportsStayBlocked`
- `TestPlatformTaskRetiredManagementImportsStayBlocked`
- `TestStateRetiredManagementImportsStayBlocked`
- `TestPlatformBaseRetiredManagementImportsStayBlocked`
- `TestProcessorRetiredManagementImportsStayBlocked`
- `TestTaskRPCAPIRetiredManagementImportsStayBlocked`
- `TestSDSClientRetiredManagementImportsStayBlocked`
- `TestSheinLoginBootstrapRetiredManagementImportsStayBlocked`
- `TestSheinLoginServiceRetiredManagementImportsStayBlocked`
- `TestSheinLoginManagedRetiredManagementImportsStayBlocked`
- `TestSharedPricingRetiredManagementImportsStayBlocked`
- `TestListingKitHTTPAPIRetiredManagementImportsStayBlocked`
- `TestCmdPackagesDoNotImportAppCompatibilityLayers`
- `TestInternalPackagesDoNotImportAppProcessorCompatibilityLayer`
- `TestInternalPackagesDoNotImportAppStateCompatibilityLayer`
- `TestAppProcessorCompatibilityLayerIsRetired`
- `TestAppStateCompatibilityLayerIsRetired`
- `TestInfraProductCrawlerAdapterIsRetired`
- `TestAppCrawlerFetcherCompatibilityLayerIsRetired`
- `TestInfrastructurePackagesDoNotImportBusinessDomains`
- `TestBusinessImplementationPackagesDoNotImportGinDirectly`
- `TestAppTaskPollingSourceUsesCapabilityNames`
- `TestPlatformProcessorRegistryDoesNotExposeRetiredManagementService`
- `TestAppConsumerTaskStatusRuntimeProviderIsNotNamedRetiredManagementService`
- `TestAppConsumerDoesNotUseManagementNamedTaskStatusAdapter`
- `TestTaskStatusAdapterCallersUseRuntimeNamedConstructor`
- `TestTaskStatusPackageDoesNotExposeManagementNamedAdapter`
- `TestTaskStatusRuntimeErrorsUseCapabilityNames`
- `TestTaskStatusPackageDoesNotImportRetiredManagementPackage`
- `TestAmazonTaskStatusUpdatesUseTaskStatusRuntime`
- `TestAmazonAuthPauseUsesStoreAPIPort`
- `TestAmazonServicesUseStoreAPIPort`
- `TestTemuPricingRuntimeUsesCapabilityNames`
- `TestTemuSchedulerRuntimeUsesCapabilityNames`
- `TestTemuProcessorRuntimeUsesCapabilityNames`
- `TestTemuSyncRuntimeUsesCapabilityNames`
- `TestTemuRuntimeErrorsUseCapabilityNames`
- `TestTemuPricingFallbackLogsUseCapabilityNames`
- `TestTemuSyncFallbackLogsUseCapabilityNames`
- `TestAppRunnerSchedulerStoreRuntimeUsesCapabilityNames`
- `TestAppRunnerProcessorLifecycleUsesRuntimeNames`
- `TestAppRunnerHealthChecksUseRuntimeNames`
- `TestTaskStatusPackageDoesNotExposeBroadManagementRuntimeConstructor`

## 9. Immediate Enforcement

Use `scripts/analyze-project-deps.ps1` to generate a dependency baseline and flag likely boundary violations.

The script is advisory at first. Once known legacy exceptions are cataloged, it can be promoted into CI enforcement.
