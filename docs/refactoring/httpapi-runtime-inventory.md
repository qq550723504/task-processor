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
| `bootstrap.go` | `assembly-only` | Public bundle/module bootstrap entry. |
| `bootstrap_repositories.go` | `assembly-only` | Repository bundle construction and closer management. |
| `bootstrap_service_config.go` | `assembly-only` | ListingKit service config shaping and dependency wiring. |
| `bootstrap_runtime.go` | `assembly-only` | Service/module runtime assembly and handler construction. |
| `bootstrap_submit_module.go` | `assembly-only` | Submit module dependency shaping. |
| `bootstrap_task_module.go` | `assembly-only` | Task module dependency shaping. |
| `bootstrap_temporal_module.go` | `assembly-only` | Temporal worker/module registration assembly. |
| `runtime_builder.go` | `assembly-only` | Public runtime build entry. |
| `runtime_module.go` | `assembly-only` | Runtime module assembly wrapper. |
| `temporal_runtime.go` | `assembly-only` | Temporal runtime build wrapper around feature-owned assembly. |
| `http_module.go` | `assembly-only` | HTTP module construction and validation. |
| `routes.go` | `assembly-only` | Route registration and route table ownership; should stay thin. |
| `runtime_support.go` | `adapter construction` | Feature-owned runtime support contract that gathers repositories, hooks, and optional SDS collaborators. |
| `runtime_support_shein.go` | `adapter construction` | Builds SHEIN runtime adapters and runtime bridge factories; shaping helpers have been extracted so the main file can stay constructor-oriented. |
| `runtime_support_shein_adapter_helpers.go` | `adapter construction` | Owns SHEIN runtime adapter-local tenant lookup, cookie payload normalization, and store-config mapping helpers. |
| `shein_sync_runtime.go` | `adapter construction` | Builds SHEIN sync services and promotion-bridge runtime factories; still assembly-heavy, but worth watching if more tenant/store branching lands here. |
| `ai_clients.go` | `adapter construction` | Builds routed OpenAI chat/image clients and runtime client resolution caches. |
| `zitadel_auth.go` | `adapter construction` | Runtime auth/authz middleware construction; transport/runtime concern, not ListingKit business logic. |

## Follow-Up Candidates

### Highest-signal candidate

`internal/listingkit/httpapi/shein_sync_runtime.go`

Why it stands out:

- it is still mostly adapter assembly,
- but it also owns tenant parsing and some local bridge/runtime shaping,
- if more branching lands there, it could become the next mixed runtime hotspot.

Suggested next slice:

- keep service construction in place, but extract tenant/bridge shaping helpers if the file grows further.

## Current Boundary Conclusion

At this checkpoint:

- `internal/app/httpapi` is mostly in the right place and should not be widened,
- the default SHEIN store heuristic should be feature-owned in `internal/listingkit`, not `httpapi`-owned,
- the most meaningful remaining cleanup is inside feature-owned runtime adapter helpers under `internal/listingkit/httpapi`,
- the next safe refactor should target suspicious shaping helpers, not reopen the already-stable app-layer assembly split.
