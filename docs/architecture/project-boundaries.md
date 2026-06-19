# Project Boundary Rules

This document defines the default package and dependency boundaries for the Task Processor / ListingKit codebase.

It complements the project-wide refactoring authority:

- [`project-target-architecture.md`](./project-target-architecture.md)
- [`docs/refactoring/project-wide-refactoring-plan.md`](../refactoring/project-wide-refactoring-plan.md)

When implementing broad refactoring, use these rules unless a newer ADR or refactoring document explicitly supersedes them.

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
| Submission state / retry / recovery | `internal/listing/submission` for generic mechanics; SHEIN transition sequencing stays at the root `internal/listingkit/shein_submit_state.go` stop-line; do not recreate `internal/listingkit/submission` |
| Product facts | `internal/catalog` |
| Reusable asset facts | `internal/asset` |
| Product image processing | `internal/productimage` or `internal/asset` depending on ownership |

Preview extraction is additionally guarded by
`TestListingPreviewPackageStaysPlatformNeutral`, which keeps
`internal/listing/preview` from becoming another entry point for marketplace-
specific or ListingKit-facade imports.
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

For the full architecture review checklist, use
[`architecture-review-checklist.md`](./architecture-review-checklist.md).

## 8. Current Enforcement

Project boundaries are enforced by import-boundary tests first. Representative
guards include:

- `TestBusinessDomainsDoNotImportAppHTTPAPI`
- `TestDomainHTTPPackagesDoNotImportAppHTTPAPI`
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
- `TestSheinManagementClientImportsStayAllowlisted`
- `TestSheinOpenAIImportsStayAllowlisted`
- `TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted`
- `TestPublishingSheinOpenAIImportsStayAllowlisted`
- `TestPublishingSheinManagedAPIImportsStayAllowlisted`
- `TestPublishingSheinManagedManagementImportsStayAllowlisted`
- `TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit`
- `TestPublishingCommonUsesCanonicalPackage`
- `TestPublishingCommonDoesNotImportPlatformImplementations`
- `TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated`
- `TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated`
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
- `TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages`
- `TestPlatformModulesHistoricalImplementationImportsStayAllowlisted`
- `TestPlatformRegistrationPackagesStayThin`
- `TestPlatformRegistrationPackagesContainNoLocalArtifacts`
- `TestBusinessDomainsDoNotImportAppRuntimeAssembly`
- `TestCmdPackagesDoNotImportAppCompatibilityLayers`
- `TestInternalPackagesDoNotImportAppProcessorCompatibilityLayer`
- `TestInternalPackagesDoNotImportAppStateCompatibilityLayer`
- `TestAppProcessorCompatibilityLayerIsRetired`
- `TestAppStateCompatibilityLayerIsRetired`
- `TestInfraProductCrawlerAdapterIsRetired`
- `TestAppCrawlerFetcherCompatibilityLayerIsRetired`
- `TestInfrastructurePackagesDoNotImportBusinessDomains`
- `TestBusinessImplementationPackagesDoNotImportGinDirectly`

Use `scripts/analyze-project-deps.ps1` as an advisory dependency baseline when
reviewing broader refactors, but do not treat the script as a substitute for the
active guard tests.

For the SHEIN publishing path specifically, treat managed management clients as
a retirement seam rather than a stable dependency direction: new flows should
prefer direct database-backed composition instead of expanding management API
coupling.

That same publishing boundary also forbids reintroducing legacy runtime or
ListingKit-root dependencies into the publishing slice; migration work should
converge on dedicated publishing/domain storage seams instead of reviving the
older facade chain.

At the convergence layer, keep `internal/publishing/common` platform-neutral as
well: shared publishing code can depend on canonical contracts, but it must not
pull concrete platform implementations back upward into the common slice.

That common slice should also anchor on canonical publishing packages rather
than ad hoc platform-specific type aliases, so cross-platform flows converge on
one stable contract surface before reaching concrete implementations.

Likewise, platform modules should stay downstream of domain and HTTP assembly
boundaries. They can consume stable contracts and adapters, but they must not
take direct dependencies on business-domain packages or the app HTTP assembly
layer.

At the HTTP API edge, external client runtime dependencies should stay in
dedicated runtime-deps types rather than bleeding across unrelated module
contracts or domain-facing surfaces.

OpenAI assembly should stay behind dedicated HTTP API adapters as well, so
feature modules and broader runtime wiring do not absorb concrete provider
construction concerns.

The runtime side of that assembly should remain dedicated too: OpenAI runtime
bootstrap belongs in its own seam instead of accreting inside generic runtime
startup paths.

Shared-resource assembly follows the same rule: common bootstrap resources
should be attached through a dedicated runtime seam rather than mixed into
unrelated startup responsibilities.

Prompt runtime setup should stay isolated in its own assembly seam too, so
prompt registry wiring and tenant prompt-store attachment do not leak across
unrelated runtime concerns.

ProductEnrich runtime setup should follow the same boundary: LLM manager,
understanding pipeline, and parser bootstrap belong in a dedicated runtime seam
instead of spreading into generic startup code.

Path resolution belongs in a dedicated runtime seam too, so environment/workdir
normalization does not drift back into broader bootstrap orchestration.

Config loading follows the same rule: runtime config assembly should stay in a
dedicated seam instead of being rebuilt ad hoc across startup entrypoints.

The supporting methods on `runtimeDeps` should stay isolated too, so accessors,
closers, and module-attach helpers do not drift back into the top-level runtime
or generic bootstrap files.

Task-repository assembly should stay behind dedicated HTTP API adapters as
well, so persistence wiring does not spread into unrelated runtime or feature
assembly seams.

Prompt-store assembly should follow the same rule, keeping prompt persistence
and tenant store wiring behind a dedicated adapter seam instead of leaking into
broader runtime setup.

At bootstrap time, task-repository assembly should still stay in its own
dedicated file instead of turning the top-level bootstrap path back into a
catch-all construction hub.

Where historical platform implementation imports still exist, keep them
explicitly allowlisted as migration seams only. They should shrink over time,
not become precedent for new platform-to-business coupling.

Platform registration packages should remain thin delegation shells as well.
They can wire stable entrypoints together, but they should not accumulate local
business rules, runtime orchestration, or long-lived feature ownership.

Those registration packages should also stay free of checked-in local
artifacts, so temporary scaffolding and machine-specific outputs do not turn
into accidental platform-layer ownership.

The dependency direction between domain and app assembly should stay one-way as
well: business-domain packages must not import app runtime assembly packages,
because composition belongs at the app edge rather than inside domain logic.

Production entrypoints should not keep deprecated compatibility layers alive
either. `cmd/*` packages must depend on the current owning packages directly
rather than routing new behavior back through retired app compatibility paths.

The same retirement rule applies inside `internal/*`: packages should not
reconnect themselves to the old app processor compatibility layer once the
direct owner package already exists.

That processor bridge is not merely discouraged; it is retired. New work should
land in `internal/processor` instead of preserving `internal/app/processor` as
an alternate path.

That applies to the old app state compatibility layer too: internal packages
should use `internal/state` directly instead of reviving the deprecated app
state bridge.

And like the processor bridge, that app state bridge is retired rather than
optional. New code should treat `internal/state` as the only supported owner.

The old `internal/infra/productcrawler` adapter is retired as well. Product
sourcing flows should move through the current sourcing and crawler ownership
packages instead of preserving that legacy infra bridge.

The same is true for `internal/app/crawler/fetcher`: it is a retired
compatibility path, and new crawler fetch logic should live under the current
`internal/crawler/fetcher` owner instead.
