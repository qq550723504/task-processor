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
| `listingkit_support.go` | `assembly-only` | Shapes `listingkithttpapi.RuntimeBuildInput` and delegates feature-owned support building to `BuildRuntimeSupport(...)`. |
| `feature_builder_listingkit.go` | `assembly-only` | Composes ProductEnrich, ProductImage, and ListingKit runtime modules without embedding ListingKit behavior. |
| `listingkit_temporal_worker.go` | `assembly-only` | Boots the standalone ListingKit Temporal worker process and delegates feature/runtime construction. |
| `runtime_support_listingkit.go` | `adapter construction` | Prepares shared prerequisites such as the SHEIN cookie store, SDS sync service, and SDS baseline remote provider for the feature-owned runtime support contract. |

Current app-layer read:

- no obvious ListingKit business rules remain in `internal/app/httpapi`,
- the app layer is mostly assembling modules and preparing shared runtime prerequisites,
- `runtime_support_listingkit.go` is the file to keep watching so it does not drift from prerequisite prep into ListingKit-specific policy.

## `internal/listingkit/httpapi`

| File | Classification | Notes |
| --- | --- | --- |
| `bootstrap.go` | `assembly-only` | Public bundle/module bootstrap entry after store-catalog runtime mapping moved into a dedicated support helper. |
| `bootstrap_repositories.go` | `assembly-only` | Repository bundle construction and closer management. |
| `bootstrap_service_config.go` | `assembly-only` | ListingKit service config shaping and dependency wiring. |
| `bootstrap_runtime.go` | `assembly-only` | Service/module runtime assembly and handler construction after recovery sweep and submit-prep global setup were pushed into dedicated runtime support helpers. |
| `bootstrap_submit_module.go` | `assembly-only` | Submit module dependency shaping. |
| `bootstrap_task_module.go` | `assembly-only` | Task module dependency shaping. |
| `bootstrap_temporal_module.go` | `assembly-only` | Temporal worker/module registration assembly. |
| `runtime_builder.go` | `assembly-only` | Public runtime build entry. |
| `runtime_module.go` | `assembly-only` | Runtime module assembly wrapper. |
| `temporal_runtime.go` | `assembly-only` | Temporal runtime build wrapper around feature-owned assembly. |
| `http_module.go` | `assembly-only` | HTTP module construction and validation. |
| `routes.go` | `assembly-only` | Route registration and route table ownership; should stay thin. |
| `runtime_support.go` | `adapter construction` | Feature-owned runtime support contract that gathers repository/hook bundles and optional SDS collaborators. |
| `runtime_support_recovery.go` | `adapter construction` | Owns runtime-only task recovery sweep loop wiring and shutdown behavior so bootstrap assembly stays focused on composition. |
| `runtime_support_repositories.go` | `adapter construction` | Owns ListingKit runtime support repository bundle construction. |
| `runtime_support_hooks.go` | `adapter construction` | Owns ListingKit runtime support hook bundle construction. |
| `runtime_support_store_catalog.go` | `adapter construction` | Owns runtime-side mapping from listing-admin store records into ListingKit-facing SHEIN store catalog shapes. |
| `runtime_support_submit_prep.go` | `adapter construction` | Owns SHEIN submit-prep global runtime configuration that still needs to happen during feature boot. |
| `runtime_support_shein.go` | `adapter construction` | Builds SHEIN runtime resolver/bridge builders while delegating factory/provider details to narrower helper files. |
| `runtime_support_shein_adapter_helpers.go` | `adapter construction` | Owns SHEIN runtime adapter-local tenant lookup, cookie payload normalization, and store-config mapping helpers. |
| `runtime_support_shein_factories.go` | `adapter construction` | Owns SHEIN runtime API client factories and bound cookie-provider helpers used by the runtime support layer. |
| `shein_sync_runtime.go` | `adapter construction` | Builds SHEIN sync services and composes the narrower strategy/bridge helpers. |
| `shein_sync_runtime_bridge_helpers.go` | `adapter construction` | Owns SHEIN sync runtime bridge shaping helpers, including tenant parsing and promotion bridge factory construction. |
| `shein_sync_runtime_strategy_helpers.go` | `adapter construction` | Owns management strategy-provider construction for the SHEIN sync runtime path. |
| `ai_clients.go` | `adapter construction` | Owns top-level ListingKit AI client builder entrypoints and routed client assembly. |
| `ai_client_fallback_helpers.go` | `adapter construction` | Owns ListingKit AI client fallback shaping, fallback sanitizing, and client-name normalization helpers. |
| `ai_client_image_routing.go` | `adapter construction` | Owns ListingKit image-client routing, selector normalization, and image timeout clamping helpers. |
| `ai_client_strict_chat.go` | `adapter construction` | Owns strict chat-client wrapper behavior and resolved OpenAI chat-client cache construction. |
| `ai_client_strict_image.go` | `adapter construction` | Owns strict image-client wrapper behavior and resolved image-client cache construction. |
| `zitadel_auth.go` | `adapter construction` | Runtime auth/authz middleware construction and route-level authorization wiring. |
| `zitadel_auth_parsing_helpers.go` | `adapter construction` | Owns ZITADEL role parsing, allowlist/set normalization, and shared identity value helpers. |

## Follow-Up Candidates

### Highest-signal candidate

`internal/listingkit/httpapi/shein_sync_runtime.go`

Why it stands out:

- it is still mostly adapter assembly,
- the remaining bridge shaping has already been narrowed into a dedicated helper file,
- management strategy-provider construction has also been split into its own helper file,
- if more branching lands there, it could become the next mixed runtime hotspot.

Suggested next slice:

- keep service construction in place, and continue extracting any new tenant/bridge shaping helpers instead of letting them drift back into the main assembly file.

### Additional candidate

`internal/listingkit/httpapi/ai_clients.go`

Why it stands out:

- it now mostly owns builder entrypoints and routed client assembly,
- strict chat/image client wrappers and cache resolution have been pushed into dedicated helper files,
- fallback shaping and naming are already isolated,
- if more request-shaping or model-selection rules land there, they should stay in helper homes rather than re-grow the main builder file.

### Additional note

`runtime_support.go` has now been narrowed to the runtime support contract itself; repository, hook, submit-prep, and recovery-loop setup live in dedicated helper files so the top-level support file stays easier to read and review.

## Current Boundary Conclusion

At this checkpoint:

- `internal/app/httpapi` is mostly in the right place and should not be widened,
- the default SHEIN store heuristic should be feature-owned in `internal/listingkit`, not `httpapi`-owned,
- the most meaningful remaining cleanup is inside feature-owned runtime adapter helpers under `internal/listingkit/httpapi`,
- the next safe refactor should target suspicious shaping helpers, not reopen the already-stable app-layer assembly split.
