# ListingKit SDS Baseline Grouped Bulk Create Design

## Goal

Support grouped SDS-to-SHEIN bulk creation by allowing only SDS products with a prepared baseline cache to enter grouped creation, while keeping the existing ListingKit execution model of one task per SDS product and one store per task.

## Scope

This design covers phase 1 only:

- define an SDS baseline cache for stable product skeleton data
- gate grouped bulk creation on baseline readiness
- reuse baseline data during ListingKit standard-product workflow
- keep grouped creation as orchestration that expands into ordinary ListingKit tasks

This phase does not merge multiple SDS products into one SHEIN listing. It also does not redesign the final SHEIN submit flow, revision workflow, or store routing model.

## Problem

The current SDS-to-SHEIN flow assumes one SDS product selection per task. That assumption exists in both the studio frontend and the backend workflow:

- frontend task creation builds one `selection` into one ListingKit request
- backend `GenerateRequest` carries one SDS context
- `runStandardProductWorkflow` prepares one canonical product
- SHEIN assembly builds one package from that canonical product

At the same time, the user workflow now needs to group multiple SDS products and send each product in the group to the same target store. The stable parts of these products are already mostly known after the SDS-to-standard-product conversion:

- category path
- stable product attributes and specifications
- variant matrix
- sales attributes
- pricing-relevant fields

The unstable parts are usually:

- rendered or selected images
- some title or description copy
- design-specific SDS output

The current canonical product cache is not a good fit for this reuse pattern because its fingerprint is derived from request-level image URLs, text, and product URL. That means small image or copy changes can miss cache even when the SDS product skeleton is unchanged.

## Design

Phase 1 introduces a dedicated `SDS baseline cache` that stores the stable product skeleton required to build ListingKit and SHEIN outputs for a known SDS product selection. Grouped bulk creation will only accept products whose baselines are already prepared.

The baseline cache becomes a stable reuse boundary between:

- the expensive SDS-to-standard-product preparation work
- the dynamic per-run image and copy overlay work

During grouped bulk creation, the system does not build a new multi-product task shape. Instead, it expands each group into multiple ordinary ListingKit tasks. Each task still represents:

- one SDS product selection
- one canonical product
- one SHEIN package
- one resolved SHEIN store

This keeps the workflow compatible with existing package assembly, submit readiness, revision history, and store routing behavior.

## Baseline Cache Model

Add a dedicated persistence model for SDS baseline data, for example:

- table: `listing_kit_sds_baseline_cache`
- repository interface inside `internal/listingkit`

Each baseline entry stores:

- tenant identity
- SDS identity
- stable canonical product base
- optional SHEIN stable resolutions that can be reused safely
- baseline status and metadata
- source task id and timestamps

The payload should be purpose-built for baseline reuse instead of reusing the entire `ListingKitResult`. The key idea is to persist the stable skeleton, not the whole request result.

### Recommended Payload

- `tenant_id`
- `baseline_key`
- `status`
- `version`
- `source_task_id`
- `sds_identity`
- `canonical_product_base`
- `category_resolution`
- `attribute_resolution`
- `sale_attribute_resolution`
- `pricing_snapshot`
- `created_at`
- `updated_at`

### SDS Identity

The SDS identity should be stable across image and copy changes. It should include:

- `parent_product_id`
- `prototype_group_id`
- primary `variant_id`
- normalized `selected_variant_ids`

If future behavior needs stronger separation, a baseline version can also include a schema version and optional product-shape version.

## Baseline Key

Phase 1 should not reuse the existing canonical-product request fingerprint. Instead, build a new baseline key from stable SDS identity:

- `tenant_id`
- `parent_product_id`
- `prototype_group_id`
- sorted `selected_variant_ids`

This key intentionally ignores image URLs, generated text, and transient design copy. Those belong to the dynamic overlay phase, not to baseline identity.

## Baseline Readiness Rules

A product is `baseline ready` when:

- a baseline entry exists for the SDS identity
- the entry status is `ready`
- required SDS identity fields are present
- the canonical product base is non-empty

An entry is `not ready` when:

- no baseline exists
- baseline build failed
- baseline version is stale and must be rebuilt

Phase 1 can keep staleness simple. The first implementation may treat all existing ready entries as valid until an explicit rebuild is requested or baseline schema version changes.

## Workflow Changes

`runStandardProductWorkflow` should gain a new first-class baseline path:

1. inspect the incoming SDS request identity
2. resolve the SDS baseline key
3. if a ready baseline exists, hydrate canonical product base from baseline
4. apply runtime SDS overlays for this run
5. continue existing workflow assembly

The runtime overlay step keeps current dynamic behavior:

- apply rendered or selected SDS images
- apply title updates derived from SDS product detail
- preserve any dynamic image strategy output

This means the baseline provides the stable skeleton, and the existing SDS overlay helpers continue to handle per-run variation.

## SHEIN Resolution Reuse

Phase 1 may store stable SHEIN resolutions inside the baseline when they are safe to reuse:

- category resolution
- display attribute resolution
- sale attribute resolution

This fits the current resolver-cache direction, which already computes identity from stable SDS and package data. Baseline persistence should not replace resolver caches outright. Instead, it becomes a higher-level reuse source that can prefill package state before normal review and submit readiness checks run.

If a stored resolution cannot be applied cleanly because runtime overlays change the package shape, the workflow should fall back to existing resolver behavior rather than failing the task.

## Grouped Bulk Creation Behavior

Grouped bulk creation remains an orchestration feature, not a new task model.

Each group has:

- a group id or client-side temporary id
- a target SHEIN store
- a list of SDS product selections

When creation starts:

- only baseline-ready SDS selections are accepted
- each accepted selection becomes one ordinary ListingKit task request
- the request carries the group-selected `shein_store_id`

This preserves the current backend assumptions while allowing operations to manage products in grouped batches.

## Frontend Behavior

The studio UI should expose baseline readiness explicitly.

Phase 1 frontend rules:

- only baseline-ready SDS products can be added to a group
- non-ready products are shown as unavailable for grouped bulk creation
- grouped creation displays how many products are ready vs blocked
- blocked products show a clear reason such as `baseline missing` or `baseline failed`

The frontend does not need to synthesize a new backend multi-product model. It can submit grouped selections and let the backend expand them into ordinary tasks, or it can call the existing create flow repeatedly behind a grouped operation wrapper.

## Backend API Direction

Phase 1 should add an explicit grouped bulk-create path rather than overloading the existing single-selection helper in ambiguous ways.

Recommended request shape:

- groups
- each group includes `shein_store_id`
- each group includes a list of SDS selections

The server expands this into normal ListingKit task creation. The response should preserve per-group and per-selection success or failure details so the UI can explain partial completion.

## Error Handling

Grouped creation should fail softly per selection whenever possible.

Examples:

- missing baseline for one product should not cancel all groups
- invalid store id for one group should fail that group and leave others untouched
- stale baseline schema can return a clear rebuild-needed status

The UI should distinguish:

- not eligible to create
- create request failed
- task created but later workflow failed

## Testing

Phase 1 should add coverage for:

- SDS baseline key generation stability
- baseline repository read and write behavior
- standard workflow baseline hit path
- standard workflow fallback path when no baseline exists
- grouped bulk-create validation that rejects non-ready products
- grouped expansion into ordinary ListingKit tasks with correct store assignment

Frontend coverage should prove:

- only ready products can enter groups
- blocked products are surfaced with reasons
- grouped creation request payload preserves per-group store ids

## Open Decisions Fixed For Phase 1

To keep scope controlled, phase 1 fixes these decisions:

- grouped bulk creation creates multiple ordinary listings, not one merged listing
- baseline identity is SDS-product-driven, not image-driven
- baseline invalidation is manual or version-driven, not automatic SDS diff detection
- runtime overlays still own images and dynamic copy

## Validation

- add targeted tests around baseline lookup and grouped eligibility
- keep existing ListingKit workflow and SHEIN package tests green
- verify that baseline hits skip expensive standard-product regeneration while preserving dynamic image and title overlays
