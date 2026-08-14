# Remove ListingKit Default SHEIN Store Design

## Goal

Remove the ListingKit/SHEIN "default store" concept so every SHEIN task uses an explicitly selected `shein_store_id`, while preserving store entities, tenant ownership checks, store profiles, and explicit batch/store selection.

## Scope

This change covers the ListingKit service, its HTTP settings contract, settings health reporting, task creation, submission settings resolution, and the ListingKit UI settings surface.

It does not remove:

- SHEIN store records or store administration APIs.
- `ListingKitStoreProfile` records, because they hold per-store site, warehouse, stock, submit-mode, and pricing settings.
- Explicit `shein_store_id` fields on task, batch, group, snapshot, and submission models.
- Tenant ownership and platform validation for an explicitly selected store.
- Unrelated infrastructure defaults such as Temporal's `defaultStore`.

## Current problem

The service currently carries `SheinDefaultStoreID` into `SheinSettings.DefaultStoreID`. Generation request preparation can fill a missing `shein_store_id` from that value. Settings health then reports a missing default store as a blocking SHEIN account error, and the settings schema exposes `default_store_id` as a writable setting.

That creates an implicit store selection path. It also makes the runtime health of a tenant depend on a store choice that should belong to the task or batch being processed.

## Design

### 1. Explicit store is the only task selection source

For requests targeting SHEIN:

1. Normalize the request as today.
2. Do not apply any default store value.
3. Require `req.SheinStoreID > 0` before task persistence or dispatch.
4. Validate that the selected store belongs to the authenticated tenant and is an active SHEIN store.
5. Persist the task and store-resolution snapshot using that explicit store.

Requests targeting non-SHEIN platforms retain their current behavior. Existing explicit multi-platform requests continue to use the same `shein_store_id` for the SHEIN target.

The error must be stable and actionable: `shein_store_id is required for SHEIN tasks`.

### 2. Remove default-store configuration from the service contract

Remove these ListingKit-only concepts:

- `ServiceSheinDependencies.SheinDefaultStoreID`.
- `ServiceConfig.Shein.SheinDefaultStoreID` wiring.
- `generateRequestDefaults.sheinDefaultStoreID` and its automatic mutation path.
- `SheinSettings.DefaultStoreID` and JSON field `default_store_id`.
- The HTTP settings schema field `default_store_id`.
- Settings update behavior that accepts or persists `DefaultStoreID`.

`defaultSheinSettings` will continue to create non-store SHEIN configuration such as site, warehouse, stock, submit mode, and pricing. It will no longer accept or initialize a store ID.

### 3. Store profiles remain explicit per-store configuration

`ListingKitStoreProfile.StoreID` remains required. When a task has an explicit `shein_store_id`, submission settings resolution may load that store's profile and apply its site, warehouse, stock, submit mode, and pricing values.

No profile is selected by priority, fallback flag, or absence of a task store. A missing profile is allowed to use the non-store base settings; it must never supply a store ID.

### 4. Health checks stop requiring a default store

The SHEIN account health item will validate only configuration that is not a store selection:

- site is present;
- default stock is positive;
- submit mode is `publish` or `save_draft`.

It will no longer inspect `DefaultStoreID`, mention "default store", or recommend selecting a default store. The item may still report readiness for these settings even when the tenant has no selected store; individual SHEIN task creation remains responsible for requiring and validating its explicit store.

The health endpoint remains authenticated and continues to report other AI, integration, SDS, object storage, and pricing probe states unchanged.

### 5. UI and API wording

The ListingKit settings page will no longer render a default-store setting or describe a default store. Store selection remains in task/batch/store-specific workflows.

The `GET`/`PUT /api/v1/listing-kits/shein/settings` contract will stop advertising and accepting `default_store_id`. Existing clients sending that field will have it ignored by JSON decoding and will not change server state; the repository will not preserve a compatibility alias because this concept has not been released for the current workflow.

### 6. Existing persisted data

The current default-store value is runtime configuration/state, not a separate database column in the ListingKit settings model. No database migration is required. Existing task and profile `store_id` data remains untouched.

## Data flow

```text
UI task/batch selection
        |
        v
explicit shein_store_id
        |
        v
request validation
        |
        v
tenant + active SHEIN store access validation
        |
        v
task + store-resolution snapshot
        |
        v
store profile lookup by that explicit store
        |
        v
SHEIN submission/runtime client
```

There is no fallback edge from tenant settings to a store ID.

## Error handling and compatibility

- Missing `shein_store_id` for a SHEIN task fails before repository creation and dispatch.
- Invalid, inactive, cross-tenant, or non-SHEIN store IDs continue to use the existing store-access error mapping.
- Submission of an already persisted task continues to prefer its store-resolution snapshot, then its explicit request store, and otherwise fails as unavailable; no new fallback is introduced.
- Non-SHEIN task creation, store administration, store-specific sync endpoints, and Temporal configuration are out of scope.

## Testing strategy

Add or update tests before implementation:

1. Request-default tests prove a missing SHEIN store is not populated from configuration and that explicit IDs are preserved.
2. Task lifecycle tests prove a SHEIN request without `shein_store_id` fails before repository creation; an explicit store continues through access validation.
3. Settings model/service tests prove `SheinSettings` has no default-store field and updates cannot alter a store selection.
4. Settings health tests prove a zero store ID does not block the SHEIN account item, while missing site/stock/submit mode still blocks it.
5. HTTP schema/handler tests prove `default_store_id` is absent from the schema and does not affect updates.
6. UI tests prove the settings page no longer renders default-store controls or copy.
7. Boundary/search tests prove the ListingKit runtime no longer wires or consumes `SheinDefaultStoreID`.

Verification will run focused Go tests for `internal/listingkit` and `internal/listingkit/api`, focused UI tests for the changed settings components, formatting/static checks, then the full Go suite with an explicit timeout and the relevant UI test command.

## Non-goals

- Do not introduce a new global selected-store, current-store, or fallback-store concept.
- Do not make settings health verify that every tenant has a SHEIN store; task-level explicit selection is the correct boundary.
- Do not change 1688 `source_account_id` behavior or add back `source_store_id`.
- Do not deploy, mutate K8s Secrets, or run real-provider task creation as part of this code change.
