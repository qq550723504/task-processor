# HTTPAPI Runtime Inventory

## Purpose

This inventory is the `Phase 5.1` checkpoint for ListingKit runtime assembly cleanup.

Goal:

- keep `internal/app/httpapi` focused on runtime assembly and prerequisite preparation,
- keep `internal/listingkit/httpapi` focused on feature-owned assembly and adapter construction,
- highlight the remaining files where runtime shaping is starting to look like domain logic.

## Classification Rubric

- `assembly-only`: construction, registration, composition, or module/runtime wiring only
- `adapter construction`: builds infrastructure-facing adapters, factories, or support contracts for the feature
- `suspicious mixed responsibility`: still in an assembly/runtime package, but already owns shaping or defaults that may belong in a narrower feature or adapter home

## `internal/app/httpapi`

| File | Classification | Notes |
| --- | --- | --- |
| `feature_builder_listingkit.go` | `assembly-only` | Composes ProductEnrich, ProductImage, and ListingKit runtime modules, including ListingKit runtime input shaping, without embedding ListingKit behavior. |
| `listingkit_temporal_worker.go` | `assembly-only` | Boots the standalone ListingKit Temporal worker process and delegates feature/runtime construction. |
| `runtime_support_listingkit.go` | `adapter construction` | Prepares shared prerequisites such as the SHEIN cookie store, SDS sync service, and SDS baseline remote provider for the feature-owned runtime support contract. |

Current app-layer read:

- no obvious ListingKit business rules remain in `internal/app/httpapi`,
- the app layer is mostly assembling modules and preparing shared runtime prerequisites,
- `runtime_support_listingkit.go` is the file to keep watching so it does not drift from prerequisite prep into ListingKit-specific policy.

## `internal/listingkit/httpapi`

Current package shape: 74 non-test Go files.

| File group | Classification | Current files | Notes |
| --- | --- | --- | --- |
| Bootstrap module assembly | `assembly-only` | `bootstrap.go`, `bootstrap_admin_module.go`, `bootstrap_module_service.go`, `bootstrap_runtime.go`, `bootstrap_service_config.go`, `bootstrap_submit_module.go`, `bootstrap_task_module.go`, `bootstrap_temporal_module.go`, `bootstrap_validation.go` | Public module/runtime/service entrypoints and validation. Keep these as composition and dependency checks only. |
| Bootstrap repository wiring | `assembly-only` | `bootstrap_repositories_admin.go`, `bootstrap_repositories_contracts.go`, `bootstrap_repositories_core.go`, `bootstrap_repositories_merge.go`, `bootstrap_closers.go`, `bootstrap_contracts.go` | Repository bundle construction, closer management, and local bootstrap contracts. No domain rules should move here. |
| DB/repository builders | `adapter construction` | `builders_repositories.go`, `builders_repositories_admin.go`, `builders_repositories_listingkit.go`, `builders_repositories_support.go`, `builders_db_admin_repositories.go`, `builders_db_listingkit_repositories.go`, `builders_db_repository_support.go`, `builders_db_support_repositories.go`, `builders_repository_schema.go`, `builders_image_store.go`, `builders_legacy_tenant.go`, `builders_recovery.go` | Adapter construction around DB-backed repositories, image stores, legacy tenant bridging, and recovery collaborators. |
| HTTP module and routes | `assembly-only` | `http_module.go`, `routes_handler.go`, `routes_admin.go`, `routes_task.go`, `routes_settings.go`, `routes_shein_sync.go`, `routes_store_subscription.go` | Handler contract fragments and route registration surfaces. These should remain route ownership, not business policy. |
| Route descriptors | `assembly-only` | `routes_descriptor_entrypoints.go`, `routes_descriptor_task.go`, `routes_descriptor_settings.go`, `routes_descriptor_shein_sync.go`, `routes_descriptor_store_subscription.go`, `routes_descriptor_admin.go`, `routes_descriptor_admin_catalog.go`, `routes_descriptor_admin_rules.go`, `routes_descriptor_admin_store.go`, `routes_descriptor_admin_topics.go` | Route table/descriptor ownership, including settings health descriptor registration. |
| Runtime entrypoints | `assembly-only` | `runtime_builder.go`, `runtime_module.go`, `temporal_runtime.go` | Public runtime/module/Temporal wrappers around feature-owned assembly. |
| Runtime support contract | `adapter construction` | `runtime_support.go`, `runtime_support_hooks.go`, `runtime_support_recovery.go`, `runtime_support_repositories.go`, `runtime_support_store_catalog.go`, `runtime_support_submit_prep.go` | Feature-owned runtime support construction and prerequisite setup. Keep as adapter reporting and boot-time wiring. |
| SHEIN runtime support | `adapter construction` | `runtime_support_shein.go`, `runtime_support_shein_adapter_helpers.go`, `runtime_support_shein_factories.go` | Runtime resolver/bridge builders, tenant/cookie/store config adapters, and SHEIN API factory binding. |
| Settings health | `adapter construction` | `settings_health_probes.go` | Runtime capability probe construction from config and submit-module availability; not business readiness policy. |
| SHEIN sync runtime | `adapter construction` | `shein_sync_runtime.go`, `shein_sync_runtime_bridge_helpers.go`, `shein_sync_runtime_strategy_helpers.go` | SHEIN sync service construction, bridge factory/shaping helpers, enrollment adapter construction, and management strategy-provider construction. |
| AI clients | `adapter construction` | `ai_clients.go`, `ai_client_builders.go`, `ai_client_fallback_helpers.go`, `ai_client_image_routing.go`, `ai_client_strict_chat.go`, `ai_client_strict_image.go` | ListingKit AI client entrypoints, strict client builders, routing, strict wrappers, timeout/fallback shaping, and cache construction. |
| ZITADEL auth | `adapter construction` | `zitadel_auth.go`, `zitadel_auth_middleware.go`, `zitadel_auth_parsing_helpers.go`, `zitadel_auth_route_authorization.go`, `zitadel_auth_runtime.go` | Runtime auth/authz middleware construction, route authorization wiring, and role/allowlist parsing helpers. |

## Follow-Up Candidates

### Highest-signal candidate

`internal/listingkit/httpapi/shein_sync_runtime.go`

Why it stands out:

- it is now mostly service assembly,
- bridge shaping, bridge-factory construction, and enrollment adapter construction live in a dedicated helper file,
- management strategy-provider construction has also been split into its own helper file,
- if more branching lands there, it could become a mixed runtime hotspot again.

Suggested next slice:

- keep service construction in place, and continue routing any new tenant/bridge/adapter shaping helpers into the existing helper files instead of letting them drift back into the main assembly file.

### Additional candidate

`internal/listingkit/httpapi/ai_clients.go`

Why it stands out:

- it now mostly owns public builder entrypoints,
- concrete strict chat/image/nanobanana builders now live in `ai_client_builders.go`,
- routed image client assembly and selector handling now live in `ai_client_image_routing.go`,
- strict chat/image client wrappers and cache resolution have been pushed into dedicated helper files,
- fallback shaping and naming are already isolated,
- if more request-shaping or model-selection rules land there, they should stay in helper homes rather than re-grow the entrypoint file.

### Additional note

`runtime_support.go` has now been narrowed to the runtime support contract itself; repository, hook, submit-prep, and recovery-loop setup live in dedicated helper files so the top-level support file stays easier to read and review.

### 2026-06-17 closeout note

The settings-health endpoint is now wired through feature-owned ListingKit HTTPAPI files:

- `routes_descriptor_settings.go` owns only route descriptor registration.
- `routes_settings.go` owns only the handler contract fragment.
- `settings_health_probes.go` owns runtime capability probe construction from config and submit-module availability.
- `internal/app/httpapi` continues to consume the ListingKit module contract only; its test stub was synchronized with `GetSettingsHealth` during checkpoint validation.
- direct `internal/listingkit` root imports from non-test app/httpapi files are now guarded and limited to `runtime_support_listingkit.go` plus `types.go`; new ListingKit feature logic should move into `internal/listingkit/httpapi` or domain packages instead.
- direct `internal/listingkit/httpapi` imports from non-test app/httpapi files are also guarded and limited to the current module, runtime, route, and server assembly files.

This keeps health reporting close to the ListingKit runtime support boundary while avoiding app-layer business policy ownership.

### 2026-06-17 inventory refresh

The current `internal/listingkit/httpapi` package has been refreshed from the live file list after the bootstrap, repository builder, route descriptor, ZITADEL auth, AI client, and SHEIN sync helper splits.

- The old one-file rows for `bootstrap_repositories.go` and `routes.go` were replaced by grouped current files because those responsibilities now live across narrower helper files.
- The package still classifies as feature-owned assembly plus adapter construction; this refresh did not identify app-layer business policy drift.
- `shein_sync_runtime.go` and `ai_clients.go` remain watchlist entrypoints, but their bridge/strategy/fallback/routing/strict-wrapper details are already held in narrower helper files.

## Current Boundary Conclusion

At this checkpoint:

- `internal/app/httpapi` is mostly in the right place and should not be widened,
- the `internal/listingkit/httpapi` live file map has been refreshed and remains feature-owned runtime assembly plus adapter construction,
- the default SHEIN store heuristic should be feature-owned in `internal/listingkit`, not `httpapi`-owned,
- settings-health probe construction is feature-owned adapter reporting, not app-layer policy,
- the most meaningful remaining cleanup is inside feature-owned runtime adapter helpers under `internal/listingkit/httpapi`,
- the next safe refactor should target newly introduced suspicious shaping helpers, not continue splitting already-thin entrypoints or reopen the stable app-layer assembly split.
