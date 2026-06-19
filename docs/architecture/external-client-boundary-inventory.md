# External Client Boundary Inventory

## Goal

This inventory records the current direct coupling between business-facing
packages and concrete external client adapters under `internal/infra/clients`.
It is not a migration plan and should not trigger broad rewrites by itself.

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
  - should shrink behind ListingKit-owned AI interfaces before new features add
    more concrete OpenAI types
  - `internal/listingkit` root OpenAI seams are guarded by
    `TestListingKitRootOpenAIImportsStayAllowlisted`
  - `internal/listingkit/httpapi` AI runtime/bootstrap seams are guarded by
    `TestListingKitHTTPAPIExternalClientImportsStayAllowlisted`
  - `internal/listingkit/httpapi` management retirement seams are guarded by
    `TestListingKitHTTPAPIManagementClientImportsStayAllowlisted`
- `internal/publishing/shein`
  - concentrated `openai` coupling in attribute, category, content, and listing
    copy helpers
  - future publishing rules should depend on local inference interfaces rather
    than concrete adapter packages
  - current direct OpenAI seams are guarded by
    `TestPublishingSheinOpenAIImportsStayAllowlisted`
- `internal/publishing/sheinmanaged`
  - current managed-publishing helpers still import `management` to build
    category, attribute, sale-attribute, and API factory seams
  - these are management retirement seams, not a long-lived publishing
    direction; future managed publishing data should use in-repository
    database/repository access
  - current direct management seams are guarded by
    `TestPublishingSheinManagedManagementImportsStayAllowlisted`
- `internal/shein`
  - broad `management` coupling across inventory, scheduler, publish,
    validation, activity, mapping, and product packages
  - direct `openai` coupling also remains in category, content, pipeline,
    product, submit-prep, and translate helpers
  - current direct management seams are guarded by
    `TestSheinManagementClientImportsStayAllowlisted`
  - current direct OpenAI seams are guarded by
    `TestSheinOpenAIImportsStayAllowlisted`
  - cleanup should replace management calls with in-repository
    database/repository access by marketplace slice, not one large adapter
    rewrite
- `internal/amazon`
  - current `management` coupling is concentrated in context/DTO seams and
    tests, while `openai` coupling is isolated to the current processor and
    LLM adapter path
  - direct external client imports are guarded by
    `TestAmazonExternalClientImportsStayAllowlisted`
  - future Amazon cleanup should introduce Amazon-owned ports before adding
    new concrete management or OpenAI adapter call sites
- `internal/temu`
  - broad `management` coupling in sync, pricing, product, store, and scheduler
    paths, plus `openai` coupling in AI, image, SKU, product, and pipeline helpers
  - `internal/temu/sync` and `internal/temu/pricing` management seams are guarded by
    `TestTEMUSyncAndPricingManagementImportsStayAllowlisted`
  - `internal/temu/product`, `internal/temu/store`, and `internal/temu/scheduler`
    management seams are guarded by
    `TestTEMUProductStoreAndSchedulerManagementImportsStayAllowlisted`
  - TEMU OpenAI seams are guarded by `TestTEMUOpenAIImportsStayAllowlisted`
  - cleanup should begin at service constructors and package-local contracts
- `internal/app`
  - expected to construct concrete clients, but should avoid leaking concrete
    client types into business-facing contracts when a narrower port is enough
  - `internal/app/task` still imports `management` in task source, dispatch,
    claim, and fetcher seams; these are task-framework retirement seams and new
    task data access should move toward in-repository database/repository access
  - `internal/app/runner` still imports `management` in scheduler, processor,
    and health-check runtime assembly seams; these should stay narrow while
    runtime data access moves toward in-repository database/repository access
  - `internal/app/consumer` still imports `management` in processor registry,
    RabbitMQ service, task handler, shared-resource, and auto-shard seams; these
    should remain explicit retirement seams while consumer data access moves
    toward in-repository database/repository access
  - `internal/app/bootstrap` still imports `management` in top-level app,
    scheduler factory, scheduler dependency, and shared-resource assembly seams;
    these should stay narrow while bootstrap data access moves toward
    in-repository database/repository access
  - `internal/app/httpapi` still imports `management` in runtime dependency
    methods and SHEIN module test seams; these should stay narrow while HTTP
    assembly data access moves toward in-repository database/repository access
  - `internal/app/runtime/listing` still imports `management` in debug task
    runner seams; these should remain explicit runtime retirement seams while
    listing runtime data access moves toward in-repository database/repository
    access
  - `internal/app/taskstatus` still imports `management` in task status service
    seams; these should remain explicit retirement seams while task-status data
    access moves toward in-repository database/repository access
  - ProductImage model/default provider assembly seams in `internal/app/httpapi`
    are guarded by
    `TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted`
  - `internal/app/task` management retirement seams are guarded by
    `TestAppTaskManagementClientImportsStayAllowlisted`
  - `internal/app/runner` management retirement seams are guarded by
    `TestAppRunnerManagementClientImportsStayAllowlisted`
  - `internal/app/consumer` management retirement seams are guarded by
    `TestAppConsumerManagementClientImportsStayAllowlisted`
  - `internal/app/bootstrap` management retirement seams are guarded by
    `TestAppBootstrapManagementClientImportsStayAllowlisted`
  - `internal/app/httpapi` management retirement seams are guarded by
    `TestAppHTTPAPIManagementClientImportsStayAllowlisted`
  - `internal/app/runtime/listing` management retirement seams are guarded by
    `TestAppRuntimeListingManagementClientImportsStayAllowlisted`
  - `internal/app/taskstatus` management retirement seams are guarded by
    `TestAppTaskStatusManagementClientImportsStayAllowlisted`
- `internal/platformtask`
  - current platform task helpers still import `management` in product sync,
    inventory sync, and auto pricing task seams
  - these are platform-task retirement seams, not a long-lived task data
    direction; future platform task data should use in-repository
    database/repository access
  - current direct management seams are guarded by
    `TestPlatformTaskManagementClientImportsStayAllowlisted`
- `internal/platformbase`
  - current platform base factory still imports `management` in the shared
    platform-factory seam
  - this is a platform-factory retirement seam, not a long-lived platform data
    direction; future platform factory data should use in-repository
    database/repository access
  - current direct management seam is guarded by
    `TestPlatformBaseManagementClientImportsStayAllowlisted`
- `internal/processor`
  - current processor base still imports `management` in the shared processor
    seam
  - this is a processor retirement seam, not a long-lived processor data
    direction; future processor data should use in-repository
    database/repository access
  - current direct management seam is guarded by
    `TestProcessorManagementClientImportsStayAllowlisted`
- `internal/taskrpcapi`
  - current task RPC assembly still imports `management` in build and handler
    seams
  - this is a task-RPC retirement seam, not a long-lived task transport data
    direction; future task RPC data should use in-repository
    database/repository access
  - current direct management seams are guarded by
    `TestTaskRPCAPIManagementClientImportsStayAllowlisted`
- `internal/sds/client`
  - current SDS auth bootstrap still imports `management` in the shared SDS
    bootstrap seam
  - this is an SDS bootstrap retirement seam, not a long-lived SDS state data
    direction; future SDS state data should use in-repository
    database/repository access
  - current direct management seam is guarded by
    `TestSDSClientManagementClientImportsStayAllowlisted`
- `internal/sheinlogin/bootstrap`
  - current SHEIN login bootstrap still imports `management` in the shared
    login bootstrap seam
  - this is a login bootstrap retirement seam, not a long-lived login state
    data direction; future login bootstrap data should use in-repository
    database/repository access
  - current direct management seams are guarded by
    `TestSheinLoginBootstrapManagementClientImportsStayAllowlisted`
- `internal/sheinlogin`
  - current SHEIN login service package still imports `management` in
    bootstrap and login-service seams
  - these are login-service retirement seams, not a long-lived login state
    data direction; future login service data should use in-repository
    database/repository access
  - current direct management seams are guarded by
    `TestSheinLoginServiceManagementClientImportsStayAllowlisted`
- `internal/sheinloginmanaged`
  - current managed login bridge still imports `management` in bridge and
    account seams
  - these are managed-login retirement seams, not a long-lived login data
    direction; future managed-login data should use in-repository
    database/repository access
  - current direct management seams are guarded by
    `TestSheinLoginManagedManagementClientImportsStayAllowlisted`
- `internal/pricing`
  - current shared pricing helper still imports `management` in the cost-config
    lookup seam
  - this is a shared pricing retirement seam, not a long-lived pricing data
    direction; future pricing config data should use in-repository
    database/repository access
  - current direct management seam is guarded by
    `TestSharedPricingManagementClientImportsStayAllowlisted`
- `internal/state`
  - current state runtime helpers still import `management` in manager and
    daily-count seams
  - these are state-runtime retirement seams, not a long-lived state data
    direction; future state data should use in-repository database/repository
    access
  - current direct management seams are guarded by
    `TestStateManagementClientImportsStayAllowlisted`
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
3. `internal/publishing/sheinmanaged` category, attribute, sale-attribute, and
   API factory seams currently import `internal/infra/clients/management`.
   Current imports are explicitly allowlisted to freeze current seams; future
   managed publishing cleanup should move this business data access to
   in-repository database/repository access.
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
11. App runner scheduler, processor, and health-check assembly seams currently
   importing `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current runtime assembly seams; future runtime data
   access should prefer in-repository database/repository access rather than
   adding new management API call sites.
12. App consumer processor registry, RabbitMQ service, task handler,
   shared-resource, and auto-shard seams currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current consumer runtime seams; future consumer data
   access should prefer in-repository database/repository access rather than
   adding new management API call sites.
13. App bootstrap application, scheduler factory, scheduler dependency, and
   shared-resource assembly seams currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current application assembly seams; future bootstrap
   data access should prefer in-repository database/repository access rather
   than adding new management API call sites.
14. App HTTPAPI runtime dependency and SHEIN module test seams currently
   importing `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current HTTP assembly seams; future HTTP-facing data
   access should prefer in-repository database/repository access rather than
   adding new management API call sites.
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
20. Processor base seam currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current processor seam; future processor data should
   prefer in-repository database/repository access rather than adding new
   management API call sites.
21. Task RPC build and handler seams currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current RPC seam; future task RPC data should prefer
   in-repository database/repository access rather than adding new management
   API call sites.
22. SDS auth bootstrap seam currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current SDS bootstrap seam; future SDS state data
   should prefer in-repository database/repository access rather than adding new
   management API call sites.
23. SHEIN login bootstrap seam currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current login bootstrap seam; future login bootstrap
   data should prefer in-repository database/repository access rather than
   adding new management API call sites.
24. SHEIN managed-login bridge and account seams currently importing
   `internal/infra/clients/management`. Current imports are explicitly
   allowlisted to freeze current managed-login seams; future managed-login data
   should prefer in-repository database/repository access rather than adding new
   management API call sites.
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
