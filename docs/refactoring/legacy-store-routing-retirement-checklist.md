# Legacy Store Routing Retirement Checklist

> Status: prep only. This note tracks the remaining compatibility surfaces for `/store-routing` after runtime behavior and frontend entrypoints were removed.

## Goal

Retire legacy SHEIN store routing settings safely, without breaking:

- historical task / preview snapshots,
- existing handler / route tests,
- constructor and repository wiring,
- any external caller still depending on `/api/v1/listing-kits/store-routing`.

## Already Done

- Runtime store selection no longer depends on store routing settings.
- Frontend no longer exposes store routing UI or task-create auto-routing behavior.
- Legacy routing code paths are isolated into dedicated `*_legacy*` files.
- `httpapi` and `service` wiring now label this dependency as `legacy`.

## Remaining Compatibility Surfaces

### 1. Public HTTP contract

These still expose the legacy settings endpoint:

- `internal/listingkit/httpapi/routes.go`
- `internal/listingkit/api/store_routing_legacy_handler.go`
- `internal/app/httpapi/server_test.go`
- `internal/listingkit/httpapi/http_module_test.go`

Retirement decision needed:

- remove the route entirely, or
- keep a compatibility shell that always returns default/manual settings.

### 2. Service interfaces

These still require the legacy admin capability in interface form:

- `internal/listingkit/interfaces.go`
- `internal/listingkit/service_shein_store_routing_legacy_entrypoints.go`
- `internal/listingkit/settings_admin_store_routing_legacy_service.go`

Retirement decision needed:

- remove the methods from the public service contract, or
- keep them behind a narrower compatibility-only interface.

### 3. Repository and persistence layer

These still persist legacy routing settings:

- `internal/listingkit/store_routing_legacy.go`
- `internal/listingkit/store_routing_legacy_repository.go`
- `internal/listingkit/httpapi/builders.go`
- `internal/listingkit/httpapi/runtime_support_repositories.go`

Retirement decision needed:

- remove persistence and default to synthesized manual settings, or
- keep repository support until external callers are confirmed gone.

### 4. Service constructor / DI

These still inject the legacy dependency:

- `internal/listingkit/service_types.go`
- `internal/listingkit/service_config.go`
- `internal/listingkit/service_defaults.go`
- `internal/listingkit/service_admin_wiring.go`
- `internal/listingkit/httpapi/bootstrap.go`
- `internal/listingkit/httpapi/bootstrap_repositories.go`
- `internal/listingkit/httpapi/bootstrap_service_config.go`

Retirement decision needed:

- drop the dependency from `ServiceCoreDependencies`, or
- keep it until the HTTP/service compatibility layer is removed.

### 5. Tests and stubs

These still assert or stub legacy routing behavior:

- `internal/listingkit/store_profile_service_test.go`
- `internal/listingkit/service_test.go`
- `internal/listingkit/service_wiring_test.go`
- `internal/listingkit/api/store_profile_handler_test.go`
- `internal/listingkit/api/task_recovery_handler_test.go`
- `internal/listingkit/api/stub_generation_task_service_customer_flow_test.go`

Retirement decision needed:

- rewrite these to validate removal behavior, or
- keep them until the compatibility API is intentionally deleted.

## Recommended Removal Order

1. Confirm whether any non-frontend caller still uses `/api/v1/listing-kits/store-routing`.
2. If no caller remains, convert the handler/service methods into a compatibility shell or remove them.
3. Remove legacy routing methods from service interfaces and route handler interfaces.
4. Remove constructor and bootstrap wiring for legacy routing repositories.
5. Remove persistence types/repositories and associated tests.

## Safe First Cut

If we want a low-risk next implementation slice, prefer this order:

1. Keep the route path.
2. Make handler/service return synthesized manual defaults instead of persisted settings.
3. Delete repository wiring and persistence only after tests are updated.

That keeps the external API stable while collapsing internal maintenance cost.
