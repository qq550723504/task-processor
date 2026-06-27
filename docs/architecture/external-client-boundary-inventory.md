# External Client Boundary Inventory

## Goal

This inventory records the current direct coupling between business-facing
packages and concrete external client adapters under `internal/infra/clients`.
It is not a migration plan and should not trigger broad rewrites by itself.

Use `docs/architecture/project-boundaries.md` as the repository-wide default
for ownership and dependency direction. This inventory is the focused follow-up
document for external-client hotspots and cleanup sequencing; it should narrow
review on concrete adapter coupling, not compete with the top-level boundary
entrypoint.

The goal is to make future cleanup predictable: new code should prefer local
interfaces, while existing direct dependencies should be reduced in narrow
slices when the owning domain boundary is clear.

## Client Families

Current concrete client families that frequently cross domain boundaries:

- `internal/infra/clients/management`
- `internal/infra/clients/openai`
- `internal/infra/clients/nanobanana`

These packages are allowed infrastructure adapters. The architectural risk is
not their existence. The risk is business packages importing adapter types as
part of their domain-facing contracts.

`internal/infra/clients/management` is a management retirement target, not a
long-lived integration direction. New business behavior should prefer
in-repository database/repository access owned by the relevant domain or
assembly layer. Current management allowlists freeze current seams so the
coupling cannot grow silently while each slice moves away from the management
API.

## Hotspots

Current direct dependency hotspots are:

- `internal/listingkit`
  - broad `openai` coupling across facade, studio, task, and settings code
  - a narrow `management` retirement seam remains in `internal/listingkit/httpapi`
    for SHEIN sync runtime strategy wiring
  - a separate legacy promotion bridge seam remains in
    `internal/listingkit/sheinsync/promotion_bridge_legacy_adapter.go` for
    management DTO and legacy SHEIN activity bridge compatibility
  - should shrink behind ListingKit-owned AI interfaces before new features add
    more concrete OpenAI types
  - `internal/listingkit` root OpenAI seams are guarded by
    `TestListingKitRootOpenAIImportsStayAllowlisted`
  - `internal/listingkit/httpapi` AI runtime/bootstrap seams are guarded by
    `TestListingKitHTTPAPIExternalClientImportsStayAllowlisted`
  - `internal/listingkit/httpapi` management retirement seams are guarded by
    `TestListingKitHTTPAPIRetiredManagementImportsStayBlocked`
  - `internal/listingkit/sheinsync` legacy promotion bridge seams are guarded by
    `TestListingKitSheinSyncLegacyPromotionImportsStayAllowlisted`
- `internal/publishing/shein`
  - concentrated `openai` coupling in attribute, category, content, and listing
    copy helpers
  - future publishing rules should depend on local inference interfaces rather
    than concrete adapter packages
  - current direct OpenAI seams are guarded by
    `TestPublishingSheinOpenAIImportsStayAllowlisted`
- `internal/publishing/sheinmanaged`
  - current managed-publishing helpers still import concrete SHEIN API clients
    in builder and category/attribute API factory seams
  - these are managed runtime API construction seams; future publishing logic
    should keep concrete SHEIN API client wiring behind local publishing-owned
    factories instead of spreading adapter types
  - current direct SHEIN API seams are guarded by
    `TestPublishingSheinManagedAPIImportsStayAllowlisted`
  - managed-publishing helpers no longer import retired management service; the legacy
    online runtime factory is retired and the package falls back to offline
    publishing resolution when no runtime API is provided
  - the empty direct-management guard is
    `TestPublishingSheinManagedManagementImportsStayAllowlisted`
- `internal/shein`
  - broad `management` coupling across inventory, scheduler, publish,
    validation, activity, mapping, and product packages
  - direct `openai` coupling also remains in category, content, pipeline,
    product, submit-prep, and translate helpers
  - current direct management seams are guarded by
    `TestSheinRetiredManagementImportsStayBlocked`
  - current direct OpenAI seams are guarded by
    `TestSheinOpenAIImportsStayAllowlisted`
  - cleanup should replace management calls with in-repository
    database/repository access by marketplace slice, not one large adapter
    rewrite
- `internal/amazon`
  - current `management` coupling is concentrated in DTO seams and tests,
    while `openai` coupling is isolated to the current processor and
    LLM adapter path
  - direct external client imports are guarded by
    `TestAmazonExternalClientImportsStayAllowlisted`
  - future Amazon cleanup should introduce Amazon-owned ports before adding
    new concrete management or OpenAI adapter call sites
- `internal/sheinbridge`
  - current sale-attribute bridge still imports concrete `management/api` and
    `openai` clients in its runtime bridge seam
  - this bridge should stay narrow while publishing-facing sale-attribute
    orchestration moves toward bridge-local contracts instead of concrete
    adapter types
  - direct external client imports are guarded by
    `TestSheinBridgeExternalClientImportsStayAllowlisted`
- `internal/temu`
  - broad `management` coupling in sync, pricing, product, store, and scheduler
    paths, plus `openai` coupling in AI, image, SKU, product, and pipeline helpers
  - `internal/temu/sync` and `internal/temu/pricing` management seams are guarded by
    `TestTEMUSyncAndPricingManagementImportsStayAllowlisted`
  - `internal/temu/product`, `internal/temu/store`, and `internal/temu/scheduler`
    management seams are guarded by
    `TestTEMUProductStoreAndSchedulerManagementImportsStayAllowlisted`
  - `internal/temu/api/client`, `internal/temu/context`,
    `internal/temu/bulkrelist`, `internal/temu/filter`,
    `internal/temu/handlerbase`, `internal/temu/rules`, and the root
    `processor.go` runtime seam are guarded by
    `TestTEMURuntimeAndBridgeManagementImportsStayAllowlisted`
  - TEMU OpenAI seams are guarded by `TestTEMUOpenAIImportsStayAllowlisted`
  - cleanup should begin at service constructors and package-local contracts
- `internal/app`
  - expected to construct concrete clients, but should avoid leaking concrete
    client types into business-facing contracts when a narrower port is enough
  - `internal/app/task` no longer imports `management`; task polling, dispatch
    guard, store dispatch, listing-count reads, and status updates are held
    behind task-local runtime capabilities
  - `internal/app/runner` no longer stores a broad retired management service in the
    processor service; remaining management references are limited to runtime
    port compatibility and tests while older processor seams are retired
  - `internal/app/consumer` no longer carries a retired management service through shared
    resources, platform runtime context, or the processor registry
  - `internal/app/bootstrap/resources` no longer constructs `ClientManager` or
    imports the management adapter package; shared runtime assembly now goes
    through the neutral `internal/listingruntime/local` entrypoint for local
    provider/runtime ports
  - `internal/app/httpapi` no longer has a runtime retired-management hook;
    login assembly uses the injected StoreAPI port and task-RPC assembly uses
    local runtime status / retired-unavailable semantics instead of the old
    retired management service
  - `internal/app/runtime/listing` now keeps local runtime health validation
    behind a package-local validator interface; debug task lookup and SHEIN
    recovery watchdog setup use local repository ports instead of carrying a
    concrete retired management service through runtime context
  - `internal/app/taskstatus` no longer imports `management` directly; concrete
    runtime adapters live outside the app task-status service while the service
    keeps only the task-status update contract
  - `internal/taskstatus` no longer imports `management` directly; concrete
    `ClientManager` task-status adapters live in `internal/infra/clients/management`
  - ProductImage model/default provider assembly seams in `internal/app/httpapi`
    are guarded by
    `TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted`
  - `internal/app/task` is guarded by an empty
    `TestAppTaskRetiredManagementImportsStayBlocked` allowlist plus
    task-local capability-name guards
  - `internal/app/runner` management retirement seams are guarded by
    `TestAppRunnerRetiredManagementImportsStayBlocked`
  - `internal/app/consumer` management retirement seams are guarded by
    `TestAppConsumerRetiredManagementImportsStayBlocked`
  - `internal/app/bootstrap` management retirement seams are guarded by
    `TestAppBootstrapRetiredManagementImportsStayBlocked`
  - `internal/listingruntime/local` is the local provider/runtime implementation
    package for bootstrap runtime assembly and is guarded by
    `TestListingRuntimeLocalDoesNotImportRetiredManagementPackage`
  - `internal/app/httpapi` management retirement seams are guarded by
    `TestAppHTTPAPIRetiredManagementImportsStayBlocked`
  - `internal/app/httpapi` OpenAI runtime state and adapter assembly seams are
    guarded by
    `TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated`,
    `TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated`, and
    `TestHTTPAPIRuntimeKeepsOpenAIRuntimeAssemblyDedicated`
  - `internal/app/runtime/listing` management retirement seams are guarded by
    `TestAppRuntimeListingRetiredManagementImportsStayBlocked`
  - `internal/app/taskstatus` management import retirement is guarded by
    `TestAppTaskStatusRetiredManagementImportsStayBlocked`
- `internal/platformtask`
  - current platform task helpers still import `management` in product sync,
    inventory sync, and auto pricing task seams
  - these are platform-task retirement seams, not a long-lived task data
    direction; future platform task data should use in-repository
    database/repository access
  - current direct management seams are guarded by
    `TestPlatformTaskRetiredManagementImportsStayBlocked`
- `internal/platformbase`
  - current platform base factory still imports `management` in the shared
    platform-factory seam
  - this is a platform-factory retirement seam, not a long-lived platform data
    direction; future platform factory data should use in-repository
    database/repository access
  - current direct management seam is guarded by
    `TestPlatformBaseRetiredManagementImportsStayBlocked`
- `internal/processor`
  - processor base no longer imports, stores, or constructs retired management service;
    it exposes only explicitly injected StoreAPI, TaskStatusRuntime, and
    DailyCountClientProvider ports
  - the empty direct-management guard is
    `TestProcessorRetiredManagementImportsStayBlocked`
- `internal/taskrpcapi`
  - task RPC assembly no longer imports retired management service or accepts a
    `ClientProvider`; task status lookup and task actions return explicit
    retired/unavailable semantics while queue stats come from local runtime
    status
  - the empty direct-management guard is
    `TestTaskRPCAPIRetiredManagementImportsStayBlocked`
- `internal/sds/client`
  - SDS auth bootstrap no longer imports retired management service; static bootstrap,
    SDS login-service state, and direct account login remain the supported
    refresh sources
  - the empty direct-management guard is
    `TestSDSClientRetiredManagementImportsStayBlocked`
- `internal/sheinlogin/bootstrap`
  - SHEIN login bootstrap no longer accepts retired management service; store sync and
    duplicate-store lookup use the injected StoreAPI port
  - the empty direct-management guard is
    `TestSheinLoginBootstrapRetiredManagementImportsStayBlocked`
- `internal/sheinlogin`
  - SHEIN login service no longer has retired management service imports in the
    bootstrap path; future login service data should keep using package-local
    ports or in-repository data access
  - the empty direct-management guard is
    `TestSheinLoginServiceRetiredManagementImportsStayBlocked`
- `internal/sheinloginmanaged`
  - managed login bridge no longer imports retired management service; legacy
    `*management.ClientManager` constructors were removed in favor of
    store-port and factory-port constructors
  - the empty direct-management guard is
    `TestSheinLoginManagedRetiredManagementImportsStayBlocked`
- `internal/pricing`
  - current shared pricing helper still imports `management` in the cost-config
    lookup seam
  - this is a shared pricing retirement seam, not a long-lived pricing data
    direction; future pricing config data should use in-repository
    database/repository access
  - current direct management seam is guarded by
    `TestSharedPricingRetiredManagementImportsStayBlocked`
- `internal/state`
  - current state runtime helpers still import `management` in manager and
    daily-count seams
  - these are state-runtime retirement seams, not a long-lived state data
    direction; future state data should use in-repository database/repository
    access
  - current direct management seams are guarded by
    `TestStateRetiredManagementImportsStayBlocked`
- `internal/productimage`
  - uses `openai` and `nanobanana` as provider adapters
  - provider-facing interfaces should stay in the product image domain, with
    concrete adapter construction kept in HTTP/runtime assembly
  - OpenAI-compatible image edit request/response mapping is isolated to
    `internal/productimage/openai_image_edit_adapter.go`; renderer logic should
    use the local image edit port instead of concrete OpenAI client request
    types
  - ProductImage HTTPAPI model provider assembly is isolated to
    `internal/productimage/httpapi/model_provider_builder.go`; module bootstrap
    should delegate provider construction instead of owning concrete provider
    selection
  - current direct adapter seams are guarded by
    `TestProductImageExternalClientImportsStayAllowlisted`

## Local Interface Rule

When adding new behavior that needs a remote service:

1. Put the required capability behind a package-local interface first.
2. Keep concrete adapter construction in app, HTTP, runtime, or narrow
   bootstrap code.
3. Do not add concrete `internal/infra/clients/*` types to public domain
   contracts unless the package is explicitly an adapter boundary.
4. If an existing direct dependency remains, treat it as a migration hotspot,
   not as precedent for new code.

## Next Slice Candidates

Good next slices are small seams where a local interface can replace concrete
adapter types without changing business behavior:

2026-06-24 update: `internal/app/runtime/listing/local_runtime_health.go`
no longer imports `internal/infra/clients/management` directly. The temporary
adapter is `ClientManager.ValidateLocalListingRuntimeFields`, which keeps the
concrete management report type out of listing runtime while the remaining
runtime-owned ports are extracted.

2026-06-26 update: consumer/runtime assembly no longer carries broad
retired management service fields through `PlatformRuntimeContext`, consumer
`SharedResources`, or `PlatformProcessorRegistry`. Bootstrap shared resources
also no longer construct `ClientManager`; they build a local provider/runtime
and expose named store, product, pricing, task, quota, repository, and health
ports. The bootstrap import was later routed through `internal/listingruntime/local`
so app bootstrap no longer imports the management adapter package directly.
Future work should continue retiring the remaining explicit legacy seams
in pricing, platformbase, platformtask, and marketplace compatibility paths.

2026-06-27 update: HTTP task-RPC/login, processor base, taskrpcapi, SHEIN login
bootstrap, and sheinloginmanaged no longer import or accept retired management service.
Their guards are now empty allowlists or explicit no-construction/name guards.
App bootstrap shared resources also route local runtime construction through
`internal/listingruntime/local` instead of importing the management adapter
package directly, and that local runtime package now owns the bootstrap local
provider/runtime implementation instead of delegating to the old client service
package.
Future retirement work should focus on remaining marketplace compatibility
paths such as pricing, platformbase, platformtask, TEMU, SHEIN legacy packages,
and Amazon DTO/test seams.

1. ListingKit AI settings and studio media generation seams currently importing
   `internal/infra/clients/openai`. The HTTPAPI runtime/bootstrap seam is now
   explicitly allowlisted; root facade OpenAI seams are also explicitly
   allowlisted. The next ListingKit cleanup should move one service-facing
   capability at a time behind ListingKit-owned interfaces before adding new
   concrete OpenAI adapter call sites.
2. ListingKit HTTPAPI SHEIN sync runtime strategy wiring currently imports
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current seams; future ListingKit cleanup should move
   this business data access to in-repository database/repository access rather
   than adding new management API call sites.
3. `internal/publishing/sheinmanaged` no longer imports retired management service; the
   managed runtime online factory is retired and the management guard is now an
   empty allowlist.
4. SHEIN publishing attribute/category inference helpers currently importing
   `internal/infra/clients/openai`. Current imports are explicitly allowlisted;
   reduce them by introducing package-local inference interfaces before adding
   new concrete OpenAI adapter call sites.
5. Historical SHEIN inventory, scheduler, publish, validation, activity, mapping,
   product, product sync, store, and managed-client paths currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current seams; future SHEIN cleanup should move one
   marketplace capability at a time to in-repository database/repository access
   before adding new concrete management adapter call sites.
6. Historical SHEIN category, content, pipeline, product, submit-prep, and
   translate helpers currently importing `internal/infra/clients/openai`.
   Current imports are explicitly allowlisted; future SHEIN cleanup should move
   inference and translation capabilities behind package-local interfaces before
   adding new concrete OpenAI adapter call sites.
7. TEMU sync and pricing services currently importing
   `internal/infra/clients/management`. Current sync/pricing imports are explicitly
   allowlisted to freeze current seams; future TEMU cleanup should move
   service-facing management calls to in-repository database/repository access
   before adding new concrete adapter call sites. Product, store, and scheduler
   management imports are also explicitly allowlisted; keep future changes
   behind local interfaces or narrow adapter seams.
8. TEMU AI, image, SKU, product, and pipeline helpers currently importing
   `internal/infra/clients/openai`. Current imports are explicitly allowlisted;
   future feature work should introduce local AI/rewrite/mapping interfaces rather
   than adding new concrete OpenAI adapter call sites.
9. Product image provider construction currently importing `openai` and
   `nanobanana` outside the narrow runtime builder path. Current imports are
   explicitly allowlisted in productimage-owned adapter seams and app/httpapi
   ProductImage assembly seams; renderer logic should stay behind local
   ProductImage ports and must not add new concrete adapter imports without a
   local interface seam or a documented runtime-builder exception.
10. App task source, dispatch, claim, and fetcher seams currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current task-framework seams; future task data access
   should prefer in-repository database/repository access rather than adding new
   management API call sites.
11. App runner scheduler, processor, and health-check assembly seams no longer
   store a broad retired management service. Current imports are explicitly allowlisted
   only for remaining runtime compatibility seams; future runtime data access
   should prefer in-repository database/repository access rather than adding new
   management API call sites.
12. App consumer processor registry, shared-resource, and platform runtime
   seams no longer carry a broad retired management service. Remaining consumer imports
   are explicit retirement seams and should not grow.
13. App bootstrap shared-resource assembly now constructs local runtime ports
   directly instead of `ClientManager`. Remaining bootstrap management imports
   are explicit local-provider/runtime construction seams and should continue to
   shrink toward domain-owned packages.
14. App HTTPAPI runtime dependency and SHEIN login/task-RPC assembly no longer
   carry a retired management service hook. Keep the guard in place and route new
   HTTP-facing runtime data through local ports or in-repository repositories.
15. App listing runtime debug task runner seams currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current debug runtime seams; future listing runtime
   data access should prefer in-repository database/repository access rather
   than adding new management API call sites.
16. App task-status service seams currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current task-status seams; future task-status data
   access should prefer in-repository database/repository access rather than
   adding new management API call sites.
17. Platform task product sync, inventory sync, and auto pricing seams
   currently importing `internal/infra/clients/management`. Current imports are
   explicitly allowlisted to freeze current task seams; future platform task
   data access should prefer in-repository database/repository access rather
   than adding new management API call sites.
18. State runtime manager and daily-count seams currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current state seams; future state data access should
   prefer in-repository database/repository access rather than adding new
   management API call sites.
19. Platform base factory seam currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current factory seam; future platform factory data
   should prefer in-repository database/repository access rather than adding new
   management API call sites.
20. Processor base no longer imports or constructs retired management service. Keep new
   processor data behind explicitly injected StoreAPI, task-status, daily-count,
   or package-local ports.
21. Task RPC build and handler seams no longer import retired management service. New
   task RPC behavior should stay on local runtime status/repository ports rather
   than recreating the retired remote service.
22. SDS auth bootstrap no longer imports retired management service. Static credentials,
   SDS login-service, and direct account login are the supported bootstrap
   sources.
23. SHEIN login bootstrap no longer imports retired management service; store-facing
   login data goes through the StoreAPI port.
24. SHEIN managed-login bridge and account seams no longer import Management
   Client; new managed-login data should use store-port/factory-port
   constructors or in-repository access.
25. Amazon management DTO/context seams and OpenAI LLM adapter seams currently
   import concrete external clients. Current imports are explicitly allowlisted
   to freeze current seams; future Amazon feature work should prefer
   in-repository database/repository access or package-local ports before adding
   new concrete management or OpenAI adapter imports.

## Non-goals

- Do not move every external client import in one refactor.
- Do not create generic global client interfaces that every domain depends on.
- Do not hide useful provider-specific behavior behind an interface that is too
  vague to test.
- Do not add import guards until the current hotspots have either narrow
  allowlists or local interface seams.

## Review Questions

When reviewing a change that touches external clients, ask:

1. Is this package constructing an adapter, or expressing a domain capability?
2. Would a local interface make the call site easier to test?
3. Is a concrete `management`, `openai`, or `nanobanana` type leaking into a
   public domain contract?
4. Does the change reduce coupling in one hotspot, or create a new hotspot?
5. If the dependency is `management`, why is this not using in-repository
   database/repository access yet?
6. Is any remaining direct dependency documented as a temporary exception?
