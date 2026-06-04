# ListingKit SHEIN Product Sync And Activity Enrollment Design

## Goal

Add a ListingKit-owned SHEIN product sync capability that:

- periodically syncs only `已上架` SHEIN products into ListingKit
- supports manual sync with the same backend flow
- stores a durable local product mirror inside ListingKit
- carries forward cost-price data with `auto import + manual override` precedence
- produces activity-enrollment candidates for SHEIN campaigns
- reuses the existing `internal/shein/activity` enrollment logic instead of rebuilding campaign registration rules
- supports both `审核后执行` and `全自动执行` enrollment modes

The result should give ListingKit a stable operational base for campaign enrollment without depending on the older management-product sync path as its source of truth.

## Problem

Today the project has useful but disconnected capabilities:

- `internal/shein/productsync` can fetch SHEIN product data and push it into an external management DTO flow
- `internal/shein/activity` already contains mature SHEIN campaign-enrollment logic
- ListingKit has its own HTTP, repository, runtime, and admin patterns, but no ListingKit-native SHEIN catalog mirror for campaign operations

This creates four root problems.

### 1. ListingKit does not own the product truth it needs for campaign work

The business goal is campaign enrollment from ListingKit. That requires ListingKit to persist:

- synced SHEIN products
- effective cost price
- enrollment candidate state
- enrollment execution history

At the moment those concepts do not exist as first-class ListingKit records.

### 2. The existing SHEIN product sync is aimed at a different destination

`internal/shein/productsync` is designed around management-side DTO persistence. Even where the API fetching logic is useful, the storage and lifecycle assumptions are not aligned with ListingKit's operating model.

### 3. Activity enrollment logic already exists, but its upstream product source is wrong for the new workflow

The current `internal/shein/activity` logic is valuable and should be reused. The problem is not the campaign algorithms. The problem is that campaign execution must be fed from ListingKit's own synchronized catalog and cost-price state, not from older external assumptions.

### 4. Manual and scheduled operations could easily diverge

If manual sync, scheduled sync, manual enrollment, and auto enrollment are implemented as separate paths, the system will drift. That would make results inconsistent and debugging expensive.

The design must instead make trigger mode a parameter, not a separate business implementation.

## Decision

Introduce a ListingKit-owned SHEIN operations slice with three backend stages:

1. `catalog sync`
2. `candidate generation`
3. `activity enrollment`

ListingKit becomes the owner of:

- synced SHEIN on-shelf product mirror records
- cost-price precedence and persistence
- sync job history
- candidate pool state
- enrollment execution history
- manual and scheduled orchestration

The project reuses, rather than rewrites:

- SHEIN product, inventory, and price API clients from `internal/shein/api/product`
- mature campaign rule and submission logic from `internal/shein/activity`

This yields one shared operational flow:

`SHEIN on-shelf sync -> ListingKit local mirror -> effective cost-price resolution -> candidate generation -> manual approval or auto enrollment -> enrollment run history`

## Non-Goals

This design does not include:

- syncing SHEIN draft or off-shelf products in phase 1
- replacing the existing `internal/shein/activity` business rules with a new campaign engine
- migrating historical management-system product data into ListingKit before the first sync
- redesigning the full ListingKit frontend in this spec
- supporting every possible SHEIN campaign type on day one if the existing enrollment code already has phased capability boundaries

## Existing Reusable Building Blocks

### SHEIN Product Fetching

ListingKit should reuse:

- `internal/shein/api/product.Client.ListProducts`
- inventory and price query capabilities in the same package

The synced source must explicitly use `shelf_type=ON_SHELF` so the ListingKit mirror matches the real `已发布商品 / 已上架` business target.

### SHEIN Activity Enrollment

ListingKit should reuse the mature activity layer from:

- `internal/shein/activity/registration.go`
- `internal/shein/activity/registration_config.go`
- related time-limited and mixed-activity helpers where needed

The migration target is not a raw copy-paste. The intended outcome is:

- keep SHEIN-facing campaign configuration and submission behavior
- replace the upstream product selection source with ListingKit-synchronized products and ListingKit cost-price truth

### ListingKit Infrastructure Patterns

New ListingKit persistence and routes should follow the existing project patterns:

- `Record + Repository + Mem/Gorm`
- HTTP route registration through `internal/listingkit/httpapi`
- service composition through ListingKit bootstrap/runtime wiring

## Product Model

### 1. Synced Product Mirror

Add a ListingKit-native record such as `SheinSyncedProductRecord`.

Table name suggestion:

- `listingkit_shein_synced_products`

One record represents one operational SHEIN product unit for enrollment. The uniqueness boundary should be:

- `tenant_id + store_id + skc_name`

Core fields:

- `id`
- `tenant_id`
- `store_id`
- `spu_name`
- `spu_code`
- `skc_name`
- `skc_code`
- `supplier_code`
- `category_id`
- `brand_name`
- `product_name_multi`
- `main_image_url`
- `sale_name`
- `shelf_status`
- `publish_time`
- `first_shelf_time`
- `currency`
- `price_snapshot`
- `inventory_snapshot`
- `site_snapshot`
- `auto_cost_price`
- `manual_cost_price`
- `effective_cost_price`
- `cost_price_source`
- `sync_version`
- `last_sync_at`
- `is_active`
- `created_at`
- `updated_at`

Key rules:

- `manual_cost_price` always overrides `auto_cost_price`
- `effective_cost_price` is a stored projection for query convenience, not an independent source of truth
- products missing both auto and manual cost price stay synced but are not auto-enrollable
- if a product disappears from the latest on-shelf sync, do not hard-delete immediately; mark inactive

### 2. Sync Job History

Add `SheinSyncJobRecord`.

Table name suggestion:

- `listingkit_shein_sync_jobs`

Core fields:

- `id`
- `tenant_id`
- `store_id`
- `trigger_mode` with values `manual` or `schedule`
- `status`
- `started_at`
- `finished_at`
- `fetched_count`
- `inserted_count`
- `updated_count`
- `deactivated_count`
- `skipped_count`
- `error_summary`
- `created_at`
- `updated_at`

This is the durable history surface for both operators and debugging.

### 3. Enrollment Candidate Pool

Add `SheinActivityCandidateRecord`.

Table name suggestion:

- `listingkit_shein_activity_candidates`

Core fields:

- `id`
- `tenant_id`
- `store_id`
- `synced_product_id`
- `activity_type`
- `activity_key`
- `candidate_version`
- `effective_cost_price`
- `price_snapshot`
- `inventory_snapshot`
- `calculated_profit_rate`
- `eligibility_status`
- `eligibility_reason`
- `review_status`
- `auto_mode_eligible`
- `selected_for_run`
- `created_at`
- `updated_at`

Suggested state split:

- `eligibility_status`: `eligible`, `ineligible`
- `review_status`: `pending_review`, `approved`, `rejected`, `auto_queued`, `enrolled`, `failed`

This table separates:

- "does it qualify?"
- "has a human approved it?"
- "has it already been submitted?"

### 4. Enrollment Run And Items

Add:

- `SheinActivityEnrollmentRunRecord`
- `SheinActivityEnrollmentItemRecord`

Table name suggestions:

- `listingkit_shein_activity_enrollment_runs`
- `listingkit_shein_activity_enrollment_items`

Run-level fields:

- `id`
- `tenant_id`
- `store_id`
- `activity_type`
- `trigger_mode` with values such as `manual_confirmed` or `auto_schedule`
- `status`
- `candidate_count`
- `submitted_count`
- `succeeded_count`
- `failed_count`
- `started_at`
- `finished_at`
- `error_summary`
- `created_at`
- `updated_at`

Item-level fields:

- `id`
- `run_id`
- `candidate_id`
- `synced_product_id`
- `skc_name`
- `status`
- `request_payload`
- `response_payload`
- `error_message`
- `created_at`
- `updated_at`

These records make enrollment execution inspectable and retryable.

## Cost Price Precedence

ListingKit must support:

1. automatic cost-price import
2. manual override

The precedence rule is fixed:

- if `manual_cost_price` exists, use it
- otherwise use `auto_cost_price`
- if neither exists, the product remains unsatisfied for auto enrollment

Store the chosen source in `cost_price_source`:

- `manual`
- `auto`
- `none`

Automatic cost-price import should be abstracted behind a small ListingKit-facing interface, for example:

- `CostPriceResolver.ResolveByStoreAndSKC(...)`

This avoids coupling ListingKit product mirror persistence directly to one external data source.

## Sync Flow

Provide one shared service:

- `SyncSheinOnShelfProducts(ctx, tenantID, storeID, triggerMode)`

Both manual and scheduled sync must call this service.

### Sync Steps

1. resolve store and SHEIN access context
2. create a sync job record
3. fetch SHEIN products page by page using `shelf_type=ON_SHELF`
4. enrich the fetched list with inventory and price snapshots where needed
5. resolve automatic cost prices
6. upsert synced product mirror rows
7. mark no-longer-present rows inactive
8. finalize sync job counters and status

### Sync Behavior Rules

- sync is store-scoped
- sync is idempotent
- repeated sync should update snapshots and timestamps, not create duplicates
- manual sync and scheduled sync should produce the same resulting mirror state
- missing downstream enrichments such as inventory or price should degrade gracefully where possible and be logged, not necessarily abort the whole sync

## Candidate Generation Flow

Provide a ListingKit service such as:

- `RefreshSheinActivityCandidates(ctx, tenantID, storeID, activityType)`

This service reads from the ListingKit product mirror and writes the ListingKit candidate pool.

### Candidate Steps

1. load active on-shelf synced products for the store
2. drop products without `effective_cost_price`
3. apply inventory and campaign prerequisites
4. invoke reusable enrollment-rule helpers from `internal/shein/activity` where possible
5. compute profit and price-risk eligibility
6. upsert candidate rows
7. write explicit ineligible reasons for operator visibility

### Candidate Rules

- candidate generation is deterministic for a given product snapshot and strategy version
- candidate generation should not itself submit anything to SHEIN
- candidates may exist in `eligible but not yet approved` state
- auto-enrollment only reads from candidates already marked auto-eligible

## Enrollment Flow

Provide one shared execution service:

- `ExecuteSheinActivityEnrollment(ctx, storeID, activityType, triggerMode, candidateIDs...)`

This service is called by:

- manual operator-confirmed enrollment
- scheduled auto-enrollment

### Enrollment Steps

1. create an enrollment run
2. load approved or auto-queued candidates
3. map ListingKit candidates into the input shape expected by the migrated `internal/shein/activity` logic
4. invoke the reused campaign configuration and submission logic
5. persist item-level results
6. update candidate statuses
7. finalize run summary

### Enrollment Modes

#### Review-Then-Run

Operators:

- sync products
- review candidates
- fill or fix manual cost price
- approve selected candidates
- trigger enrollment

#### Full Auto

Scheduled execution:

- sync products
- refresh candidates
- select candidates matching auto-enrollment policy
- execute enrollment

The same backend service runs both modes. Only candidate selection and trigger source differ.

## API Surface

Add ListingKit route families under the existing ListingKit HTTP module.

### Sync

- `POST /api/v1/listing-kits/shein-sync/stores/:store_id/sync`
  - manually trigger a sync
- `GET /api/v1/listing-kits/shein-sync/stores/:store_id/jobs`
  - list sync jobs
- `GET /api/v1/listing-kits/shein-sync/stores/:store_id/products`
  - list synced products with filtering

### Cost Price

- `PATCH /api/v1/listing-kits/shein-sync/products/:id/cost`
  - set or clear manual cost price

### Candidates

- `POST /api/v1/listing-kits/shein-sync/stores/:store_id/candidates/refresh`
  - recompute candidate pool
- `GET /api/v1/listing-kits/shein-sync/stores/:store_id/candidates`
  - list candidates and reasons
- `PATCH /api/v1/listing-kits/shein-sync/candidates/:id/review`
  - approve or reject one candidate

### Enrollment

- `POST /api/v1/listing-kits/shein-sync/stores/:store_id/enrollments`
  - execute manual enrollment for selected candidates
- `GET /api/v1/listing-kits/shein-sync/stores/:store_id/enrollment-runs`
  - list enrollment runs
- `GET /api/v1/listing-kits/shein-sync/enrollment-runs/:run_id`
  - view run details

## Scheduling

Add two schedulable store-scoped jobs.

### 1. Product Sync Job

Purpose:

- keep the ListingKit SHEIN mirror fresh

Behavior:

- trigger `SyncSheinOnShelfProducts(..., triggerMode=schedule)`

### 2. Auto Enrollment Job

Purpose:

- refresh candidates and submit eligible items when the store policy allows it

Behavior:

1. sync latest products
2. refresh candidates
3. execute enrollment for auto-queued candidates

### Concurrency And Idempotency

Use store-scoped locking so:

- one store cannot run concurrent syncs
- one store cannot run concurrent enrollment executions for the same activity type

Recommended idempotency identity for enrollment items:

- `store_id + activity_key + skc_name + candidate_version`

This prevents duplicate submissions when scheduled jobs overlap or retries occur.

## Migration Strategy

The design should be implemented in phases.

### Phase 1

- ListingKit mirror persistence
- manual sync
- synced product browsing
- cost-price import plus manual override

### Phase 2

- candidate pool generation
- manual review and manual enrollment
- enrollment run history

### Phase 3

- scheduled sync
- scheduled auto candidate refresh
- full auto enrollment with store-level policy toggle

This sequencing keeps business risk lower while still reusing the final architecture.

## Error Handling

### Sync

- page fetch failures fail the sync job
- enrichment failures may be partial if the core product mirror can still be updated
- sync job summaries must preserve enough context for operator diagnosis

### Candidate Generation

- rule-evaluation problems should produce explicit candidate ineligibility reasons where possible
- catastrophic strategy-resolution errors fail the refresh operation

### Enrollment

- run-level status must distinguish partial success from full failure
- item-level failures must remain inspectable
- candidate statuses must not claim success unless SHEIN submission succeeded

## Testing Strategy

### Unit Tests

- cost-price precedence
- mirror upsert identity rules
- inactive product marking
- candidate eligibility computation
- auto-vs-manual candidate selection
- idempotency key behavior

### Service Tests

- manual sync and scheduled sync produce the same mirror state
- candidate refresh uses ListingKit mirror, not an external management list
- manual override cost price wins over imported cost price
- enrollment run updates candidate and run/item state correctly

### Integration Tests

- adapter from ListingKit candidate rows into reused `internal/shein/activity` enrollment inputs
- end-to-end store-scoped sync -> candidate -> enrollment happy path using mocked SHEIN APIs

## Open Implementation Notes

- If `internal/shein/activity` currently assumes management-client product sourcing, extract a narrow interface so ListingKit can supply its own candidate-backed product set without rewriting campaign logic.
- The ListingKit persistence layer should follow the project's existing `Mem/Gorm + AutoMigrate` repository pattern to keep local testing and production wiring aligned.
- ListingKit admin and operator surfaces should reuse the existing ListingKit route-module structure instead of creating a parallel HTTP module.

## Success Criteria

The feature is complete when:

- ListingKit can manually sync only SHEIN on-shelf products for a store
- scheduled sync can run the same backend flow safely
- synced products persist in ListingKit with effective cost-price precedence
- operators can review candidates and manually trigger enrollment
- stores can opt into full auto enrollment using the same execution path
- enrollment behavior reuses the mature SHEIN campaign logic instead of duplicating it
