# ListingKit SHEIN Activity Enrollment UI Design

## Overview

This spec defines the first frontend release for the `listingkit` SHEIN activity enrollment workflow.

The backend sync and enrollment foundation already exists:

- SHEIN on-shelf product sync into ListingKit local mirror records
- cost price persistence with auto/manual precedence
- candidate pool refresh
- manual and scheduled enrollment execution
- enrollment run history

The goal of this frontend phase is to give operators a practical daily workflow for:

1. choosing a SHEIN store
2. syncing published/on-shelf products
3. filling or overriding cost prices
4. reviewing activity candidates
5. manually enrolling selected products into activities
6. reviewing enrollment results

This UI is intentionally scoped around the current business goal: activity enrollment readiness and execution. It is not a generic product management console.

## Goals

- Add a dedicated `SHEIN 活动报名` area inside ListingKit UI
- Support multi-store overview with drill-down into a single store workbench
- Support the manual operational loop:
  - sync products
  - maintain cost prices
  - refresh candidates
  - approve/reject candidates
  - manually enroll selected candidates
  - inspect enrollment history
- Reuse existing ListingKit shell, query, route, and panel patterns
- Keep automatic enrollment visible in the UI model, but not deeply configurable in v1

## Non-Goals

- No activity-detail-first workflow in v1
- No cross-store bulk enrollment in v1
- No advanced chart-heavy BI dashboard
- No complex batch import flow for cost prices in v1
- No full automatic enrollment rules editor in v1
- No separate activity configuration UX beyond using backend defaults

## User Workflow

### Daily operator flow

1. Open `SHEIN 活动报名`
2. Review store summaries
3. Enter a store that needs attention
4. Run `立即同步` if needed
5. Fix missing or incorrect cost prices
6. Run `刷新候选池`
7. Review candidate reasons and statuses
8. Select approved candidates
9. Execute `报名活动`
10. Check the enrollment run result

### Failure-oriented flow

1. Open a store
2. Go to `报名记录`
3. Open the latest failed run
4. See which SKCs failed and why
5. Return to `候选池` or `成本价维护`
6. correct data
7. retry

## Information Architecture

## Navigation

Add a dedicated ListingKit navigation entry:

- `SHEIN 活动报名`

This area is separate from the task/workspace flow because it represents an operations workflow over synced store products rather than individual ListingKit generation tasks.

## Page structure

### 1. Multi-store overview

Route:

- `/listing-kits/shein-enrollment`

Purpose:

- show which stores need attention
- provide a lightweight cross-store operational dashboard
- route the operator into a single-store workbench

### 2. Single-store workbench

Route:

- `/listing-kits/shein-enrollment/[storeId]`

Tabs via search params:

- `/listing-kits/shein-enrollment/[storeId]?tab=products`
- `/listing-kits/shein-enrollment/[storeId]?tab=costs`
- `/listing-kits/shein-enrollment/[storeId]?tab=candidates`
- `/listing-kits/shein-enrollment/[storeId]?tab=runs`

Default tab:

- `candidates`

This default matches the operator goal most closely: after entering a store, the highest-value question is usually "what can I enroll now?"

## Screen Design

### Multi-store overview

Each store card should show:

- store name
- account/store identifier
- last sync time
- synced product count
- missing cost product count
- pending-review candidate count
- enrollable candidate count
- latest enrollment status

Top-level actions:

- search/filter stores
- open store workbench

Visual priority:

- stores with missing cost prices or pending candidates should stand out first
- stores with stale sync timestamps should be clearly identifiable

v1 does not need dense charts. Use compact metric cards or list rows.

### Single-store workbench header

The store header should display:

- store name
- platform/store account label
- last sync time
- synced count
- missing cost count
- pending-review count
- enrollable count

Primary actions:

- `立即同步`
- `刷新候选池`

Secondary action:

- `自动报名` switch shown but default off

The auto-enroll control is included because it is part of the long-term model, but v1 treats it as a status/control placeholder rather than a full automation settings center.

### Tab: 同步商品

Purpose:

- inspect the latest synced published/on-shelf product mirror
- verify that sync is healthy
- quickly jump into cost maintenance

Columns:

- main image
- product name
- SPU/SKC identifiers
- shelf status
- publish time
- last sync time
- cost price status

Interactions:

- search by product name, SPU, or SKC
- filter by cost status
- jump to cost tab for a selected product

### Tab: 成本价维护

Purpose:

- resolve the main operational blocker before enrollment

Default focus:

- missing cost prices
- manual overrides

Columns:

- product identity
- auto cost price
- manual cost price
- effective cost price
- cost source

Interactions:

- inline edit single product manual cost price
- batch edit multiple selected products
- clear manual override when needed

The UI must clearly distinguish:

- auto cost price
- manual cost price
- effective cost price

This is important because manual values intentionally override imported values.

### Tab: 候选池

Purpose:

- serve as the core v1 action surface

Columns:

- product identity
- effective cost price
- price snapshot summary
- inventory snapshot summary
- eligibility status
- review status
- reason
- auto-mode eligibility

Interactions:

- filter by review status
- filter by eligibility
- select multiple rows
- approve/reject a candidate
- mark candidate selected for run
- execute `报名活动` on selected rows

The reason field is critical and should be immediately visible, not buried in a modal. Typical examples:

- missing effective cost price
- product is not on shelf
- insufficient inventory
- duplicate executable candidate

### Tab: 报名记录

Purpose:

- inspect run-level and item-level outcomes

Run list fields:

- execution time
- activity type/key
- trigger mode
- status
- candidate count
- submitted count
- succeeded count
- failed count

Run detail fields:

- SKC
- candidate version
- item status
- request/response payload summary
- failure reason

v1 can render run details inline, in an expansion panel, or in a simple detail drawer.

## Frontend Data Flow

## General approach

Use the existing ListingKit UI patterns:

- TanStack Query for server state
- local component state for transient selection state
- URL search params for workbench tab and lightweight filters

Do not introduce new global client state unless later requirements justify it.

## Query model

Recommended query hooks:

- `useSheinEnrollmentStoreSummaries`
- `useSheinEnrollmentStoreSummary`
- `useSheinSyncedProducts`
- `useSheinEnrollmentCostProducts` or reuse synced products with cost filters
- `useSheinActivityCandidates`
- `useSheinEnrollmentRuns`

Recommended mutation hooks:

- `useTriggerSheinStoreSync`
- `useRefreshSheinCandidates`
- `useUpdateSheinProductCost`
- `useReviewSheinCandidate`
- `useExecuteSheinEnrollment`

## Mutation refresh rules

### Trigger sync

Call:

- `POST /api/v1/listing-kits/shein-sync/stores/:store_id/sync`

On success:

- invalidate store summary
- invalidate synced products
- optionally invalidate candidates if the UI shows a stale warning

### Refresh candidates

Call:

- `POST /api/v1/listing-kits/shein-sync/stores/:store_id/candidates/refresh`

On success:

- invalidate store summary
- invalidate candidates

### Update cost price

Call:

- `PATCH /api/v1/listing-kits/shein-sync/products/:id/cost`

On success:

- update or invalidate synced products
- update or invalidate cost tab list
- invalidate candidate summary metrics

### Review candidate

Call:

- `PATCH /api/v1/listing-kits/shein-sync/candidates/:id/review`

On success:

- update the row immediately
- invalidate store summary

### Execute enrollment

Call:

- `POST /api/v1/listing-kits/shein-sync/stores/:store_id/enrollments`

On success:

- invalidate candidates
- invalidate store summary
- invalidate runs

## Routing

## Routes

- `/listing-kits/shein-enrollment`
- `/listing-kits/shein-enrollment/[storeId]`

Workbench params:

- `tab=products|costs|candidates|runs`
- optional lightweight filters later such as:
  - `status=missing-cost`
  - `review=pending_review`

Using search params instead of nested subroutes keeps the first release simple and consistent with current ListingKit patterns.

## Component Plan

Top-level components:

- `SheinEnrollmentDashboardPage`
- `SheinEnrollmentStoreWorkbench`
- `SheinEnrollmentStoreHeader`
- `SheinEnrollmentStoreSummaryCards`
- `SheinSyncedProductsTable`
- `SheinCostPriceTable`
- `SheinCandidatesTable`
- `SheinEnrollmentRunsTable`

Supporting UI pieces:

- store status card
- cost badge
- candidate status badge
- run status badge
- batch action bar for selected candidates
- row-level inline editing controls for cost/review actions

## Reuse Strategy

Reuse existing UI structure where possible:

- [web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx](D:/code/task-processor/web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx:1)
- existing ListingKit admin/query/table patterns
- current TanStack Query provider and hook conventions
- existing badge/panel/button primitives already used in task/admin screens

Do not create a brand-new shell or state framework for this area.

## Error Handling

The UI should distinguish:

- request failed
- request succeeded but no rows are available
- operation blocked by missing data

Examples:

- sync failed: show inline error banner in store workbench header
- no synced products yet: show empty-state guidance with `立即同步`
- no candidates: explain whether the likely reason is no cost prices or no eligible products
- enrollment partial failure: show success/failure counts and direct the user to `报名记录`

## Loading and Empty States

### Overview page

- loading skeleton for store cards
- empty state if there are no store summaries yet

### Products/costs/candidates/runs tabs

- table skeleton while fetching
- filter-aware empty states
- no-results empty states that preserve visible filters

Example:

- `当前没有缺成本商品`
- `当前没有待审核候选`

## Testing Strategy

Frontend tests should cover:

- route rendering for the new section
- overview page displays store summaries
- workbench tab switching via query param
- sync button triggers mutation and refresh
- cost update mutation updates table state
- candidate review and selection behavior
- enrollment mutation refreshes candidates and runs
- empty/error states for each tab

Suggested scope:

- component tests for page shells and tables
- query hook tests where project conventions already exist
- route-level smoke tests for new pages

## Implementation Phasing

### Phase 1

- add routes
- add overview page
- add single-store workbench shell
- wire products, costs, candidates, runs tabs
- support manual sync
- support cost editing
- support candidate review
- support manual enrollment

### Phase 2

- better summary metrics
- richer run detail UX
- optional auto-enroll indicator behavior

### Phase 3

- automatic enrollment controls
- activity-first views
- cross-store operational tooling

## Recommendation

Build the UI as:

- a dedicated `SHEIN 活动报名` area
- a lightweight multi-store overview
- a single-store workbench centered on `候选池`

This keeps the first release directly aligned with the real operator workflow while preserving room for future automation and activity-specific views.
