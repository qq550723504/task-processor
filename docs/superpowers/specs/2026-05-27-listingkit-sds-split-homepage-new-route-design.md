# ListingKit SDS Split Homepage and New Route Design

## Background

The current `/listing-kits/sds` experience has grown into a dense hybrid screen:

- recent batch homepage
- risk and bulk-operations dashboard
- SDS product browser
- batch workbench entry

This was a reasonable transitional step while recent batches, queue mode, grouped SDS behavior, and homepage operations were being added. However, the resulting page now asks users to understand multiple different jobs at once:

- continue an existing batch
- diagnose risky batches
- start a new batch
- browse and filter the full SDS product library

The user feedback is correct: the page feels too heavy because one route currently carries too many product identities.

## Problem Statement

`/listing-kits/sds` currently behaves like all three of these at once:

1. a batch dashboard
2. a product-selection page
3. a workbench entry page

That creates two UX problems:

- the first screen is visually crowded
- the user must decide between “continue” and “start new” while simultaneously seeing full product selection controls

This is not mainly a styling problem. It is an information architecture problem.

## Goals

- Make `/listing-kits/sds` a true entry homepage instead of a mixed dashboard-plus-selection page
- Keep the homepage focused on the first user decision:
  - continue an existing batch
  - start a new batch
- Move full product selection into a dedicated new route
- Keep batch editing in a dedicated batch context
- Reduce first-screen density without removing existing batch/risk/queue capabilities

## Non-Goals

- No rewrite of backend batch persistence or task semantics
- No change to grouped SDS baseline, prompt history, store routing, or task fan-out behavior
- No redesign of the batch editor itself in this phase
- No removal of recent batch risk operations, queue mode, or homepage analytics

## Product Decision

Split the SDS flow into three explicit route roles:

- `/listing-kits/sds`
  - entry homepage
- `/listing-kits/sds/new`
  - dedicated new-batch product selection flow
- `/listing-kits/sds/batches/:id`
  - dedicated single-batch workbench

This makes the product structure match the actual user journey:

1. decide whether to continue or create
2. if creating, select products in a focused flow
3. once a batch exists, edit it in a dedicated batch context

## Alternatives Considered

### Option 1: Keep one route and collapse sections conditionally

Pros:

- lowest short-term implementation cost
- reuses current page composition

Cons:

- preserves the core ambiguity of one page with multiple identities
- logic becomes increasingly conditional and harder to maintain
- route refresh/share behavior remains less clear

### Option 2: Keep `/listing-kits/sds` and switch modes via query params

Example:

- `/listing-kits/sds?mode=create`
- `/listing-kits/sds?mode=dashboard`

Pros:

- cheaper than introducing a new route tree

Cons:

- still keeps multiple product identities in one route
- easier for internal state to sprawl
- less intuitive for navigation and browser history

### Option 3: Recommended, explicit route split

Pros:

- clearer mental model
- cleaner first screen
- easier long-term maintenance
- better support for refresh, direct links, and future expansion

Cons:

- requires routing and composition changes
- requires careful migration of current “single page” assumptions

## Route Model

### `/listing-kits/sds`

This becomes the SDS homepage and no longer shows the full SDS product browser by default.

Responsibilities:

- show start guidance
- show recent batch summary cards
- show homepage risk and bulk-operation surfaces
- route users toward:
  - continue existing work
  - create a new batch

It should not immediately render the full product library.

### `/listing-kits/sds/new`

This becomes the dedicated “new batch and select products” route.

Responsibilities:

- SDS product browser
- shipping-country/category/search filtering
- variant selection
- candidate pool maintenance
- creating a new batch from selected SDS products

This route should be optimized for focused product selection, not for batch operations.

### `/listing-kits/sds/batches/:id`

This becomes the dedicated batch editor route.

Responsibilities:

- continue generation
- review designs
- grouped SDS additions
- store assignment
- prompt history
- task creation
- queue-mode entry context when opened from recent batches

The workbench remains a single-batch editor.

## Homepage UX

The homepage should present one primary choice before anything else:

- `继续最近批次`
- `新建批次并选品`

Below that, show a lightweight recent-batch summary list.

### Recent Batch Summary Rules

- show only the most recent `3` batches by default
- allow `查看全部批次`
- each card shows only high-signal information:
  - batch name
  - primary product
  - lifecycle status
  - top risk badge, if any
  - primary action

More advanced risk breakdowns, batch selection, and bulk actions remain available, but should live inside the expanded “all batches” experience instead of dominating the first screen.

## New Batch Flow

Clicking `新建批次并选品` should navigate to `/listing-kits/sds/new`.

That route should open in a creation-focused state:

- no homepage recent-batches dashboard
- no homepage-level risk overview
- no mixed continue/create messaging

The user should feel that they have entered a dedicated creation flow.

Once a primary SDS product and required setup are chosen, the system should create or initialize a new batch and then transition into the dedicated batch editor route.

Recommended flow:

1. open `/listing-kits/sds/new`
2. choose SDS product and variants
3. create or initialize batch
4. navigate to `/listing-kits/sds/batches/:id`

## Continue Existing Batch Flow

Clicking a batch card or `继续最近批次` should navigate to the dedicated batch route, not partially expand a workbench inside the homepage.

This is an important behavioral cleanup:

- homepage is for choosing work
- batch page is for doing work

The homepage should no longer act like a half-open workbench.

## Batch Editor Behavior

The batch editor route should reuse the existing workbench behavior as much as possible.

That includes:

- grouped SDS selection handling
- baseline readiness checks
- grouped image mode
- prompt history
- recent-batch queue mode
- store assignment and grouped store overrides

This spec is about relocation and simplification of entry flow, not redefinition of batch editing behavior.

## Homepage Density Reduction

The main density reduction comes from removing the full product browser from the homepage.

Recommended first-screen structure:

1. page intro
2. primary decision CTA pair
3. recent batch summary cards
4. optional “view all batches” expansion

Not on first screen anymore:

- full product category filters
- full product list
- variant browser
- candidate pool management

Those move to `/listing-kits/sds/new`.

## Navigation Rules

### Homepage to New

- `新建批次并选品` -> `/listing-kits/sds/new`

### Homepage to Existing Batch

- batch card click
- `继续最近批次`
- queue-entry actions

All of these should resolve to `/listing-kits/sds/batches/:id`.

### New to Batch

Once a batch exists, do not keep the user on the generic new route. Move them into the dedicated batch route.

This makes the URL reflect reality:

- the user is no longer “creating”
- they are now editing a concrete batch

## State and Compatibility

### Existing Recent Batch Homepage Logic

Recent-batch summaries, risk filters, result filters, queue launchers, and bulk operations already exist. These should stay on the homepage route, but be visually scoped to the “recent batches” area instead of sharing screen weight with the product browser.

### Existing Product Browser Logic

The current SDS product browser, candidate pool, and related focus helpers should move to the new route with minimal behavioral change.

### Existing Workbench Logic

The current workbench should migrate behind `/listing-kits/sds/batches/:id` instead of depending on the homepage route as its host.

### Compatibility Strategy

To avoid breaking existing saved state:

1. existing recent-batch persistence remains unchanged
2. existing batch ids continue to load into the same editor data
3. homepage cards now deep-link into batch routes
4. legacy “homepage-expanded workbench” behavior may be supported temporarily behind compatibility code, but should no longer be the primary UX

## Risks

### Risk 1: Route split exposes hidden coupling

The current page may contain assumptions that the homepage, product browser, and workbench are all mounted together.

Mitigation:

- move shared state into clear route-level owners
- keep existing workbench APIs stable where possible
- test direct route entry for all three route types

### Risk 2: Recent batch operations feel buried after summary compression

If too much is hidden behind “view all,” operators may feel slowed down.

Mitigation:

- keep the first 3 cards actionable
- preserve a fast way to expand into the full dashboard
- keep high-value actions on visible cards

### Risk 3: New route introduces more navigation hops

Users starting from scratch now go homepage -> new route -> batch route.

Mitigation:

- keep `/listing-kits/sds/new` lightweight
- transition quickly into batch editor once selection is complete
- make the route change feel purposeful, not bureaucratic

## Testing Strategy

Add or update coverage for:

- homepage renders only recent batch entry content by default
- homepage shows only 3 recent batches before expansion
- `查看全部批次` expands the dashboard
- `新建批次并选品` navigates to `/listing-kits/sds/new`
- `/listing-kits/sds/new` renders the SDS product browser without homepage clutter
- selecting a recent batch opens `/listing-kits/sds/batches/:id`
- direct batch route entry restores workbench correctly
- queue and risk recommendation entry paths still land on the right batch behavior

## Rollout Recommendation

Implement in two phases.

### Phase 1

- add `/listing-kits/sds/new`
- add `/listing-kits/sds/batches/:id`
- move full product browser off the homepage
- compress homepage to recent batch entry + summary cards

### Phase 2

- refine expanded recent-batch dashboard interactions
- further simplify homepage copy and hierarchy
- remove leftover compatibility rendering paths from the old mixed page model

## Decision

Proceed with an explicit SDS route split:

- `/listing-kits/sds` becomes a lightweight entry homepage
- `/listing-kits/sds/new` becomes the focused new-batch product selection page
- `/listing-kits/sds/batches/:id` becomes the dedicated batch editor

This is the cleanest way to reduce page density because it fixes the root issue: one route currently does too many jobs.
