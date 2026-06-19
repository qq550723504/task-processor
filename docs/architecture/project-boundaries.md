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
- `TestTemporalSDKImportsStayInRuntimeAndOrchestrationAdapters`
- `TestTemporalRuntimePackagesDoNotImportHTTPAPI`
- `TestProductImageExternalClientImportsStayAllowlisted`
- `TestAmazonExternalClientImportsStayAllowlisted`
- `TestSheinBridgeExternalClientImportsStayAllowlisted`
- `TestSheinManagementClientImportsStayAllowlisted`
- `TestSheinOpenAIImportsStayAllowlisted`
- `TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted`
- `TestPublishingSheinOpenAIImportsStayAllowlisted`
- `TestPublishingSheinManagedManagementImportsStayAllowlisted`
- `TestInfrastructurePackagesDoNotImportBusinessDomains`
- `TestBusinessImplementationPackagesDoNotImportGinDirectly`

Use `scripts/analyze-project-deps.ps1` as an advisory dependency baseline when
reviewing broader refactors, but do not treat the script as a substitute for the
active guard tests.

For the SHEIN publishing path specifically, treat managed management clients as
a retirement seam rather than a stable dependency direction: new flows should
prefer direct database-backed composition instead of expanding management API
coupling.
