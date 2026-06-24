# ListingKit SDS Retirement User-Confirmed Delist Design

## Summary

ListingKit needs a user-confirmed remediation flow for cases where an upstream SDS base product or design surface becomes unavailable after ListingKit has already cached baseline data, generated tasks, or prepared SHEIN listings.

The first release targets SHEIN only. The data model and API include a `platform` field so later marketplace support can be added without changing the core retirement workflow.

The system discovers affected records automatically, but the real delist operation is never automatic. A user must review the generated candidate list, choose the affected SKCs and sites, and explicitly confirm execution.

## Goals

- Detect and present SDS base product retirement cases from SDS identifiers:
  - `parent_product_id`
  - `prototype_group_id`
  - `variant_id`
  - optional `selected_variant_ids`
- Show the affected ListingKit tasks, SDS baseline cache entries, and SHEIN synced products.
- Refresh SHEIN product data before execution so delist requests use current SPU, SKC, business model, and site data.
- Let the user confirm which SKCs and sites to delist.
- Call the existing SHEIN `OffShelf` client only after user confirmation.
- Persist an audit trail for each remediation run and item.
- Support retrying failed items without rerunning successful items.

## Non-Goals

- No background auto-delist in the first release.
- No TEMU or Amazon delist execution in the first release.
- No direct database-only shelf status changes as a substitute for SHEIN API execution.
- No deletion of historical ListingKit tasks or SDS baseline cache entries.

## Existing Capabilities To Reuse

- SDS baseline validation already identifies remote failures through reason codes such as `product_detail_check_failed`.
- `listing_kit_sds_baseline_cache` already stores SDS identity and validation state.
- `listing_kit_tasks` stores the original SDS options and result snapshots.
- `listingkit_shein_synced_products` stores synced SHEIN SPU/SKC rows and local shelf state.
- `internal/shein/api/product.Client.OffShelf` already wraps the SHEIN shelf operation endpoint.
- SHEIN sync already refreshes product rows and marks missing products inactive.

## Data Model

Add `listingkit_sds_retirement_runs`.

Important fields:

- `id`
- `tenant_id`
- `platform`
- `store_id`
- `parent_product_id`
- `prototype_group_id`
- `variant_id`
- `selected_variant_ids`
- `baseline_key`
- `validation_status`
- `reason_code`
- `reason`
- `status`
- `created_by`
- `confirmed_by`
- `created_at`
- `confirmed_at`
- `started_at`
- `finished_at`
- `updated_at`

Run statuses:

- `draft`
- `ready`
- `running`
- `succeeded`
- `partially_succeeded`
- `failed`
- `cancelled`

Add `listingkit_sds_retirement_items`.

Important fields:

- `id`
- `run_id`
- `tenant_id`
- `platform`
- `store_id`
- `task_id`
- `synced_product_id`
- `spu_name`
- `skc_name`
- `skc_code`
- `supplier_code`
- `shelf_status_before`
- `selected`
- `site_selection`
- `request_snapshot`
- `response_snapshot`
- `status`
- `error`
- `started_at`
- `finished_at`
- `created_at`
- `updated_at`

Item statuses:

- `pending`
- `selected`
- `running`
- `succeeded`
- `succeeded_already_off_shelf`
- `failed`
- `skipped`

`site_selection` stores the exact `off_sub_sites` payload selected by the user. `request_snapshot` stores the final SHEIN `ShelfOperateRequest` payload summary used for execution.

## Backend Flow

### Create Or Refresh Run

Endpoint shape:

`POST /api/v1/listing-kits/sds/retirements`

Input:

- SDS identity
- `tenant_id`
- `platform=shein`
- `store_id`
- optional `source_task_id`

Behavior:

1. Validate SDS identity.
2. Resolve baseline key using the existing SDS baseline identity logic.
3. Run existing SDS baseline readiness checks.
4. Query `listing_kit_tasks` and `listing_kit_sds_baseline_cache` for references to the SDS identity.
5. Refresh SHEIN product data for the target store before building items.
6. Match affected SHEIN rows using known ListingKit identifiers:
   - task result SHEIN package identifiers where available
   - synced product rows by SPU/SKC/SKU/supplier code
   - source SDS SKU attributes from canonical product snapshots when available
7. Build retirement items with all currently known SPU/SKC/site data.
8. Set run status to `ready` when at least one actionable item exists, otherwise keep `draft` with a reason explaining why no SHEIN product was found.

If SDS validation fails because of a transient network or login issue, the run should not become executable. It should remain `draft` with the validation reason so the user can retry detection.

### Preview

Endpoint shape:

`GET /api/v1/listing-kits/sds/retirements/:run_id`

Returns:

- run summary
- SDS validation state
- affected ListingKit tasks
- affected baseline cache entries
- retirement items
- default site selections
- execution eligibility

Default selection behavior:

- Select every active item whose current SHEIN shelf status is `ON_SHELF`.
- Select all known SHEIN sites for each SKC.
- Do not select items already `OFF_SHELF`; show them as already handled.

### Update Selection

Endpoint shape:

`PATCH /api/v1/listing-kits/sds/retirements/:run_id/items`

Behavior:

- Let the user select or deselect items.
- Let the user adjust selected sites for each item.
- Reject updates after the run enters `running`, `succeeded`, `partially_succeeded`, or `failed`.

### Confirm And Execute

Endpoint shape:

`POST /api/v1/listing-kits/sds/retirements/:run_id/confirm`

Behavior:

1. Lock the run.
2. Require at least one selected executable item.
3. Mark run `running`.
4. For each selected item:
   - build `sheinproduct.ShelfOperateRequest`
   - call `ProductAPI.OffShelf`
   - record request and response summary
   - update item status
   - update `listingkit_shein_synced_products` to `shelf_status=OFF_SHELF` and `is_active=false` only after SHEIN success or already-off-shelf confirmation
5. Mark run `succeeded`, `partially_succeeded`, or `failed`.

Failed items can be retried through a retry endpoint that only re-executes failed items.

## Frontend Flow

Add an SDS retirement entry point from:

- SDS product browser or selection details
- failed ListingKit task detail when the error maps to SDS product/detail/design unavailable

Primary screen flow:

1. User opens the retirement flow for an SDS base product.
2. The page creates or refreshes a retirement run.
3. The page displays:
   - SDS identity
   - failure reason
   - impacted tasks
   - impacted baseline cache entries
   - SHEIN products to delist
4. All actionable items and all known sites are selected by default.
5. User can deselect SKCs or sites.
6. User confirms execution.
7. Page shows per-item progress and final result.

The UI should make the destructive operation explicit. The primary confirmation button should remain disabled until the user has at least one selected executable item and acknowledges that SHEIN products will be delisted.

## Error Handling

- SDS validation transient failure:
  - show run as not executable
  - allow refresh detection
- No SHEIN products found:
  - keep run as `draft`
  - show affected tasks/cache but no executable items
- SHEIN auth/cookie failure:
  - mark affected items failed
  - keep run retryable
- Item already off shelf:
  - mark `succeeded_already_off_shelf`
  - update local synced product row if needed
- Partial SHEIN failure:
  - run becomes `partially_succeeded`
  - successful items are not retried
  - failed items can be retried

## Audit And Safety

- Store actor identity on run creation and confirmation.
- Store the exact selected item and site set before execution.
- Store request and response summaries per item.
- Use row-level locking or an equivalent transaction boundary so the same run cannot execute twice concurrently.
- Treat direct local shelf state updates as a post-success mirror of SHEIN state, not as the source of truth.

## Testing

Backend tests:

- SDS identity resolves the expected baseline key.
- A product-detail failure creates a non-automatic, user-confirmable run.
- Detection collects affected tasks and cache entries.
- SHEIN refresh populates actionable items.
- Default selection includes active `ON_SHELF` items and excludes already off-shelf items.
- `OffShelf` request payload includes SPU, SKC, business model, and selected `off_sub_sites`.
- Partial failure results in `partially_succeeded`.
- Retry executes only failed items.
- Local synced product rows update only after SHEIN success.

Frontend tests:

- Preview renders SDS failure, tasks, cache entries, and affected products.
- All sites are selected by default.
- User can deselect SKCs and individual sites.
- Confirmation is blocked without selected executable items.
- Success, partial failure, and retry states render clearly.

Integration-style tests:

- Use a stub SHEIN `ProductAPI` for execution.
- Use repository tests for run/item persistence.
- Do not call real SHEIN or SDS APIs in automated tests.

## Open Extension Points

- Add scheduled detection later by creating runs in `draft` without executing them.
- Add TEMU or Amazon by implementing platform-specific item discovery and execution adapters.
- Add notifications after run creation or completion.
