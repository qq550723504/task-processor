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
- Expose platform runtime capabilities through narrow interfaces; product readers,
  caches, and fetcher statistics remain separate from the composed fetcher
  contract.

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

For the 1688 source boundary, internal/product must not import internal/listingkit, internal/compatibility, internal/crawler, or internal/integration. The product-owned `internal/product/sourcing` package consumes only provider-neutral `SourceEnvelope` values; it does not own or consume the legacy crawler DTO or a 1688-specific snapshot.

The adapter direction is explicit: internal/integration/crawler/a1688 converts legacy crawler DTOs into adapter-local snapshots and internal/product/sourcing SourceEnvelope values. Integration adapters may import this pure product contract, but they may not call product services or own product workflows.

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

Current runtime-foundation owners:

- `internal/platform/config`
- `internal/platform/logging`
- `internal/platform/database`
- `internal/platform/redis`
- `internal/platform/queue/rabbitmq`
- `internal/platform/workerpool`
- `internal/platform/temporal`
- `internal/platform/featureflags`
- `internal/platform/observability`
- `internal/integration/openai`
- `internal/integration/geminiimage`
- `internal/integration/grsai`
- `internal/integration/s3`
- `internal/integration/httpimage`

Own:

- Platform owns application runtime mechanisms and connection lifecycle.
- Integration owns external client construction and transport adaptation.
- Health checks.
- Low-level connection and retry behavior.

Should expose small interfaces to business modules.

Must not depend on:

- ListingKit business services.
- Marketplace business rules.
- HTTP handlers.

#### Phase 2 closure boundary

Application assembly owns lifecycle, provider registration, migration
execution, instrumentation, and shutdown. Platform owns config mechanics,
logging, database/Goose, Redis, RabbitMQ, worker pool, Temporal dial, feature
flags, and tracing. Integration owns OpenAI, Gemini, GRSAI, S3, and remote
image HTTP.

The following retained packages are explicit migration debt, not new homes:

- `internal/core/config` retains application-shaped schemas; later domain
  phases split those schemas while `internal/platform/config` remains the
  generic loading mechanism.
- `internal/core/logger` is a forwarding-only compatibility facade with an
  82-importer non-growth ceiling. State and implementation live in platform;
  later owners replace facade calls with local logger ports.
- `internal/core/metrics` moves with listing and marketplace business metrics.
- `internal/infra/auth` moves after organization ports exist; its later owner
  is an organization integration adapter.
- `internal/infra/httpx` is split between app HTTP mechanics and later
  product/marketplace crawler adapters.
- `internal/crawler` and mixed GORM persistence packages remain frozen until
  product, marketplace, listing, agent, and organization owners expose ports.

Only these nine target roots satisfy the final no-concrete-infrastructure rule
at Phase 2 close: `internal/listing`, `internal/product`,
`internal/marketplace`, `internal/agent`, `internal/knowledge`,
`internal/resourcecatalog`, `internal/commercial`, `internal/ledger`, and
`internal/organization`. MCP, pgvector, and TigerBeetle are not admitted
technologies; adding one requires a separate approved architecture decision.

### 3.7 `internal/compatibility/listingkit`

Owns backward-compatible ListingKit entrypoints and cross-boundary DTO
translation while the real business owners move into product, listing, and
marketplace packages.

For the 1688 task path, internal/compatibility/listingkit/sourcehandoff owns the 1688 to ListingKit application handoff. It keeps the existing command, store-access, identity, request-shape, and HTTP compatibility behavior while delegating source facts to product sourcing.

### 3.8 `internal/aicapability`

Current role: neutral platform and integration module for AI capability
selection.

Owns:

- Model catalog, policy, routing, and invocation contracts.
- Provider-neutral routing decisions and invocation observability records.

Does not own:

- Product facts or marketplace rules.
- Prompt meaning or prompt construction.
- Provider SDKs, provider request/response DTOs, or concrete credential
  adapters.

### 3.9 `internal/imageagent`

The Image Agent domain package retains its public model, errors, blocked codes,
fingerprints, and repository ports. Within that boundary:

- `internal/imageagent/effectpolicy` owns pure v3 transition eligibility,
  including phase, blocking, budget, lease, fencing, fingerprint, and recovery
  decisions. It depends only on the Go standard library and the parent
  `internal/imageagent` domain package.
- `internal/imageagent/store` owns persistence and concurrency mechanics:
  memory locks, GORM transactions and row locks, record mapping, not-found
  classification, compare-and-swap predicates, and atomic persistence. It
  delegates transition eligibility to `effectpolicy` instead of duplicating
  policy predicates.
- Temporal, HTTP, tool, service, object-store, and ProductImage callers use the
  existing Image Agent repository ports. They do not import `effectpolicy`
  directly, so orchestration and delivery adapters cannot become a second
  transition owner.

### 3.10 `internal/ledger/orgresource`

The organization-resource ledger owns organization-scoped platform balances
such as `store_renewal_period`, `ai_point`, and `data_row`. It does not own
subscription plan assignment, usage metering, account registration, ZITADEL,
or RMB payment flows.

- `internal/ledger/orgresource` owns commands, fixed positive-mint use cases,
  reservation admission contracts, verified-principal and eligibility ports,
  deterministic fingerprints, and immutable result contracts.
- `internal/integration/orgresource` implements the domain's persistence port
  and owns GORM schema, PostgreSQL transaction deadlines and retry
  classification, row locking, source claims, reservations, events, and audit
  persistence.
- A resource reservation is available only to an explicitly registered durable
  owner adapter. The adapter receives the resource transaction, locks the exact
  organization-scoped owner attempt, and writes the reservation binding in the
  same transaction as Operation, Reservation, Bucket, Event, and Audit. The
  resource adapter verifies the locked owner scope and reads the owner back
  after binding; an adapter that does not persist the exact binding fails the
  whole transaction.
- No owner adapter is registered by this foundation slice. A concrete owner is
  not eligible for rollout until its domain also supplies durable attempt
  state, finality/recovery ownership, and the shared Owner Start versus expired
  recovery fence. Runtime reservation and settlement therefore remain closed
  until a concrete owner satisfies that contract.
- Settlement accepts only organization, operation, and reservation identities;
  it never accepts a caller-selected commit/release decision. The registered
  owner adapter locks and returns terminal proof in the same transaction.
  `succeeded_terminal` commits reserved value to consumed, while
  `failed_terminal` or `cancelled_terminal` releases it. Non-terminal and
  `outcome_unknown` attempts remain reserved for owner-specific recovery.
- Every positive available-balance flow uses the same bucket-then-debt lock
  order and debt-first allocator. Welcome credits and reservation releases both
  record gross credit, debt repaid, and net credit in the immutable Event; only
  net credit reaches available.
- Runtime assembly supplies verified service-principal authorization and
  immutable onboarding eligibility adapters. Browser and tenant HTTP layers do
  not construct a generic positive-mint command.
- `internal/listingsubscription` remains the existing plan/entitlement and
  usage-metering authority; its usage ledger is not reused as an enterprise
  resource-balance authority.

### 3.11 `internal/storecenter` expanded Store state

Store Center keeps the legacy `lifecycle_status` during the expand/compatibility
window, while the V7 state contract introduces nullable transitional
`record_status`, `service_status`, `service_started_at`, and
`service_expires_at` columns. `RecordStatus` and `ServiceStatus` validation is
pure domain code; it enforces the record/service/timestamp invariants but does
not infer legacy history. In particular, a legacy disabled row is not treated
as confirmed-absent history. New Store creates and every existing lifecycle
write now use a compatibility writer: provisioning/active/deleting mirror to
the corresponding record state, active starts as `pending_activation`, legacy
disabled mirrors to suspended, and soft delete mirrors to deleted. A known
service period is preserved by lifecycle/profile writes; no period is invented
for legacy rows. Pure Activate/Renew/Reactivate decisions validate these
states and calculate periods without persistence side effects. The
organization-resource integration adapter commits their Store and resource
updates as one unit of work. The Store GORM adapter exposes only narrow
transaction-bound lock/CAS methods to that integration boundary and does not
open a nested transaction. The shared transaction persists the immutable
Operation result (or terminal business failure), resource Bucket/Event/Audit,
and Store service state/version together. A changed Store version or connection
reference prevents resource consumption, and a failed audit rolls every write
back. The lifecycle application service derives Organization and actor from
the verified identity context, rechecks the lifecycle permission, computes the
canonical request fingerprint, and checks the durable Operation before reading
the server-owned quantity policy, Store snapshot, or volatile connection
status. Only Activate queries the connection provider; Renew and Reactivate do
not acquire that unrelated volatile dependency. The Phase 1 history migrator
accepts only a strict, approved `NoAuthoritativeHistorySource` manifest and
persists its immutable token as per-row `confirmed_absent` evidence; it never
infers absence from nullable fields or Store timestamps. Expanded service state
without that evidence is non-authoritative and is re-derived from the legacy
lifecycle before the resolution is persisted, preventing an unexplained paid
period from being blessed by migration. New Stores are marked
`not_applicable_new` with evidence bound to their create fingerprint, so they
cannot re-enter the legacy migration cohort. Legacy provisioning Stores must
also receive `confirmed_absent` evidence before verification can pass, fencing
recovery from activating an unresolved row after the readiness check. The
migration command defaults to read-only verification, opens only an existing
database, and requires an explicit action for one bounded backfill batch using
an existing writable database. Constraints, authority switching, and lifecycle
HTTP routes remain separate later phases and must not be enabled by this
foundation.

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
| Submission state / retry / recovery | `internal/listing/submission`; do not recreate `internal/listingkit/submission` |
| Product facts | `internal/catalog` |
| Reusable asset facts | `internal/asset` |
| Product image processing | `internal/productimage` or `internal/asset` depending on ownership |
| SHEIN publishing rules | `internal/marketplace/shein/publishing` |
| SHEIN workspace/editor/repair rules | `internal/marketplace/shein/workspace` |
| Amazon-specific rules | `internal/amazon` now; later `internal/marketplace/amazon` |
| TEMU-specific rules | `internal/temu` now; later `internal/marketplace/temu` |
| OpenAI client adapter | `internal/integration/openai`; app wires a domain-local port |
| S3/object storage adapter | `internal/integration/s3`; app wires a domain-local port |

For product modeling, keep `internal/catalog/canonical.Product` as the
platform-neutral source of product facts. `internal/publishing/shein.Package`
owns SHEIN workflow state, and its `DraftPayload` is the SHEIN draft contract
for new reads and writes. `RequestDraft` and the package-level `SkcList` are
compatibility/display surfaces during migration; they must not become a second
source of truth.

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
- `depguard: domain_app_httpapi_boundaries`
- `TestProjectBoundaryDomainsDoNotImportListingKitFacade`
- `depguard: project_boundary_listingkit`
- `TestProductDomainDoesNotDependOnOuterAdapters`
- `TestZitadelAuthRuntimeDoesNotImportListingKit`
- `depguard: authruntime_zitadel_listingkit`
- `TestAlibaba1688CrawlerDoesNotImportListingKitRoot`
- `depguard: alibaba1688_listingkit_root`
- `depguard: listingkit_subdomains_root_facade`
- `TestListingKitImportDirectionStaysRetiredAcrossBuildTargets`
- `TestListingKitRootSheinWorkspaceBridgesDoNotImportWorkspaceDomainDirectly`
- `depguard: listingkit_root_workspace_shein`
- `TestListingKitSheinWorkspaceBridgeDoesNotImportLegacyWorkspaceDomain`
- `depguard: listingkit_legacy_shein_runtime`
- `depguard: listingkit_shein_api_root`
- `TestListingKitNonAPISheinImportsStayAllowlisted`
- `TestListingKitAmazonListingImportsStayAllowlisted`
- `depguard: catalog_legacy_productenrich`
- `TestProductEnrichCanonicalImportsStayRetiredAcrossBuildTargets`
- `TestCanonicalTypesDoNotUseProductEnrichCompatibilityAliases`
- `depguard: shein_pipeline_legacy_listingkit`
- `depguard: shein_submitprep_legacy_tenantctx`
- `TestListingKitRootSheinHelpersStayAllowlisted`
- `TestListingKitRootServiceSubmitFilesStayAllowlisted`
- `TestListingKitRootTaskSubmissionFilesStayAllowlisted`
- `TestListingKitRootServiceGenerationFilesStayAllowlisted`
- `TestListingKitRootGenerationFilesStayAllowlisted`
- `depguard: listing_preview_platform_neutral`
- `TestListingPreviewImportsStayPlatformNeutralAcrossBuildTargets`
- `TestTemporalSDKImportsStayInRuntimeAndOrchestrationAdapters`
- `TestTemporalRuntimePackagesDoNotImportHTTPAPI`
- `depguard: temporal_runtime_httpapi`
- `TestProductImageExternalClientImportsStayAllowlisted`
- `TestAmazonExternalClientImportsStayAllowlisted`
- `TestSheinBridgeExternalClientImportsStayAllowlisted`
- `TestSheinRetiredManagementImportsStayBlocked`
- `TestSheinOpenAIImportsStayAllowlisted`
- `TestListingKitHTTPAPIExternalClientImportsStayAllowlisted`
- `TestListingKitSheinSyncLegacyPromotionImportsStayAllowlisted`
- `TestListingKitRootOpenAIImportsStayAllowlisted`
- `depguard: listingkit_root_management_api`
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
- `depguard: publishing_shein_legacy_runtime`
- `depguard: publishing_common_legacy_productenrich`
- `TestProductEnrichCanonicalImportsStayRetiredAcrossBuildTargets`
- `depguard: publishing_common_platforms`
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
- `TestHTTPAPILoginModuleResultAliasesStayRetired`
- `TestHTTPAPIRuntimeStateUsesOwningLoginBootstrapResultTypes`
- `TestHTTPAPIRuntimeStateUsesOwningFeatureHTTPAPIModuleTypes`
- `TestHTTPAPIRuntimeDepsMethodsUseOwningFeatureHTTPAPIModuleTypes`
- `TestHTTPModulesDoNotOwnFeatureHTTPAPIModuleSelectionSignatures`
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
- `depguard: platform_registration_boundaries`
- `TestAICapabilityModuleUsesOnlyApprovedDependencies`
- `depguard: aicapability_boundaries`
- `TestPlatformModulesHistoricalImplementationImportsStayAllowlisted`
- `TestPlatformRegistrationPackagesStayThin`
- `TestPlatformRegistrationPackagesContainNoLocalArtifacts`
- `TestBusinessDomainsDoNotImportAppRuntimeAssembly`
- `TestAppBootstrapRetiredManagementImportsStayBlocked`
- `depguard: listingruntime_local_legacy_management`
- `TestListingRuntimeLocalManagementImportsStayRetiredAcrossBuildTargets`
- `TestAppTaskRetiredManagementImportsStayBlocked`
- `TestAppTaskRuntimeStoreAliasesStayRetired`
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
- `depguard: cmd_domain_dependencies`
- `TestCmdPackagesDoNotImportAppCompatibilityLayers`
- `depguard: cmd_legacy_app_compatibility`
- `depguard: internal_legacy_app_compatibility`
- `TestInternalPackagesDoNotImportAppCompatibilityLayersAcrossBuildTargets`
- `TestAppProcessorCompatibilityLayerIsRetired`
- `TestAppStateCompatibilityLayerIsRetired`
- `TestInfraProductCrawlerAdapterIsRetired`
- `TestAppCrawlerFetcherCompatibilityLayerIsRetired`
- `TestInfrastructurePackagesDoNotImportBusinessDomains`
- `depguard: infrastructure_business_boundaries`
- `TestBusinessImplementationPackagesDoNotImportGinDirectly`
- `TestAppTaskPollingSourceUsesCapabilityNames`
- `TestPlatformProcessorRegistryDoesNotExposeRetiredManagementService`
- `TestAppConsumerTaskStatusRuntimeProviderIsNotNamedRetiredManagementService`
- `TestAppConsumerDoesNotUseManagementNamedTaskStatusAdapter`
- `TestTaskStatusAdapterCallersUseRuntimeNamedConstructor`
- `TestTaskStatusCompatibilityPackageStaysRetired`
- `TestTaskStatusPackageDoesNotExposeManagementNamedAdapter`
- `TestTaskStatusRuntimeErrorsUseCapabilityNames`
- `depguard: app_taskstatus_legacy_management`
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
