# ListingKit SDS Responsive Design

## Goal

Make the SDS frontend usable across mobile, tablet, and desktop without horizontal overflow, broken action areas, or desktop-only layout assumptions.

Phase 1 covers the full SDS flow:

- homepage
- recent batches dashboard
- batch workbench shell
- batch run progress view
- batch detail shell and shared SDS page chrome

The intent is to finish SDS first, then reuse the same layout rules for the rest of `listing-kits`.

## Problem

The current SDS UI is mostly desktop-first. Several components still assume wide viewports:

- hero sections place copy and actions side by side too early
- metric cards and summary panels rely on fixed or implied minimum widths
- bulk action bars and filters expand horizontally before they stack
- batch cards and progress rows use wide information density that becomes cramped on phone widths
- long identifiers and labels can stretch cards or create uncomfortable wrapping

This makes the SDS flow harder to use on small screens even when the underlying functionality works.

## Decision

Adopt a mobile-first responsive layout strategy for SDS and normalize a small set of shared layout behaviors:

- single-column primary flow on phones
- progressive multi-column enhancement on tablet and desktop
- no mobile-blocking fixed minimum widths in top-level layout and action areas
- wrap-safe text treatment for long identifiers, titles, and status tags
- action groups that stack vertically on small screens and flatten horizontally only when space allows

The work stays inside existing pages and components. This is a layout and interaction-density redesign, not a feature rewrite.

## Non-Goals

This design does not include:

- visual rebranding of ListingKit
- rewriting SDS workflows or changing business logic
- redesigning non-SDS `listing-kits` pages in this phase
- introducing a new component library
- full container-query adoption across the whole app

## Responsive Principles

### Mobile-First Width Strategy

All SDS screens should remain fully operable from roughly `360px` through `430px` widths before enhancing upward.

Rules:

- content starts as one column
- controls default to full-width or naturally wrapping layout
- information density is reduced before adding columns
- no horizontal scrolling at the page level for core flows

### Progressive Layout Expansion

The layout expands only when content can support it:

- phone: one column
- tablet: two-column grids where cards remain readable
- desktop: wider multi-column layouts for summaries, metrics, and card galleries

The design should prefer fluid values and auto-fit grids over brittle fixed-width pairings.

### Safe Text and Action Handling

The following content must never be allowed to break layout:

- run IDs
- batch IDs
- long batch names
- status chips
- bulk action labels
- product names

Approach:

- allow wrapping where comprehension matters
- clamp or truncate decorative secondary text
- keep action buttons reachable with comfortable tap height

## Target Areas

## SDS Homepage

### Scope

Files centered around:

- `web/listingkit-ui/src/components/listingkit/sds/sds-homepage-entry.tsx`

### Changes

- convert the hero area to a phone-first single column
- stack primary actions vertically on narrow widths and return to inline layout at larger breakpoints
- make the featured batch cards a `1 -> 2 -> 3` progressive grid
- keep the “view all batches” action visually separate from summary cards
- ensure error, loading, and empty states use the same responsive shell

### Expected Outcome

The homepage should read top-to-bottom on mobile without card compression or action crowding.

## Recent Batches Dashboard

### Scope

Files centered around:

- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx`

### Changes

- stack the filter area, risk triage area, and bulk action area on narrow widths
- convert broad horizontal toolbars into grouped vertical sections on mobile
- render batch cards in a `1 -> 2 -> 3` progressive grid
- move card actions below card content on smaller widths
- ensure selection summaries and queue feedback blocks wrap cleanly
- remove or narrow explicit minimum widths that force layout overflow

### Expected Outcome

Users should be able to filter, select, triage risks, and launch bulk actions from a phone-width screen without layout collapse.

## Batch Workbench Shell

### Scope

Files centered around:

- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-page-shell.tsx`
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-page-shell.tsx`
- SDS support components used in the shell’s header, metric, and summary regions

### Changes

- convert top-level summary and metric sections to mobile-first stacked layouts
- remove desktop-biased width guards such as large minimum-width metric regions where they block narrow screens
- keep step guidance readable with smaller spacing and fluid typography on phones
- make quick-entry action groups wrap or stack instead of competing with descriptive content
- preserve the modular card structure of the workbench while reducing top-level density

### Expected Outcome

The workbench should still feel like a structured control surface on desktop, but on mobile it should become a clean vertical workflow instead of a squeezed dashboard.

## Batch Run Progress View

### Scope

Files centered around:

- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-run-progress.tsx`

### Changes

- separate title and action controls vertically on narrow widths
- convert the four summary metrics into a phone-friendly `1` or `2x2` layout
- allow run IDs and batch IDs to break safely
- make item rows vertical on mobile with status and error text beneath identifiers
- preserve clear cancellation-state messaging without requiring wide horizontal space

### Expected Outcome

A user should be able to monitor a run, read failures, and cancel safely from a small screen.

## Batch Detail and Shared SDS Shell States

### Scope

Files centered around:

- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.tsx`
- shared SDS shell wrappers used by batch pages

### Changes

- normalize page padding and section spacing across breakpoints
- convert dual-column detail summaries into stacked mobile sections
- keep navigational links and back-links readable and tappable on narrow widths

## Implementation Notes

### Preferred Layout Techniques

- mobile-first Tailwind breakpoints
- fluid padding and gaps
- `grid` with progressive column counts
- `flex-wrap` only where multi-row actions remain readable
- selective `min-w-0`, `break-all`, `break-words`, and `truncate` handling for overflow-prone text

### Avoid

- page-level horizontal scrolling as a workaround
- keeping desktop grids and only shrinking font size
- introducing separate mobile-only duplicate components
- large breakpoint-only fixes that leave `360px` to `430px` widths broken

## Validation

### Automated Checks

- run relevant frontend tests affected by conditional rendering changes
- run `npm run typecheck`

### Manual Responsive Checks

Validate with real rendering at approximately:

- `390px` width
- `768px` width
- `1280px` width

For each target page, verify:

- no horizontal page overflow
- primary actions stay tappable
- filter and bulk-action sections remain understandable
- long IDs and names do not break cards
- the SDS workflow remains operable from homepage to workbench to run progress

## Success Criteria

This phase is complete when:

- SDS homepage is responsive across phone, tablet, and desktop
- recent batches dashboard is operable on phone widths
- batch workbench shell no longer depends on desktop-only width assumptions
- batch run progress view is readable and actionable on small screens
- no known page-level horizontal overflow remains in the SDS flow
